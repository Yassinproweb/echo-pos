package models

import (
	"fmt"
)

// Orders model
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
	Transit   Status = "Transit"

	// all these are the same only considering the order type
	Delivered Status = "Delivered" // Delivery
	Taken     Status = "Taken"     // Takeaway
	Served    Status = "Served"    // DineIn
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

// CalculateOrderTotal computes the total cost and item count for an order
func (o *Order) CalculateOrderTotal() error {
	totalCost := 0
	totalItems := 0

	// 1. Fetch products to get the current price list
	products := FetchProducts()
	productMap := make(map[string]int)
	for _, p := range products {
		productMap[p.Name] = p.Price
	}

	// 2. Iterate through the cart to calculate totals
	for i := range o.OrderCart {
		item := &o.OrderCart[i]

		// Look up the price based on the product name
		price, exists := productMap[item.PdtName]
		if !exists {
			return fmt.Errorf("product %s not found in catalog", item.PdtName)
		}

		// Update UnitPrice if it wasn't already set
		if item.UnitPrice == 0 {
			item.UnitPrice = price
		}

		// Calculate total cost and item count
		totalCost += item.UnitPrice * item.Quantity
		totalItems += item.Quantity
	}

	// 3. Update the Order struct fields
	o.Cost = totalCost
	o.Items = totalItems

	return nil
}

func FetchOrders() []Order {
	return []Order{
		{
			"#ORD0011",
			DineIn,
			Placed,
			0,
			0,
			"Ahmad",
			"0722678837",
			"",
			"11-06-2025 09:30",
			[]OrderItem{
				{"Tropical Fruitsalad", 6, 15000},
				{"Pilau & Goat", 4, 25000},
				{"Ettooke Eriboobedde", 2, 5000},
				{"Luwombo Chicken", 2, 35000},
				{"Pineapple Juice", 6, 5000},
			},
		},
		{
			"#ORD0012",
			DineIn,
			Preparing,
			0,
			0,
			"Ahmad",
			"0722678837",
			"",
			"11-06-2025 09:30",
			[]OrderItem{
				{"Luwombo Chicken", 2, 35000},
				{"Pilau & Goat", 4, 25000},
			},
		},
		{
			"#ORD0013",
			DineIn,
			Canceled,
			0,
			0,
			"Ahmad",
			"0722678837",
			"",
			"11-06-2025 09:30",
			[]OrderItem{
				{"Tropical Fruitsalad", 1, 15000},
				{"Pineapple Juice", 2, 5000},
			},
		},
		{
			"#ORD0014",
			Delivery,
			Delivered,
			0,
			0,
			"Ahmad",
			"0722678837",
			"Nakasozi, Wakiso",
			"11-06-2025 09:30",
			[]OrderItem{
				{"Pilau Goat", 1, 25000},
				{"Pineapple Juice", 2, 5000},
			},
		},
		{
			"#ORD0015",
			Takeaway,
			Ready,
			0,
			0,
			"Ahmad",
			"0722678837",
			"Nakasozi, Wakiso",
			"11-06-2025 09:30",
			[]OrderItem{
				{"Pilau & Goat", 3, 25000},
			},
		},
		{
			"#ORD0016",
			Delivery,
			Transit,
			0,
			0,
			"Ahmad",
			"0722678837",
			"Nakasozi, Wakiso",
			"11-06-2025 09:30",
			[]OrderItem{
				{"Tropical Fruitsalad", 3, 15000},
			},
		},
		{
			"#ORD0017",
			Takeaway,
			Taken,
			0,
			0,
			"Ahmad",
			"0722678837",
			"Nakasozi, Wakiso",
			"11-06-2025 09:30",
			[]OrderItem{
				{"Spicy Rolex", 3, 7000},
			},
		},
		{
			"#ORD0018",
			DineIn,
			Served,
			0,
			0,
			"Ahmad",
			"0722678837",
			"",
			"11-06-2025 09:30",
			[]OrderItem{
				{"Pilau & Goat", 4, 25000},
			},
		},
		{
			"#ORD0019",
			Delivery,
			Canceled,
			0,
			0,
			"Ahmad",
			"0722678837",
			"Nakasozi, Wakiso",
			"11-06-2025 09:30",
			[]OrderItem{
				{"Luwombo Chicken", 1, 35000},
				{"Pineapple Juice", 1, 5000},
				{"Ettooke Eriboobedde", 1, 5000},
			},
		},
	}
}

// PrepareReceipt updates the order totals and returns the items for rendering
func (o *Order) PrepareReceipt() ([]OrderItem, error) {
	err := o.CalculateOrderTotal()
	if err != nil {
		return nil, err
	}

	return o.OrderCart, nil
}

// Products model
type Product struct {
	Name        string
	Description string
	Price       int
	Image       string
}

func FetchProducts() []Product {
	return []Product{
		{
			"Luwombo Chicken",
			"Steamed chicken with a delicious taste, wrapped in banana leaves to keep the natural aroma.",
			35000,
			"/assets/imgs/lw-chicken.jpeg",
		},
		{
			"Spicy Rolex",
			"Prepared from the best wheat and vegetable oil plus eggs from locally bred poultry.",
			7000,
			"/assets/imgs/ff-rolex.jpeg",
		},
		{
			"Pineapple Juice",
			"Perfectly blended from organic fruits locally grown in Uganda with zero sugar added.",
			5000,
			"/assets/imgs/juice-pineapple.jpg",
		},
		{
			"Tropical Fruitsalad",
			"A mix of most tropical fruits, vegetables, berries, nuts and citrus fruits.",
			15000,
			"/assets/imgs/fr-fruit_salad.jpg",
		},
		{
			"Pilau & Goat",
			"Yummy brown rice with goat's meat. Not Biriyani, it's prepared in a local way.",
			25000,
			"/assets/imgs/st-pilau.jpeg",
		},
		{
			"Ettooke Eriboobedde",
			"Steamed matooke/bananas wrapped in banana leaves to keep the natural aroma.",
			5000,
			"/assets/imgs/st-matooke.jpeg",
		},
	}
}

// Tables model
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

// AssignOrder ensures only DineIn orders are linked to a table
func (t *Table) AssignOrder(o *Order) error {
	if o.Type != DineIn {
		return fmt.Errorf("invalid order type: tables only accept DineIn orders")
	}

	t.OrderCurr = o    // Link the order
	t.State = Occupied // Update table state
	return nil
}

// CompleteService finalizes the DineIn order and frees up the table
func (t *Table) CompleteService() error {
	if t.OrderCurr == nil {
		return fmt.Errorf("no active order found on this table")
	}

	t.OrderCurr.Status = Served
	t.OrderCurr = nil
	t.State = Pending

	return nil
}

func (t *Table) ResetTable() {
	t.State = Available
}

func FetchTables() []Table {
	return []Table{
		{
			Name:      "#TBR001",
			Capacity:  6,
			State:     Available,
			OrderCurr: nil,
		},
		{
			Name:      "#TBR002",
			Capacity:  6,
			State:     Occupied,
			OrderCurr: nil,
		},
		{
			Name:      "#TBR003",
			Capacity:  2,
			State:     Available,
			OrderCurr: nil,
		},
		{
			Name:      "#TBR004",
			Capacity:  2,
			State:     Available,
			OrderCurr: nil,
		},
		{
			Name:      "#TBR005",
			Capacity:  2,
			State:     Available,
			OrderCurr: nil,
		},
		{
			Name:      "#TBR006",
			Capacity:  6,
			State:     Available,
			OrderCurr: nil,
		},
		{
			Name:      "#TBR007",
			Capacity:  4,
			State:     Available,
			OrderCurr: nil,
		},
		{
			Name:      "#TBR008",
			Capacity:  4,
			State:     Available,
			OrderCurr: nil,
		},
		{
			Name:      "#TBR009",
			Capacity:  4,
			State:     Available,
			OrderCurr: nil,
		},
		{
			Name:      "#TBR010",
			Capacity:  4,
			State:     Available,
			OrderCurr: nil,
		},
		{
			Name:      "#TBR011",
			Capacity:  4,
			State:     Available,
			OrderCurr: nil,
		},
		{
			Name:      "#TBR012",
			Capacity:  4,
			State:     Pending,
			OrderCurr: nil,
		},
	}
}

// CanAcceptDineIn checks if there is at least one Available table
func CanAcceptDineIn(tables []Table) bool {
	for _, t := range tables {
		if t.State == Available {
			return true
		}
	}
	return false
}

// AssignOrderDestination
func AssignOrderDestination(orders []Order, tables []Table) {
	for i := range orders {
		if orders[i].Type == DineIn && orders[i].Destination == "" {
			for j := range tables {
				if tables[j].State == Available {
					orders[i].Destination = tables[j].Name
					tables[j].OrderCurr = &orders[i]
					tables[j].State = Occupied
					break
				}
			}
		}
	}

	for i := range orders {
		if orders[i].Type == Takeaway {
			orders[i].Destination = "Pick-Up"
		}
	}
}
