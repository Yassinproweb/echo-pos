package controllers

import (
	"net/http"

	"github.com/Yassinproweb/echo-pos/models"
	"github.com/labstack/echo/v5"
)

// orders controllers
func RenderOrders(c *echo.Context) error {
	tables := models.FetchTables()

	canDineIn := models.CanAcceptDineIn(tables)

	orders := models.FetchOrders()
	for i := range orders {
		orders[i].CalculateOrderTotal()
	}

	models.AssignOrderDestination(orders, tables)

	data := map[string]any{
		"orders":    orders,
		"tables":    tables,
		"canDineIn": canDineIn,
	}

	return c.Render(http.StatusOK, "orders.html", data)
}

// tables controllers
func RenderTables(c *echo.Context) error {
	orders := models.FetchOrders()
	tables := models.FetchTables()

	models.AssignOrderDestination(orders, tables)

	return c.Render(http.StatusOK, "tables.html", tables)
}
