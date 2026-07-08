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
