package main

import (
	"embed"
	"html/template"
	"io"
	"net/http"

	"github.com/Yassinproweb/echo-pos/controllers"
	"github.com/Yassinproweb/echo-pos/models"
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

		canDineIn := models.CanAcceptDineIn(tables)

		orders := models.FetchOrders()
		for i := range orders {
			orders[i].CalculateOrderTotal()
		}

		models.AssignOrderDestination(orders, tables)

		return c.Render(http.StatusOK, "main.html", map[string]any{
			"user":      "Cashier Admin",
			"orders":    orders,
			"products":  products,
			"tables":    tables,
			"canDineIn": canDineIn,
		})
	})

	// Route for the POS UI orders page
	e.GET("/pos/orders", controllers.RenderOrders)

	// Route for the POS UI orders page
	e.GET("/pos/tables", controllers.RenderTables)

	// Helper function to convert DATETIME to DD-MM-YYYY HH:MM
	// convertFromSQLiteDateTime := func(sqliteDateTime string) (string, error) {
	// 	parsed, err := time.Parse("2006-01-02 15:04:05", sqliteDateTime)
	// 	if err != nil {
	// 		return "", fmt.Errorf("invalid SQLite date_time: %v", err)
	// 	}
	// 	return parsed.Format("02-01-2006 15:04"), nil
	// }

	if err := e.Start(":4000"); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}
