package routes

import (
	"net/http"

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

	// READ THE TYPE FROM HTMX
	orderType := models.Type(c.FormValue("type"))

	orders := models.FetchOrders()
	var selectedOrder *models.Order

	for i := range orders {
		if orders[i].Name == orderID {

			switch orderType {
			case models.DineIn:
				orders[i].Status = models.Waiting

			case models.Takeaway:
				orders[i].Status = models.PickUp

			case models.Delivery:
				orders[i].Status = models.Transit
			}

			selectedOrder = &orders[i]
			break
		}
	}

	if selectedOrder == nil {
		return c.String(http.StatusNotFound, "Order not found")
	}

	// recalculate totals
	selectedOrder.CalculateOrderTotal()

	return c.Render(http.StatusOK, "receipt", selectedOrder)
}
