package controllers

import (
	"net/http"

	"github.com/Yassinproweb/echo-pos/models"
	"github.com/labstack/echo/v5"
)

// orders controllers
func RenderOrders(c *echo.Context) error {
	tables := models.FetchTables()

	orders := models.FetchOrders()
	for i := range orders {
		orders[i].CalculateOrderTotal()
	}

	data := map[string]any{
		"orders": orders,
		"tables": tables,
	}

	return c.Render(http.StatusOK, "orders.html", data)
}

// products controllers
func RenderProducts(c *echo.Context) error {
	products := models.FetchProducts()

	data := map[string]any{
		"products": products,
	}

	return c.Render(http.StatusOK, "products.html", data)
}

// tables controllers
func RenderTables(c *echo.Context) error {
	orders := models.FetchOrders()
	tables := models.FetchTables()

	data := map[string]any{
		"orders": orders,
		"tables": tables,
	}

	return c.Render(http.StatusOK, "tables.html", data)
}
