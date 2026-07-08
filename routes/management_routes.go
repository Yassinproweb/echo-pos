package routes

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Yassinproweb/echo-pos/auth"
	"github.com/Yassinproweb/echo-pos/models"
	"github.com/labstack/echo/v5"
)

// =======================
// PRODUCTS
// =======================

const uploadsDir = "static/assets/uploads"

// saveUploadedImage saves an uploaded product image (if any) under
// static/assets/uploads and returns its public URL path (e.g.
// "/assets/uploads/169..._burger.jpg"). Returns ("", nil) if no file was
// uploaded, so the caller can fall back to a manually typed image path.
func saveUploadedImage(c *echo.Context, fieldName string) (string, error) {
	fileHeader, err := c.FormFile(fieldName)
	if err != nil {
		// No file provided is not an error — it's optional.
		return "", nil
	}

	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return "", err
	}

	src, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	ext := filepath.Ext(fileHeader.Filename)
	safeName := fmt.Sprintf("%d%s", time.Now().UnixNano(), strings.ToLower(ext))
	destPath := filepath.Join(uploadsDir, safeName)

	dest, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, src); err != nil {
		return "", err
	}

	return "/assets/uploads/" + safeName, nil
}

type CreateProductRequest struct {
	Name        string `form:"name"`
	Description string `form:"description"`
	Price       string `form:"price"`
	Image       string `form:"image"` // manually typed path/URL, used if no file is uploaded
}

func renderProductFormError(c *echo.Context, message string) error {
	return c.Render(http.StatusOK, "product_new.html", map[string]any{
		"Error":     message,
		"IsAdmin":   auth.IsAdminSession(c),
		"ActorName": auth.ActorName(c),
	})
}

// CreateProductRoute adds a new menu item. The image can either be uploaded
// as a file (field "image_file") or given as a path/URL (field "image").
func CreateProductRoute(c *echo.Context) error {
	var req CreateProductRequest
	if err := c.Bind(&req); err != nil {
		return renderProductFormError(c, "Invalid form data. Please try again.")
	}

	price, err := strconv.Atoi(strings.TrimSpace(req.Price))
	if err != nil {
		return renderProductFormError(c, "Price must be a whole number.")
	}

	imagePath, err := saveUploadedImage(c, "image_file")
	if err != nil {
		return renderProductFormError(c, "Could not save the uploaded image: "+err.Error())
	}
	if imagePath == "" {
		imagePath = strings.TrimSpace(req.Image)
	}

	if err := models.CreateProduct(req.Name, req.Description, price, imagePath); err != nil {
		return renderProductFormError(c, err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/pos/products")
}

// =======================
// TABLES
// =======================

type CreateTableRequest struct {
	Capacity string `form:"capacity"`
}

func renderTableFormError(c *echo.Context, message string) error {
	return c.Render(http.StatusOK, "table_new.html", map[string]any{
		"Error":     message,
		"IsAdmin":   auth.IsAdminSession(c),
		"ActorName": auth.ActorName(c),
	})
}

// CreateTableRoute adds a new table. Its name (e.g. #TBL022) is generated
// automatically by the database.
func CreateTableRoute(c *echo.Context) error {
	var req CreateTableRequest
	if err := c.Bind(&req); err != nil {
		return renderTableFormError(c, "Invalid form data. Please try again.")
	}

	capacity, err := strconv.Atoi(strings.TrimSpace(req.Capacity))
	if err != nil {
		return renderTableFormError(c, "Capacity must be a whole number.")
	}

	if _, err := models.CreateTable(capacity); err != nil {
		return renderTableFormError(c, err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/pos/tables")
}

// =======================
// RESERVATIONS
// =======================

type CreateReservationRequest struct {
	EventType         string `form:"event_type"`
	NumPeople         string `form:"num_people"`
	CustName          string `form:"cust_name"`
	CustNumber        string `form:"cust_number"`
	ReservationDate   string `form:"reservation_date"` // yyyy-mm-dd
	ReservationTime   string `form:"reservation_time"` // HH:MM
}

func renderReservationFormError(c *echo.Context, message string) error {
	tables := models.FetchTables()

	return c.Render(http.StatusOK, "reservation_new.html", map[string]any{
		"tables":     tables,
		"eventTypes": models.EventTypes,
		"Error":      message,
		"IsAdmin":    auth.IsAdminSession(c),
		"ActorName":  auth.ActorName(c),
	})
}

// CreateReservationRoute books table(s) for an event based on the number of
// guests, automatically working out how many/which tables are needed.
func CreateReservationRoute(c *echo.Context) error {
	var req CreateReservationRequest
	if err := c.Bind(&req); err != nil {
		return renderReservationFormError(c, "Invalid form data. Please try again.")
	}

	numPeople, err := strconv.Atoi(strings.TrimSpace(req.NumPeople))
	if err != nil || numPeople <= 0 {
		return renderReservationFormError(c, "Number of people must be a whole number greater than zero.")
	}

	dateTimeStr := strings.TrimSpace(req.ReservationDate) + " " + strings.TrimSpace(req.ReservationTime)
	reservationDateTime, err := time.ParseInLocation("2006-01-02 15:04", dateTimeStr, time.Local)
	if err != nil {
		return renderReservationFormError(c, "Please provide a valid reservation date and time.")
	}

	createdBy := auth.ActorName(c)

	_, err = models.CreateReservation(
		req.EventType,
		numPeople,
		req.CustName,
		req.CustNumber,
		reservationDateTime,
		createdBy,
	)

	if err != nil {
		return renderReservationFormError(c, err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/pos/reservations")
}

// CancelReservationRoute frees up any tables reserved for this reservation
// and marks it cancelled. Any logged-in user (admin or cashier) can cancel a
// reservation — only *deleting a canceled order* is admin-only.
func CancelReservationRoute(c *echo.Context) error {
	name := c.Param("name")

	if err := models.CancelReservation(name); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/pos/reservations")
}

// =======================
// ORDERS — ADMIN-ONLY DELETE
// =======================

// DeleteOrderRoute permanently deletes an order. Only reachable by the
// admin (the route is wrapped with auth.RequireAdmin), and only succeeds if
// the order is already Canceled — this is what lets cashiers view canceled
// orders without being able to remove them.
func DeleteOrderRoute(c *echo.Context) error {
	orderID := c.Param("id")

	if err := models.DeleteCanceledOrder(orderID); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/pos/orders")
}
