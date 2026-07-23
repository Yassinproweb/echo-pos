package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Yassinproweb/echo-pos/db"
)

// =======================
// ORDER MODELS
// =======================

type Type string
type Status string

const (
	Takeaway Type = "Takeaway"
	Delivery Type = "Delivery"
	DineIn   Type = "DineIn"

	Placed    Status = "Placed"
	Preparing Status = "Preparing"
	Ready     Status = "Ready"
	Canceled  Status = "Canceled"

	Transit Status = "Transit"
	Waiting Status = "Waiting"
	PickUp  Status = "PickUp"

	Delivered Status = "Delivered"
	Taken     Status = "Taken"
	Served    Status = "Served"
)

type OrderItem struct {
	PdtName   string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int    `json:"unitPrice"`
}

type Order struct {
	Name        string
	Type        Type
	Status      Status
	Items       int
	Cost        int
	CustName    string
	CustNumber  string
	Destination string
	CashierName sql.NullString
	DateTime    time.Time
	OrderCart   []OrderItem

	// IsAdmin is not stored in the DB — it's set by the handler right
	// before rendering, purely so templates can decide whether to show the
	// "Delete" action on canceled orders (admin-only).
	IsAdmin bool
}

// CashierDisplay returns the cashier/admin name for display on receipts and
// order lists, falling back to a placeholder for legacy orders that predate
// the cashier-tracking feature.
func (o Order) CashierDisplay() string {
	if o.CashierName.Valid && o.CashierName.String != "" {
		return o.CashierName.String
	}
	return "—"
}

func (o Order) IsEditable() bool {
	return o.Status == Placed || o.Status == Preparing
}

// NextManualStatus returns the next status a cashier could manually advance
// this order to, or "" if there is none (terminal status, or Canceled).
func (o Order) NextManualStatus() Status {
	if o.Status == Canceled {
		return ""
	}
	return NextStatus(o.Type, o.Status)
}

// PreviousManualStatus returns the prior status a cashier could manually
// revert this order to, or "" if there is none (first step, or Canceled).
func (o Order) PreviousManualStatus() Status {
	if o.Status == Canceled {
		return ""
	}
	return PreviousStatus(o.Type, o.Status)
}

func (o Order) FormattedDateTime() string {
	loc, err := time.LoadLocation("Africa/Kampala")
	if err != nil {
		return o.DateTime.Format("02-01-2006 15:04")
	}

	return o.DateTime.In(loc).Format("02-01-2006 15:04")
}

// =======================
// PRODUCTS
// =======================

type Product struct {
	Name        string
	Description string
	Price       int
	Image       string
}

// =======================
// TABLES
// =======================

type State string

const (
	Available State = "Available"
	Occupied  State = "Occupied"
	Pending   State = "Pending"
)

type Table struct {
	Name             string
	Capacity         int
	State            State
	CurrentOrderName sql.NullString
	ReservedFor      sql.NullString
}

// =======================
// TABLE STATE (manual only — cashier/admin)
// =======================
//
// NextManualState / PreviousManualState kept as stubs so any existing
// template references compile — they are no longer used after the
// single-button simplification.

func (t Table) NextManualState() State     { return "" }
func (t Table) PreviousManualState() State { return "" }

// ValidateManualTableStateChange enforces:
//   - Available has no manual buttons — nothing should post from that state.
//   - Occupied -> Pending (cashier marks table needs cleaning, drops order).
//   - Pending  -> Available (table is clean and ready again).
//   - Occupied is never a valid manual target (set automatically by system).
func ValidateManualTableStateChange(current, target State) error {
	if target == Occupied {
		return fmt.Errorf("Occupied is set automatically by the system and cannot be assigned manually")
	}
	if current == Available {
		return fmt.Errorf("Available tables have no manual state change")
	}
	allowed := map[State]State{
		Occupied: Pending,
		Pending:  Available,
	}
	expected, ok := allowed[current]
	if !ok || target != expected {
		return fmt.Errorf("cannot manually move table from %s to %s", current, target)
	}
	return nil
}

// UpdateTableStateManual performs a manual, cashier/admin-triggered table
// state change (via the /pos route group, which already requires a logged
// in session), validating it moves exactly one step around the cycle
// before writing to the DB. Returns the refreshed Table on success.
func UpdateTableStateManual(tableName string, target State) (Table, error) {
	var currentState string

	err := db.DB.QueryRow(`
		SELECT state FROM tables WHERE name = ?
	`, tableName).Scan(&currentState)

	if err == sql.ErrNoRows {
		return Table{}, fmt.Errorf("table %s not found", tableName)
	}
	if err != nil {
		return Table{}, err
	}

	if err := ValidateManualTableStateChange(State(currentState), target); err != nil {
		return Table{}, err
	}

	if _, err := db.DB.Exec(`
		UPDATE tables SET state = ?,
		current_order_name = CASE WHEN ? = 'Pending' THEN NULL ELSE current_order_name END
		WHERE name = ?
	`, string(target), string(target), tableName); err != nil {
		return Table{}, err
	}

	for _, t := range FetchTables() {
		if t.Name == tableName {
			return t, nil
		}
	}

	return Table{}, fmt.Errorf("table %s not found after update", tableName)
}

// =======================
// ORDER LOGIC
// =======================

func (o *Order) CalculateOrderTotal() error {
	totalCost := 0
	totalItems := 0

	products := FetchProducts()

	productMap := make(map[string]int)

	for _, p := range products {
		productMap[p.Name] = p.Price
	}

	for i := range o.OrderCart {
		item := &o.OrderCart[i]

		price, exists := productMap[item.PdtName]

		if !exists {
			return fmt.Errorf("product %s not found", item.PdtName)
		}

		if item.UnitPrice == 0 {
			item.UnitPrice = price
		}

		totalCost += item.UnitPrice * item.Quantity
		totalItems += item.Quantity
	}

	o.Cost = totalCost
	o.Items = totalItems

	return nil
}

func (o *Order) PrepareReceipt() ([]OrderItem, error) {
	err := o.CalculateOrderTotal()

	if err != nil {
		return nil, err
	}

	return o.OrderCart, nil
}

// =======================
// FETCH PRODUCTS
// =======================

func FetchProducts() []Product {
	rows, err := db.DB.Query(`
		SELECT
			name,
			description,
			price,
			image
		FROM products
	`)

	if err != nil {
		return []Product{}
	}

	defer rows.Close()

	var products []Product

	for rows.Next() {
		var p Product

		err := rows.Scan(
			&p.Name,
			&p.Description,
			&p.Price,
			&p.Image,
		)

		if err != nil {
			continue
		}

		products = append(products, p)
	}

	return products
}

// CreateProduct adds a new menu item.
func CreateProduct(name, description string, price int, image string) error {
	if name == "" || description == "" || image == "" {
		return fmt.Errorf("name, description and image are required")
	}

	if price < 0 {
		return fmt.Errorf("price cannot be negative")
	}

	_, err := db.DB.Exec(`
		INSERT INTO products (name, description, price, image)
		VALUES (?, ?, ?, ?)
	`, name, description, price, image)

	return err
}

// =======================
// FETCH ORDER ITEMS
// =======================

func FetchOrderItems(orderName string) []OrderItem {
	rows, err := db.DB.Query(`
		SELECT
			pdt_name,
			quantity,
			unit_price
		FROM order_items
		WHERE order_name = ?
	`, orderName)

	if err != nil {
		return []OrderItem{}
	}

	defer rows.Close()

	var items []OrderItem

	for rows.Next() {
		var item OrderItem

		err := rows.Scan(
			&item.PdtName,
			&item.Quantity,
			&item.UnitPrice,
		)

		if err != nil {
			continue
		}

		items = append(items, item)
	}

	return items
}

// =======================
// FETCH ORDERS
// =======================

func FetchOrders() []Order {
	rows, err := db.DB.Query(`
		SELECT
			name,
			type,
			status,
			items,
			cost,
			cust_name,
			cust_number,
			destination,
			cashier_name,
			date_time
		FROM orders
		ORDER BY id ASC
	`)

	if err != nil {
		return []Order{}
	}

	defer rows.Close()

	var orders []Order

	for rows.Next() {
		var o Order

		err := rows.Scan(
			&o.Name,
			&o.Type,
			&o.Status,
			&o.Items,
			&o.Cost,
			&o.CustName,
			&o.CustNumber,
			&o.Destination,
			&o.CashierName,
			&o.DateTime,
		)

		if err != nil {
			continue
		}

		o.OrderCart = FetchOrderItems(o.Name)

		orders = append(orders, o)
	}

	return orders
}

// DeleteCanceledOrder permanently removes an order, but ONLY if it is
// currently in the "Canceled" state. This is an admin-only action (enforced
// at the route layer) — cashiers can view canceled orders but never delete
// them.
func DeleteCanceledOrder(orderName string) error {
	var status string

	err := db.DB.QueryRow(`
		SELECT status FROM orders WHERE name = ?
	`, orderName).Scan(&status)

	if err == sql.ErrNoRows {
		return fmt.Errorf("order %s not found", orderName)
	}
	if err != nil {
		return err
	}

	if Status(status) != Canceled {
		return fmt.Errorf("only canceled orders can be deleted")
	}

	_, err = db.DB.Exec(`DELETE FROM orders WHERE name = ?`, orderName)
	return err
}

// UpdateOrderItems replaces an order's cart, but only while it's still
// Placed or Preparing (i.e. before it reaches Ready).
func UpdateOrderItems(orderName string, items []OrderItem) error {
	var status string
	err := db.DB.QueryRow(`SELECT status FROM orders WHERE name = ?`, orderName).Scan(&status)
	if err == sql.ErrNoRows {
		return fmt.Errorf("order %s not found", orderName)
	}
	if err != nil {
		return err
	}
	if status != string(Placed) {
		return fmt.Errorf("order can no longer be edited (status: %s)", status)
	}

	valid := 0
	for _, it := range items {
		if it.Quantity > 0 {
			valid++
		}
	}
	if valid == 0 {
		return fmt.Errorf("an order must have at least one item")
	}

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

	if _, err := tx.Exec(`DELETE FROM order_items WHERE order_name = ?`, orderName); err != nil {
		return err
	}

	for _, it := range items {
		if it.Quantity <= 0 {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO order_items (order_name, pdt_name, quantity)
			VALUES (?, ?, ?)
		`, orderName, it.PdtName, it.Quantity); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// CancelOrder marks an order Canceled — used once an order has passed the
// point where items can still be swapped (Ready or later).
func CancelOrder(orderName string) error {
	res, err := db.DB.Exec(`
		UPDATE orders SET status = 'Canceled'
		WHERE name = ? AND status NOT IN ('Canceled','Delivered','Taken','Served')
	`, orderName)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("order %s cannot be canceled (already completed or not found)", orderName)
	}
	return nil
}

// =======================
// FETCH TABLES
// =======================

func FetchTables() []Table {
	rows, err := db.DB.Query(`
		SELECT
			name,
			capacity,
			state,
			current_order_name,
			reserved_for
		FROM tables
	`)

	if err != nil {
		return []Table{}
	}

	defer rows.Close()

	var tables []Table

	for rows.Next() {
		var t Table

		err := rows.Scan(
			&t.Name,
			&t.Capacity,
			&t.State,
			&t.CurrentOrderName,
			&t.ReservedFor,
		)

		if err != nil {
			continue
		}

		tables = append(tables, t)
	}

	return tables
}

// CreateTable inserts a new table with the given capacity. The table name is
// auto-generated by the `generate_table_name` trigger (e.g. #TBL022).
func CreateTable(capacity int) (string, error) {
	if capacity <= 0 {
		return "", fmt.Errorf("capacity must be greater than zero")
	}

	result, err := db.DB.Exec(`
		INSERT INTO tables (capacity, state)
		VALUES (?, 'Available')
	`, capacity)

	if err != nil {
		return "", err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}

	var name string
	err = db.DB.QueryRow(`SELECT name FROM tables WHERE id = ?`, id).Scan(&name)
	return name, err
}

// =======================
// UPDATE ORDER STATUS
// =======================

func UpdateOrderStatus(
	orderName string,
	status Status,
) error {

	_, err := db.DB.Exec(`
		UPDATE orders
		SET status = ?
		WHERE name = ?
	`,
		status,
		orderName,
	)

	return err
}

// =======================
// STATUS SEQUENCES (manual, per order Type)
// =======================
//
// Delivery: Placed -> Preparing -> Ready -> Transit   -> Delivered
// Takeaway: Placed -> Preparing -> Ready -> PickUp     -> Taken
// DineIn:   Placed -> Preparing -> Ready -> Waiting    -> Served

var statusSequence = map[Type][]Status{
	Delivery: {Placed, Preparing, Ready, Transit, Delivered},
	Takeaway: {Placed, Preparing, Ready, PickUp, Taken},
	DineIn:   {Placed, Preparing, Ready, Waiting, Served},
}

// NextStatus returns the next status in the sequence for the given order
// type, or "" if there is no next step (already at the terminal status) or
// the type/status combination is unrecognized.
func NextStatus(orderType Type, current Status) Status {
	seq, ok := statusSequence[orderType]
	if !ok {
		return ""
	}
	for i, s := range seq {
		if s == current && i+1 < len(seq) {
			return seq[i+1]
		}
	}
	return ""
}

// PreviousStatus returns the prior status in the sequence for the given
// order type, or "" if already at the first step (Placed) or the
// type/status combination is unrecognized.
func PreviousStatus(orderType Type, current Status) Status {
	seq, ok := statusSequence[orderType]
	if !ok {
		return ""
	}
	for i, s := range seq {
		if s == current && i-1 >= 0 {
			return seq[i-1]
		}
	}
	return ""
}

// ValidateManualStatusChange enforces that manual status changes move
// exactly one step — forward OR backward — along the strict sequential
// order for the order's type. Canceling is always allowed (handled
// separately via CancelOrder) — this only governs step-by-step
// progression/regression through the sequence.
func ValidateManualStatusChange(orderType Type, current, target Status) error {
	seq, ok := statusSequence[orderType]
	if !ok {
		return fmt.Errorf("unknown order type: %s", orderType)
	}

	currentIdx, targetIdx := -1, -1
	for i, s := range seq {
		if s == current {
			currentIdx = i
		}
		if s == target {
			targetIdx = i
		}
	}

	if currentIdx == -1 {
		return fmt.Errorf("order is not in a valid state for manual transition (status: %s)", current)
	}
	if targetIdx == -1 {
		return fmt.Errorf("%s is not a valid status for order type %s", target, orderType)
	}

	diff := targetIdx - currentIdx
	if diff != 1 && diff != -1 {
		if diff > 0 {
			return fmt.Errorf("cannot skip ahead from %s to %s; next step must be %s", current, target, seq[currentIdx+1])
		}
		return fmt.Errorf("cannot skip back from %s to %s", current, target)
	}

	return nil
}

// UpdateOrderStatusManual performs a manual, cashier/admin-triggered status
// change, validating that it follows the sequential order for the order's
// type before writing to the DB.
func UpdateOrderStatusManual(orderName string, target Status) error {
	var currentStatus, orderType string

	err := db.DB.QueryRow(`
		SELECT status, type FROM orders WHERE name = ?
	`, orderName).Scan(&currentStatus, &orderType)

	if err == sql.ErrNoRows {
		return fmt.Errorf("order %s not found", orderName)
	}
	if err != nil {
		return err
	}

	if Status(currentStatus) == Canceled {
		return fmt.Errorf("order is canceled and cannot be progressed")
	}

	if err := ValidateManualStatusChange(Type(orderType), Status(currentStatus), target); err != nil {
		return err
	}

	return UpdateOrderStatus(orderName, target)
}

func CanAcceptDineIn() bool {
	var count int

	err := db.DB.QueryRow(`
		SELECT COUNT(*)
		FROM tables
		WHERE state = 'Available'
	`).Scan(&count)

	if err != nil {
		return false
	}

	return count > 0
}
