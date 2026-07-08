package routes

import (
	"net/http"

	"github.com/Yassinproweb/echo-pos/auth"
	"github.com/Yassinproweb/echo-pos/models"
	"github.com/labstack/echo/v5"
)

// =======================
// BUSINESS REGISTRATION
// =======================

type RegisterRequest struct {
	BusinessName           string `form:"business_name"`
	RestaurantName         string `form:"restaurant_name"`
	Location               string `form:"location"`
	AdminPassword          string `form:"admin_password"`
	AdminPasswordConfirm   string `form:"admin_password_confirm"`
	CashierPassword        string `form:"cashier_password"`
	CashierPasswordConfirm string `form:"cashier_password_confirm"`
}

func renderRegisterError(c *echo.Context, message string) error {
	return c.Render(http.StatusOK, "register.html", map[string]any{
		"Error": message,
	})
}

// RegisterBusinessRoute creates the single business record for this
// installation. It can only ever succeed once.
func RegisterBusinessRoute(c *echo.Context) error {
	if models.BusinessExists() {
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return renderRegisterError(c, "Invalid form data. Please try again.")
	}

	if req.AdminPassword != req.AdminPasswordConfirm {
		return renderRegisterError(c, "Admin password and confirmation do not match.")
	}

	if req.CashierPassword != req.CashierPasswordConfirm {
		return renderRegisterError(c, "Cashier password and confirmation do not match.")
	}

	err := models.RegisterBusiness(
		req.BusinessName,
		req.RestaurantName,
		req.Location,
		req.AdminPassword,
		req.CashierPassword,
	)

	if err != nil {
		return renderRegisterError(c, err.Error())
	}

	auth.IssueSession(c, auth.RoleAdmin, "")

	return c.Redirect(http.StatusSeeOther, "/pos")
}

// =======================
// CASHIER SIGN-UP / LOGIN
// =======================

type CashierAuthRequest struct {
	CashierName string `form:"cashier_name"`
	Password    string `form:"password"`
}

func renderLoginError(c *echo.Context, message string) error {
	return c.Render(http.StatusOK, "login.html", map[string]any{
		"Error": message,
	})
}

// CashierAuthRoute handles the combined cashier sign-up/login form. If the
// submitted name has never signed up before, it signs them up; if it has,
// it logs them in. Either way, the password must match the single cashier
// password the admin has configured — cashiers never set their own
// password.
func CashierAuthRoute(c *echo.Context) error {
	if !models.BusinessExists() {
		return c.Redirect(http.StatusSeeOther, "/register")
	}

	var req CashierAuthRequest
	if err := c.Bind(&req); err != nil {
		return renderLoginError(c, "Invalid form data. Please try again.")
	}

	if req.CashierName == "" || req.Password == "" {
		return renderLoginError(c, "Name and password are required.")
	}

	ok, err := models.VerifyCashierPassword(req.Password)
	if err != nil {
		return renderLoginError(c, "Something went wrong. Please try again.")
	}
	if !ok {
		return renderLoginError(c, "Incorrect cashier password.")
	}

	exists, err := models.CashierExists(req.CashierName)
	if err != nil {
		return renderLoginError(c, "Something went wrong. Please try again.")
	}

	if !exists {
		if err := models.RegisterCashier(req.CashierName); err != nil {
			return renderLoginError(c, "Could not sign you up: "+err.Error())
		}
	}

	auth.IssueSession(c, auth.RoleCashier, req.CashierName)

	return c.Redirect(http.StatusSeeOther, "/pos")
}

// =======================
// ADMIN LOGIN
// =======================

type AdminAuthRequest struct {
	Password string `form:"admin_password"`
}

// AdminLoginRoute logs the admin back in with just the admin password (the
// admin doesn't sign up — that happens once, at business registration —
// they only ever log back in afterwards).
func AdminLoginRoute(c *echo.Context) error {
	if !models.BusinessExists() {
		return c.Redirect(http.StatusSeeOther, "/register")
	}

	var req AdminAuthRequest
	if err := c.Bind(&req); err != nil {
		return renderLoginError(c, "Invalid form data. Please try again.")
	}

	if req.Password == "" {
		return renderLoginError(c, "Admin password is required.")
	}

	ok, err := models.VerifyAdminPassword(req.Password)
	if err != nil {
		return renderLoginError(c, "Something went wrong. Please try again.")
	}
	if !ok {
		return renderLoginError(c, "Incorrect admin password.")
	}

	auth.IssueSession(c, auth.RoleAdmin, "")

	return c.Redirect(http.StatusSeeOther, "/pos")
}

// LogoutRoute clears the session cookie and returns to the login page.
func LogoutRoute(c *echo.Context) error {
	auth.ClearSession(c)
	return c.Redirect(http.StatusSeeOther, "/login")
}

// =======================
// ADMIN SETTINGS
// =======================

type UpdateBusinessRequest struct {
	BusinessName           string `form:"business_name"`
	RestaurantName         string `form:"restaurant_name"`
	Location               string `form:"location"`
	AdminPassword          string `form:"admin_password"`
	AdminPasswordConfirm   string `form:"admin_password_confirm"`
	CashierPassword        string `form:"cashier_password"`
	CashierPasswordConfirm string `form:"cashier_password_confirm"`
}

func renderAdminMessage(c *echo.Context, errMsg, successMsg string) error {
	business, _ := models.GetBusiness()
	return c.Render(http.StatusOK, "admin.html", map[string]any{
		"Business":  business,
		"Error":     errMsg,
		"Success":   successMsg,
		"IsAdmin":   true,
		"ActorName": auth.ActorName(c),
	})
}

// UpdateBusinessRoute lets the admin (and only the admin — enforced by the
// RequireAdmin middleware on this route) update the business/restaurant
// details and, optionally, the admin and/or cashier passwords. Password
// fields are only changed when both the field and its confirmation are
// filled in, so leaving them blank never wipes an existing password.
func UpdateBusinessRoute(c *echo.Context) error {
	var req UpdateBusinessRequest
	if err := c.Bind(&req); err != nil {
		return renderAdminMessage(c, "Invalid form data. Please try again.", "")
	}

	if err := models.UpdateBusinessDetails(req.BusinessName, req.RestaurantName, req.Location); err != nil {
		return renderAdminMessage(c, err.Error(), "")
	}

	if req.AdminPassword != "" || req.AdminPasswordConfirm != "" {
		if req.AdminPassword != req.AdminPasswordConfirm {
			return renderAdminMessage(c, "New admin password and confirmation do not match.", "")
		}
		if err := models.UpdateAdminPassword(req.AdminPassword); err != nil {
			return renderAdminMessage(c, err.Error(), "")
		}
	}

	if req.CashierPassword != "" || req.CashierPasswordConfirm != "" {
		if req.CashierPassword != req.CashierPasswordConfirm {
			return renderAdminMessage(c, "New cashier password and confirmation do not match.", "")
		}
		if err := models.UpdateCashierPassword(req.CashierPassword); err != nil {
			return renderAdminMessage(c, err.Error(), "")
		}
	}

	return renderAdminMessage(c, "", "Business details updated successfully.")
}
