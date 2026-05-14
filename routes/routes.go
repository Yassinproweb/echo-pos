package routes

import (
	"net/http"

	"github.com/Yassinproweb/echo-pos/models"
	"github.com/labstack/echo/v5"
)

func SelectOrderRoute(c *echo.Context) error {
	orderID := c.Param("id")

	// Fetch your orders slice
	orders := models.FetchOrders()

	var so *models.Order
	for i := range orders {
		// Matching the ID (e.g., "#ORD0011")
		if orders[i].Name == orderID {
			so = &orders[i]
			break
		}
	}

	if so == nil {
		return c.String(http.StatusNotFound, "Order Not Found")
	}

	// CRITICAL: Calculate totals from the OrderCart before rendering
	err := so.CalculateOrderTotal()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Calculation Error")
	}

	// Render ONLY the receipt_card partial
	return c.Render(http.StatusOK, "receipt", so)
}

func UpdateStatusAfterPrint(c *echo.Context) error {
	orderID := c.Param("id")
	orderType := c.FormValue("type")

	orders := models.FetchOrders() //
	var selectedOrder *models.Order

	for i := range orders {
		if orders[i].Name == orderID {
			// Determine new status based on logic provided
			switch orderType {
			case "DineIn":
				orders[i].Status = "Served"
			case "TakeAway":
				orders[i].Status = "Taken"
			case "Delivery":
				orders[i].Status = "Transit"
			}
			selectedOrder = &orders[i]
			break
		}
	}

	if selectedOrder == nil {
		return c.String(http.StatusNotFound, "Order not found")
	}

	// Return the updated receipt (the button will now be disabled because it's no longer "Ready")
	return c.Render(http.StatusOK, "receipt_card", selectedOrder) //
}
