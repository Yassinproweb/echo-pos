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
