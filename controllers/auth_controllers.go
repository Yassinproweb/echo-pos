package controllers

import (
	"net/http"

	"github.com/Yassinproweb/echo-pos/auth"
	"github.com/Yassinproweb/echo-pos/models"
	"github.com/labstack/echo/v5"
)

// RenderRegister shows the one-time business registration form. If a
// business already exists, registration is closed and we send the visitor
// to the login page instead.
func RenderRegister(c *echo.Context) error {
	if models.BusinessExists() {
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	return c.Render(http.StatusOK, "register.html", map[string]any{
		"Error": "",
	})
}

// RenderLogin shows the cashier sign-up/login form (plus an Admin tab). If
// no business has been registered yet there's nothing to log into, so we
// send visitors to register first.
func RenderLogin(c *echo.Context) error {
	if !models.BusinessExists() {
		return c.Redirect(http.StatusSeeOther, "/register")
	}

	return c.Render(http.StatusOK, "login.html", map[string]any{
		"Error": "",
	})
}

// RenderAdmin shows the admin-only business/password settings page.
func RenderAdmin(c *echo.Context) error {
	business, err := models.GetBusiness()
	if err != nil || business == nil {
		return c.String(http.StatusInternalServerError, "Could not load business details")
	}

	return c.Render(http.StatusOK, "admin.html", map[string]any{
		"Business":  business,
		"Error":     "",
		"Success":   "",
		"IsAdmin":   true,
		"ActorName": auth.ActorName(c),
	})
}

// RenderReservations lists every reservation.
func RenderReservations(c *echo.Context) error {
	reservations := models.FetchReservations()

	return c.Render(http.StatusOK, "reservations.html", map[string]any{
		"reservations": reservations,
		"IsAdmin":      auth.IsAdminSession(c),
		"ActorName":    auth.ActorName(c),
	})
}

// RenderNewReservation shows the "book a table" form, along with the
// current table list so staff can see capacities at a glance.
func RenderNewReservation(c *echo.Context) error {
	tables := models.FetchTables()

	return c.Render(http.StatusOK, "reservation_new.html", map[string]any{
		"tables":     tables,
		"eventTypes": models.EventTypes,
		"Error":      "",
		"IsAdmin":    auth.IsAdminSession(c),
		"ActorName":  auth.ActorName(c),
	})
}

// RenderNewProduct shows the "add product" form.
func RenderNewProduct(c *echo.Context) error {
	return c.Render(http.StatusOK, "product_new.html", map[string]any{
		"Error":     "",
		"IsAdmin":   auth.IsAdminSession(c),
		"ActorName": auth.ActorName(c),
	})
}

// RenderNewTable shows the "add table" form.
func RenderNewTable(c *echo.Context) error {
	return c.Render(http.StatusOK, "table_new.html", map[string]any{
		"Error":     "",
		"IsAdmin":   auth.IsAdminSession(c),
		"ActorName": auth.ActorName(c),
	})
}
