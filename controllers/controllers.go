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
	business, _ := models.GetBusiness()

	orders := models.FetchOrders()
	for i := range orders {
		orders[i].CalculateOrderTotal()
		// Lets the template show a "Delete" action on canceled orders, but
		// only for the admin — cashiers can view canceled orders, never
		// remove them.
		orders[i].IsAdmin = isAdmin
	}

	data := map[string]any{
		"Business":        business,
		"orders":          orders,
		"CanAcceptDineIn": models.CanAcceptDineIn(),
		"tables":          tables,
		"IsAdmin":         isAdmin,
		"ActorName":       auth.ActorName(c),
	}

	return c.Render(http.StatusOK, "orders.html", data)
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
		"Order":           order,
		"products":        products,
		"quantities":      quantities,
		"CanAcceptDineIn": models.CanAcceptDineIn(),
		"InitialItemsB64": base64.StdEncoding.EncodeToString(itemsJSON),
		"IsAdmin":         auth.IsAdminSession(c),
		"ActorName":       auth.ActorName(c),
	}

	return c.Render(http.StatusOK, "edit_order.html", data)
}

// products controllers
func RenderProducts(c *echo.Context) error {
	products := models.FetchProducts()
	business, _ := models.GetBusiness()

	data := map[string]any{
		"Business":        business,
		"products":        products,
		"CanAcceptDineIn": models.CanAcceptDineIn(),
		"IsAdmin":         auth.IsAdminSession(c),
		"ActorName":       auth.ActorName(c),
	}

	return c.Render(http.StatusOK, "products.html", data)
}

// RenderNewProduct shows the "add product" form.
func RenderNewProduct(c *echo.Context) error {
	return c.Render(http.StatusOK, "product_new.html", map[string]any{
		"Error":     "",
		"IsAdmin":   auth.IsAdminSession(c),
		"ActorName": auth.ActorName(c),
	})
}

// tables controllers
func RenderTables(c *echo.Context) error {
	orders := models.FetchOrders()
	tables := models.FetchTables()
	business, _ := models.GetBusiness()

	data := map[string]any{
		"Business":        business,
		"orders":          orders,
		"tables":          tables,
		"CanAcceptDineIn": models.CanAcceptDineIn(),
		"IsAdmin":         auth.IsAdminSession(c),
		"ActorName":       auth.ActorName(c),
	}

	return c.Render(http.StatusOK, "tables.html", data)
}

// RenderNewTable shows the "add table" form.
func RenderNewTable(c *echo.Context) error {
	return c.Render(http.StatusOK, "table_new.html", map[string]any{
		"Error":     "",
		"IsAdmin":   auth.IsAdminSession(c),
		"ActorName": auth.ActorName(c),
	})
}

// RenderReservations lists every reservation.
func RenderReservations(c *echo.Context) error {
	reservations := models.FetchReservations()
	business, _ := models.GetBusiness()

	return c.Render(http.StatusOK, "reservations.html", map[string]any{
		"reservations":    reservations,
		"Business":        business,
		"CanAcceptDineIn": models.CanAcceptDineIn(),
		"IsAdmin":         auth.IsAdminSession(c),
		"ActorName":       auth.ActorName(c),
	})
}

// RenderNewReservation shows the "book a table" form, along with the
// current table list so staff can see capacities at a glance.
func RenderNewReservation(c *echo.Context) error {
	tables := models.FetchTables()
	business, _ := models.GetBusiness()

	return c.Render(http.StatusOK, "reservation_new.html", map[string]any{
		"tables":          tables,
		"Business":        business,
		"CanAcceptDineIn": models.CanAcceptDineIn(),
		"eventTypes":      models.EventTypes,
		"Error":           "",
		"IsAdmin":         auth.IsAdminSession(c),
		"ActorName":       auth.ActorName(c),
	})
}
