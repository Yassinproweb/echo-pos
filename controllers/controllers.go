package controllers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/Yassinproweb/echo-pos/auth"
	"github.com/Yassinproweb/echo-pos/models"
	"github.com/labstack/echo/v5"
)

// orders controllers
func RenderOrders(c *echo.Context) error {
	tables := models.FetchTables()

	isAdmin := auth.IsAdminSession(c)

	orders := models.FetchOrders()
	for i := range orders {
		orders[i].CalculateOrderTotal()
		// Lets the template show a "Delete" action on canceled orders, but
		// only for the admin — cashiers can view canceled orders, never
		// remove them.
		orders[i].IsAdmin = isAdmin
	}

	data := map[string]any{
		"orders":    orders,
		"tables":    tables,
		"IsAdmin":   isAdmin,
		"ActorName": auth.ActorName(c),
	}

	return c.Render(http.StatusOK, "orders.html", data)
}

// products controllers
func RenderProducts(c *echo.Context) error {
	products := models.FetchProducts()

	data := map[string]any{
		"products":  products,
		"IsAdmin":   auth.IsAdminSession(c),
		"ActorName": auth.ActorName(c),
	}

	return c.Render(http.StatusOK, "products.html", data)
}

// RenderEditOrder shows the item-swap page for one order (only usable
// while the order is Placed or Preparing).
func RenderEditOrder(c *echo.Context) error {
	orderID := c.Param("id")

	orders := models.FetchOrders()
	var order *models.Order
	for i := range orders {
		if orders[i].Name == orderID {
			order = &orders[i]
			break
		}
	}
	if order == nil {
		return c.String(http.StatusNotFound, "Order Not Found")
	}
	order.CalculateOrderTotal()

	quantities := make(map[string]int)
	for _, it := range order.OrderCart {
		quantities[it.PdtName] = it.Quantity
	}

	itemsJSON, _ := json.Marshal(order.OrderCart)

	products := models.FetchProducts()

	data := map[string]any{
		"Order":            order,
		"products":         products,
		"quantities":       quantities,
		"InitialItemsB64":  base64.StdEncoding.EncodeToString(itemsJSON),
		"IsAdmin":          auth.IsAdminSession(c),
		"ActorName":        auth.ActorName(c),
	}

	return c.Render(http.StatusOK, "edit_order.html", data)
}

// tables controllers
func RenderTables(c *echo.Context) error {
	orders := models.FetchOrders()
	tables := models.FetchTables()

	data := map[string]any{
		"orders":    orders,
		"tables":    tables,
		"IsAdmin":   auth.IsAdminSession(c),
		"ActorName": auth.ActorName(c),
	}

	return c.Render(http.StatusOK, "tables.html", data)
}
