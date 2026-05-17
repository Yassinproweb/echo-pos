// models/models.go

package models

import (
	"database/sql"
	"fmt"

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
	PdtName   string
	Quantity  int
	UnitPrice int
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
	DateTime    string
	OrderCart   []OrderItem
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
	Name      string
	Capacity  int
	State     State
	OrderCurr *Order
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
			date_time
		FROM orders
		ORDER BY id DESC
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

// =======================
// FETCH TABLES
// =======================

func FetchTables() []Table {
	rows, err := db.DB.Query(`
		SELECT
			name,
			capacity,
			state,
			current_order_name
		FROM tables
	`)

	if err != nil {
		return []Table{}
	}

	defer rows.Close()

	var tables []Table

	for rows.Next() {
		var t Table
		var currentOrder sql.NullString

		err := rows.Scan(
			&t.Name,
			&t.Capacity,
			&t.State,
			&currentOrder,
		)

		if err != nil {
			continue
		}

		tables = append(tables, t)
	}

	return tables
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
