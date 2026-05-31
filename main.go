package main

import (
	"embed"
	"html/template"
	"io"
	"net/http"

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

	// Route for the POS UI
	e.GET("/pos", func(c *echo.Context) error {
		products := models.FetchProducts()
		tables := models.FetchTables()

		orders := models.FetchOrders()

		for i := range orders {
			orders[i].CalculateOrderTotal()
		}

		selectedOrder := orders[0]
		selectedOrder.CalculateOrderTotal()

		return c.Render(http.StatusOK, "main.html", map[string]any{
			"user":            "Cashier Admin",
			"orders":          orders,
			"products":        products,
			"tables":          tables,
			"CanAcceptDineIn": models.CanAcceptDineIn(),
			"selectedOrder":   nil,
		})
	})

	// Route for the POS UI orders page
	e.GET("/pos/orders", controllers.RenderOrders)

	e.POST("/pos/orders/create", routes.CreateOrder)

	// Route for the POS UI orders page
	e.GET("/pos/products", controllers.RenderProducts)

	// Route for the POS UI orders page
	e.GET("/pos/tables", controllers.RenderTables)

	e.GET("/pos/order/:id", routes.SelectOrderRoute)
	e.POST("/pos/order/update-status/:id", routes.UpdateStatusAfterPrint)

	if err := e.Start(":4000"); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}
