package models

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/Yassinproweb/echo-pos/db"
)

// =======================
// RESERVATIONS
// =======================

type ReservationStatus string

const (
	ReservationConfirmed ReservationStatus = "Confirmed"
	ReservationCancelled ReservationStatus = "Cancelled"
)

// EventTypes lists every event type the reservation form can offer in its
// <select>. Kept in one place so the form and the DB CHECK constraint in
// db/db.go stay in sync.
var EventTypes = []string{
	"Birthday",
	"Wedding",
	"Conference",
	"Anniversary",
	"Corporate Meeting",
	"Graduation",
	"Baby Shower",
	"Other",
}

type Reservation struct {
	Name                string
	EventType           string
	NumPeople           int
	CustName            string
	CustNumber          string
	ReservationDateTime time.Time
	TablesNeeded        int
	TablesAssigned      sql.NullString
	Status              ReservationStatus
	CreatedBy           sql.NullString
	CreatedAt           time.Time
}

func (r Reservation) FormattedDateTime() string {
	loc, err := time.LoadLocation("Africa/Kampala")
	if err != nil {
		return r.ReservationDateTime.Format("02-01-2006 15:04")
	}
	return r.ReservationDateTime.In(loc).Format("02-01-2006 15:04")
}

func (r Reservation) TablesAssignedDisplay() string {
	if r.TablesAssigned.Valid && r.TablesAssigned.String != "" {
		return r.TablesAssigned.String
	}
	return "—"
}

func (r Reservation) CreatedByDisplay() string {
	if r.CreatedBy.Valid && r.CreatedBy.String != "" {
		return r.CreatedBy.String
	}
	return "—"
}

// SuggestTablesForCapacity looks at every currently Available table and
// works out which one(s) would be needed to seat `numPeople` guests:
//
//  1. If a single available table can already fit everyone, that's the
//     recommendation (smallest table that still fits, to avoid wasting a
//     big table on a small party).
//  2. Otherwise, tables are combined largest-first until the combined
//     capacity is enough, and every table used is returned.
//
// It returns the list of table names it would assign and the total number
// of tables needed. It does NOT reserve anything — CreateReservation does
// that inside a transaction.
func SuggestTablesForCapacity(numPeople int) (tableNames []string, err error) {
	if numPeople <= 0 {
		return nil, fmt.Errorf("number of people must be greater than zero")
	}

	all := FetchTables()

	var available []Table
	for _, t := range all {
		if t.State == Available {
			available = append(available, t)
		}
	}

	if len(available) == 0 {
		return nil, fmt.Errorf("no tables are currently available")
	}

	// 1) Best single-table fit: smallest capacity that still seats everyone.
	sort.Slice(available, func(i, j int) bool {
		return available[i].Capacity < available[j].Capacity
	})

	for _, t := range available {
		if t.Capacity >= numPeople {
			return []string{t.Name}, nil
		}
	}

	// 2) Combine tables, largest first, until capacity is covered.
	sort.Slice(available, func(i, j int) bool {
		return available[i].Capacity > available[j].Capacity
	})

	var combo []string
	total := 0
	for _, t := range available {
		combo = append(combo, t.Name)
		total += t.Capacity
		if total >= numPeople {
			return combo, nil
		}
	}

	return nil, fmt.Errorf(
		"not enough available table capacity for %d guests (only %d seats available across %d tables)",
		numPeople, total, len(available),
	)
}

// CreateReservation books tables for an event. It finds suitable available
// table(s) via SuggestTablesForCapacity, marks them 'Reserved', and stores
// the reservation. Everything happens inside one transaction so a failure
// partway through never leaves tables reserved without a matching
// reservation row (or vice versa).
func CreateReservation(
	eventType string,
	numPeople int,
	custName, custNumber string,
	reservationDateTime time.Time,
	createdBy string,
) (string, error) {

	if custName == "" || custNumber == "" {
		return "", fmt.Errorf("customer name and number are required")
	}

	tableNames, err := SuggestTablesForCapacity(numPeople)
	if err != nil {
		return "", err
	}

	tx, err := db.DB.Begin()
	if err != nil {
		return "", err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	assignedList := ""
	for i, name := range tableNames {
		if i > 0 {
			assignedList += ", "
		}
		assignedList += name
	}

	result, err := tx.Exec(`
		INSERT INTO reservations (
			event_type,
			num_people,
			cust_name,
			cust_number,
			reservation_datetime,
			tables_needed,
			tables_assigned,
			status,
			created_by
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'Confirmed', ?)
	`,
		eventType,
		numPeople,
		custName,
		custNumber,
		reservationDateTime,
		len(tableNames),
		assignedList,
		createdBy,
	)

	if err != nil {
		return "", err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}

	var reservationName string
	if err := tx.QueryRow(`SELECT name FROM reservations WHERE id = ?`, id).Scan(&reservationName); err != nil {
		return "", err
	}

	for _, tableName := range tableNames {
		_, err = tx.Exec(`
			UPDATE tables
			SET state = 'Reserved', reserved_for = ?
			WHERE name = ? AND state = 'Available'
		`, reservationName, tableName)

		if err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	committed = true

	return reservationName, nil
}

// FetchReservations returns every reservation, most recent first.
func FetchReservations() []Reservation {
	rows, err := db.DB.Query(`
		SELECT
			name,
			event_type,
			num_people,
			cust_name,
			cust_number,
			reservation_datetime,
			tables_needed,
			tables_assigned,
			status,
			created_by,
			created_at
		FROM reservations
		ORDER BY id DESC
	`)

	if err != nil {
		return []Reservation{}
	}
	defer rows.Close()

	var reservations []Reservation
	for rows.Next() {
		var r Reservation
		var status string

		err := rows.Scan(
			&r.Name,
			&r.EventType,
			&r.NumPeople,
			&r.CustName,
			&r.CustNumber,
			&r.ReservationDateTime,
			&r.TablesNeeded,
			&r.TablesAssigned,
			&status,
			&r.CreatedBy,
			&r.CreatedAt,
		)

		if err != nil {
			continue
		}

		r.Status = ReservationStatus(status)
		reservations = append(reservations, r)
	}

	return reservations
}

// CancelReservation marks a reservation Cancelled and frees any tables that
// were reserved for it back to Available.
func CancelReservation(name string) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	res, err := tx.Exec(`
		UPDATE reservations SET status = 'Cancelled' WHERE name = ? AND status = 'Confirmed'
	`, name)
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("reservation %s not found or already cancelled", name)
	}

	if _, err := tx.Exec(`
		UPDATE tables
		SET state = 'Available', reserved_for = NULL
		WHERE reserved_for = ?
	`, name); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true

	return nil
}
