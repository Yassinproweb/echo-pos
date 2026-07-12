package main

import (
	"embed"
	"html/template"
	"io"
	"net/http"

	"github.com/Yassinproweb/echo-pos/auth"
	"github.com/Yassinproweb/echo-pos/controllers"
	"github.com/Yassinproweb/echo-pos/db"
	"github.com/Yassinproweb/echo-pos/models"
	"github.com/Yassinproweb/echo-pos/routes"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

//go:embed views/*
var templateFS embed.FS

// TemplateRenderer is a custom html/template renderer for Echo framework
type TemplateRenderer struct {
	templates *template.Template
}

func (t *TemplateRenderer) Render(c *echo.Context, w io.Writer, name string, data any) error {
	if viewContext, isMap := data.(map[string]any); isMap {
		viewContext["reverse"] = c.RouteInfo().Reverse
	}
	return t.templates.ExecuteTemplate(w, name, data)
}

func main() {
	db.ConnectDB()
	e := echo.New()

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	e.Static("/", "static")

	tmpl := template.Must(template.ParseFS(
		templateFS,
		"views/*.html",          // index.html and main.html
		"views/partials/*.html", // POS UI partials
		"views/others/*.html",   // POS UI subpages
	))

	e.Renderer = &TemplateRenderer{
		templates: tmpl,
	}

	// Route for the SaaS Landing Page
	e.GET("/", func(c *echo.Context) error {
		return c.Render(http.StatusOK, "index.html", nil)
	})

	// =======================
	// BUSINESS REGISTRATION (one-time)
	// =======================
	e.GET("/register", controllers.RenderRegister)
	e.POST("/register", routes.RegisterBusinessRoute)

	// =======================
	// LOGIN (cashier sign-up/login tab + admin tab)
	// =======================
	e.GET("/login", controllers.RenderLogin)
	e.POST("/login", routes.CashierAuthRoute)
	e.POST("/login/admin", routes.AdminLoginRoute)

	// =======================
	// POS UI — everything below requires a valid session (admin or cashier)
	// =======================
	pos := e.Group("/pos", auth.RequireLogin)

	pos.GET("/logout", routes.LogoutRoute)

	pos.GET("", func(c *echo.Context) error {
		products := models.FetchProducts()
		tables := models.FetchTables()

		orders := models.FetchOrders()

		for i := range orders {
			orders[i].CalculateOrderTotal()
		}

		business, _ := models.GetBusiness()

		return c.Render(http.StatusOK, "main.html", map[string]any{
			"orders":          orders,
			"products":        products,
			"tables":          tables,
			"CanAcceptDineIn": models.CanAcceptDineIn(),
			"selectedOrder":   nil,
			"Business":        business,
			"IsAdmin":         auth.IsAdminSession(c),
			"ActorName":       auth.ActorName(c),
		})
	})

	// Orders
	pos.GET("/orders", controllers.RenderOrders)
	pos.POST("/orders/create", routes.CreateOrder)
	pos.GET("/order/:id", routes.SelectOrderRoute)
	pos.GET("/order/:id/edit", controllers.RenderEditOrder)
	pos.POST("/order/:id/items", routes.UpdateOrderItemsRoute)
	pos.POST("/order/:id/cancel", routes.CancelOrderRoute)
	pos.POST("/order/update-status/:id", routes.UpdateStatusAfterPrint)
	// Deleting an order is only ever allowed if it's Canceled (enforced in
	// models.DeleteCanceledOrder), and only the admin may do it at all.
	pos.POST("/orders/:id/delete", routes.DeleteOrderRoute, auth.RequireAdmin)

	// Products
	pos.GET("/products", controllers.RenderProducts)
	pos.GET("/products/new", controllers.RenderNewProduct)
	pos.POST("/products/new", routes.CreateProductRoute)

	// Tables
	pos.GET("/tables", controllers.RenderTables)
	pos.GET("/tables/new", controllers.RenderNewTable)
	pos.POST("/tables/new", routes.CreateTableRoute)

	// Analytics
	pos.GET("/analytics", controllers.RenderAnalytics)

	// Reservations (birthdays, weddings, conferences, etc.)
	pos.GET("/reservations", controllers.RenderReservations)
	pos.GET("/reservations/new", controllers.RenderNewReservation)
	pos.POST("/reservations", routes.CreateReservationRoute)
	pos.POST("/reservations/:name/cancel", routes.CancelReservationRoute)

	// Admin-only: business details + admin/cashier passwords
	pos.GET("/admin", controllers.RenderAdmin, auth.RequireAdmin)
	pos.POST("/admin", routes.UpdateBusinessRoute, auth.RequireAdmin)

	if err := e.Start(":5000"); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}
