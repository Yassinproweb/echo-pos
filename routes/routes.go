package routes

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Yassinproweb/echo-pos/db"
	"github.com/Yassinproweb/echo-pos/models"
	"github.com/labstack/echo/v5"
)

func SelectOrderRoute(c *echo.Context) error {
	orderID := c.Param("id")

	orders := models.FetchOrders()

	var so *models.Order

	for i := range orders {
		if orders[i].Name == orderID {
			so = &orders[i]
			break
		}
	}

	if so == nil {
		return c.String(http.StatusNotFound, "Order Not Found")
	}

	err := so.CalculateOrderTotal()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Calculation Error")
	}

	return c.Render(http.StatusOK, "receipt", so)
}

func UpdateStatusAfterPrint(c *echo.Context) error {
	orderID := c.Param("id")

	if err := models.UpdateOrderStatus(orderID, models.Preparing); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to update status")
	}

	return c.Render(http.StatusOK, "order_form", nil)
}

type CreateOrderRequest struct {
	CustomerName   string `form:"customer_name"`
	CustomerNumber string `form:"customer_number"`
	OrderType      string `form:"order_type"`
	OrderDest      string `form:"order_dest"`
	OrderStatus    string `form:"order_status"`
	Items          string `form:"items"` // JSON string
}

func CreateOrder(c *echo.Context) error {
	var req CreateOrderRequest

	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "Invalid form data")
	}

	var cartItems []models.OrderItem
	if req.Items != "" {
		if err := json.Unmarshal([]byte(req.Items), &cartItems); err != nil {
			return c.String(http.StatusBadRequest, "Invalid items JSON: "+err.Error())
		}
	}

	order := models.Order{
		Type:        models.Type(req.OrderType),
		Status:      models.Status(req.OrderStatus),
		CustName:    req.CustomerName,
		CustNumber:  req.CustomerNumber,
		Destination: req.OrderDest,
		OrderCart:   cartItems,
	}

	if err := order.CalculateOrderTotal(); err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	tx, err := db.DB.Begin()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Database error")
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Insert order
	result, err := tx.Exec(`
		INSERT INTO orders (type, status, cust_name, cust_number, destination)
		VALUES (?, ?, ?, ?, ?)
	`, order.Type, order.Status, order.CustName, order.CustNumber, order.Destination)

	if err != nil {
		fmt.Println("Order insert error:")
		return c.String(http.StatusInternalServerError, "Failed to create order")
	}

	orderID, _ := result.LastInsertId()

	// Get the generated order name (after trigger)
	var orderName string
	err = tx.QueryRow("SELECT name FROM orders WHERE id = ?", orderID).Scan(&orderName)
	if err != nil {
		fmt.Println("Failed to get order name:", err)
		return c.String(http.StatusInternalServerError, "Failed to get order name")
	}

	fmt.Println("Created Order:", orderName, orderID)

	// Insert items using the actual order_name
	for _, item := range cartItems {
		_, err = tx.Exec(`
			INSERT INTO order_items (order_name, pdt_name, quantity)
			VALUES (?, ?, ?)
		`, orderName, item.PdtName, item.Quantity)

		if err != nil {
			fmt.Println("Created Order:", orderName)
			fmt.Println("Failed to save item:", err)
			return c.String(http.StatusInternalServerError, "Failed to save items: "+err.Error())
		}
	}

	if err := tx.Commit(); err != nil {
		fmt.Println("Commit error:", err)
		return c.String(http.StatusInternalServerError, "Failed to save order")
	}

	committed = true

	order.Name = orderName

	return c.Render(http.StatusCreated, "order_created", map[string]any{
		"Toast":   map[string]any{"OrderName": orderName},
		"Receipt": order,
	})
}
