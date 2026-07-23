package routes

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Yassinproweb/echo-pos/auth"
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

// UpdateStatusAfterPrint handles the FIRST print of a receipt (the kitchen
// ticket), done right after an order is created (still Placed). Printing it
// automatically moves the order into the kitchen queue: Placed -> Preparing.
func UpdateStatusAfterPrint(c *echo.Context) error {
	orderID := c.Param("id")

	if err := models.UpdateOrderStatus(orderID, models.Preparing); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to update status")
	}

	return c.Render(http.StatusOK, "order_form", nil)
}

// UpdateStatusAfterViewPrint renders the actual customer receipt while
// VIEWING an order that is already Ready. It does NOT change status itself
// — the order is already Ready by the time this button is shown. The 15s
// auto-advance (Ready -> Waiting/PickUp/Transit) is triggered separately by
// AdvanceStatusAfterViewPrint once the print completes.
func UpdateStatusAfterViewPrint(c *echo.Context) error {
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
	if so.Status != models.Ready {
		return c.String(http.StatusBadRequest, "Order must be Ready to print the final receipt")
	}

	_ = so.CalculateOrderTotal()
	return c.Render(http.StatusOK, "receipt", so)
}

// AdvanceStatusAfterViewPrint is called by the client ~15s after the actual
// receipt print completes. It moves the order from Ready to the
// type-specific next status: DineIn -> Waiting, Takeaway -> PickUp,
// Delivery -> Transit. It's a no-op if the order has moved on already (e.g.
// canceled, or advanced manually in the meantime).
func AdvanceStatusAfterViewPrint(c *echo.Context) error {
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

	if so.Status != models.Ready {
		// Status already moved on (canceled, or changed manually) — do nothing.
		return c.NoContent(http.StatusOK)
	}

	next := models.NextStatus(so.Type, models.Ready)
	if next == "" {
		return c.String(http.StatusInternalServerError, "No next status defined for order type")
	}

	if err := models.UpdateOrderStatus(orderID, next); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to update status")
	}

	return c.NoContent(http.StatusOK)
}

// ManualUpdateOrderStatus lets a cashier/admin manually advance OR revert an
// order's status by exactly one step. It strictly enforces the sequence for
// the order's type (Placed -> Preparing -> Ready -> [Waiting|PickUp|Transit]
// -> [Served|Taken|Delivered]) in either direction — it rejects any attempt
// to skip a step. Returns the updated receipt fragment so callers can swap
// it directly into whichever container is showing the order.
func ManualUpdateOrderStatus(c *echo.Context) error {
	orderID := c.Param("id")
	target := models.Status(c.FormValue("status"))

	if target == "" {
		return c.String(http.StatusBadRequest, "Missing target status")
	}

	if err := models.UpdateOrderStatusManual(orderID, target); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

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

	if err := so.CalculateOrderTotal(); err != nil {
		return c.String(http.StatusInternalServerError, "Calculation Error")
	}

	return c.Render(http.StatusOK, "receipt", so)
}

// ManualUpdateTableStatus lets a cashier/admin manually move a table one
// step forward or backward around its fixed state cycle:
// Available -> Occupied -> Pending -> Available. This is the ONLY way a
// table's state changes — there is no automatic/print-driven trigger for
// it. The route itself is inside the /pos group, which already requires a
// logged-in session (cashier or admin) via auth.RequireLogin, so no
// separate customer-facing path can reach this.
//
// Returns both the updated table_row and table_card as out-of-band swaps,
// since both may be visible on the tables page at once; the caller's own
// hx-target/hx-swap can be "none" since everything updates via OOB.
func ManualUpdateTableStatus(c *echo.Context) error {
	tableID := c.Param("id")
	target := models.State(c.FormValue("status"))

	if target == "" {
		return c.String(http.StatusBadRequest, "Missing target state")
	}

	tbl, err := models.UpdateTableStateManual(tableID, target)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	return c.Render(http.StatusOK, "table_status_fragment", tbl)
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

	cashierName := auth.ActorName(c)

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
		INSERT INTO orders (type, status, cust_name, cust_number, destination, cashier_name)
		VALUES (?, ?, ?, ?, ?, ?)
	`, order.Type, order.Status, order.CustName, order.CustNumber, order.Destination, cashierName)

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
	order.CashierName = sql.NullString{String: cashierName, Valid: cashierName != ""}

	return c.Render(http.StatusCreated, "order_created", map[string]any{
		"Toast":   map[string]any{"OrderName": orderName},
		"Receipt": order,
	})
}

// UpdateOrderItemsRoute lets the cashier swap items in an order that hasn't
// reached Ready yet.
func UpdateOrderItemsRoute(c *echo.Context) error {
	orderID := c.Param("id")
	itemsJSON := c.FormValue("items")

	var items []models.OrderItem
	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return c.String(http.StatusBadRequest, "Invalid items JSON: "+err.Error())
	}

	if err := models.UpdateOrderItems(orderID, items); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/pos/orders")
}

// CancelOrderRoute cancels an order that can no longer have its items
// swapped (already Ready or beyond).
func CancelOrderRoute(c *echo.Context) error {
	orderID := c.Param("id")
	if err := models.CancelOrder(orderID); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/pos/orders")
}
