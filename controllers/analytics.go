package controllers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"time"

	"github.com/Yassinproweb/echo-pos/db"
	"github.com/Yassinproweb/echo-pos/models"
	"github.com/labstack/echo/v5"
)

// ── View-layer types ─────────────────────────────────────────────────────────

// OrderView wraps models.Order and adds a pre-formatted cost string
// so the template can call {{ .CostFormatted }} directly.
type OrderView struct {
	models.Order
	CostFormatted string
}

// ProductStat holds aggregated data for a single product.
type ProductStat struct {
	Name             string
	Image            string
	TotalQty         int
	OrderCount       int
	Revenue          int
	RevenueFormatted string
}

// ── Controller ───────────────────────────────────────────────────────────────

func RenderAnalytics(c *echo.Context) error {
	loc, err := time.LoadLocation("Africa/Kampala")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)

	// ── Orders ──────────────────────────────────────────────────────────────
	orders := models.FetchOrders()
	for i := range orders {
		orders[i].CalculateOrderTotal()
	}

	totalOrders := len(orders)
	totalDelivered := 0
	totalCanceled := 0
	totalRevenue := 0
	dineInCount := 0
	takeawayCount := 0
	deliveryCount := 0

	statusCount := map[string]int{}

	for _, o := range orders {
		totalRevenue += o.Cost
		statusCount[string(o.Status)]++

		switch o.Status {
		case models.Served, models.Taken, models.Delivered:
			totalDelivered++
		case models.Canceled:
			totalCanceled++
		}

		switch o.Type {
		case models.DineIn:
			dineInCount++
		case models.Takeaway:
			takeawayCount++
		case models.Delivery:
			deliveryCount++
		}
	}

	// Type split percentages (avoid division by zero)
	dineInPct, takeawayPct, deliveryPct := 0, 0, 0
	if totalOrders > 0 {
		dineInPct = dineInCount * 100 / totalOrders
		takeawayPct = takeawayCount * 100 / totalOrders
		deliveryPct = deliveryCount * 100 / totalOrders
	}

	// ── All orders as view objects (newest first) ────────────────────────────
	allOrderViews := make([]OrderView, 0, len(orders))
	for i := len(orders) - 1; i >= 0; i-- {
		allOrderViews = append(allOrderViews, OrderView{
			Order:         orders[i],
			CostFormatted: formatCommas(orders[i].Cost),
		})
	}

	// ── 6 most recent for sales history panel ───────────────────────────────
	recentOrders := allOrderViews
	if len(recentOrders) > 6 {
		recentOrders = recentOrders[:6]
	}

	// ── Revenue chart data (all orders, original order) ─────────────────────
	orderNames := make([]string, 0, len(orders))
	orderCosts := make([]int, 0, len(orders))
	for _, o := range orders {
		orderNames = append(orderNames, o.Name)
		orderCosts = append(orderCosts, o.Cost)
	}

	// ── Period series: Weekly / Monthly / Yearly (revenue + order volume) ───
	weeklyLabels, weeklyRevenue, weeklyVolume := buildWeeklySeries(orders, now, loc)
	monthlyLabels, monthlyRevenue, monthlyVolume := buildMonthlySeries(orders, now, loc)
	yearlyLabels, yearlyRevenue, yearlyVolume := buildYearlySeries(orders, now, loc)

	// ── Status distribution for doughnut ────────────────────────────────────
	// Defined order so the chart is always consistent
	statusOrder := []string{
		"Served", "Taken", "Delivered",
		"Placed", "Preparing", "Ready",
		"Waiting", "PickUp", "Transit",
		"Canceled",
	}
	statusLabels := []string{}
	statusCounts := []int{}
	for _, s := range statusOrder {
		if n, ok := statusCount[s]; ok && n > 0 {
			statusLabels = append(statusLabels, s)
			statusCounts = append(statusCounts, n)
		}
	}

	// ── Top 6 products ───────────────────────────────────────────────────────
	type pdtAgg struct {
		qty        int
		orderCount int
		revenue    int
		image      string
	}
	productMap := map[string]*pdtAgg{}
	productImages := buildProductImageMap()

	rows, _ := db.DB.Query(`
		SELECT oi.pdt_name,
		       SUM(oi.quantity)                  AS total_qty,
		       COUNT(DISTINCT oi.order_name)     AS order_count,
		       SUM(oi.quantity * oi.unit_price)  AS revenue
		FROM order_items oi
		GROUP BY oi.pdt_name
	`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var qty, orderCount, revenue int
			if rows.Scan(&name, &qty, &orderCount, &revenue) == nil {
				productMap[name] = &pdtAgg{
					qty:        qty,
					orderCount: orderCount,
					revenue:    revenue,
					image:      productImages[name],
				}
			}
		}
	}

	// Sort by qty descending and take top 5
	type namedPdt struct {
		name string
		agg  *pdtAgg
	}
	allPdts := make([]namedPdt, 0, len(productMap))
	for name, agg := range productMap {
		allPdts = append(allPdts, namedPdt{name, agg})
	}
	sort.Slice(allPdts, func(i, j int) bool {
		return allPdts[i].agg.qty > allPdts[j].agg.qty
	})
	if len(allPdts) > 6 {
		allPdts = allPdts[:6]
	}

	topProducts := make([]ProductStat, 0, len(allPdts))
	for _, np := range allPdts {
		topProducts = append(topProducts, ProductStat{
			Name:             np.name,
			Image:            np.agg.image,
			TotalQty:         np.agg.qty,
			OrderCount:       np.agg.orderCount,
			Revenue:          np.agg.revenue,
			RevenueFormatted: formatCommas(np.agg.revenue),
		})
	}

	// ── Tables ───────────────────────────────────────────────────────────────
	tables := models.FetchTables()
	totalTables := len(tables)
	availTables := 0
	occupTables := 0
	pendTables := 0
	for _, t := range tables {
		switch t.State {
		case models.Available:
			availTables++
		case models.Occupied:
			occupTables++
		case models.Pending:
			pendTables++
		}
	}

	// ── Products count ───────────────────────────────────────────────────────
	products := models.FetchProducts()
	totalProducts := len(products)

	// ── JSON for Chart.js ────────────────────────────────────────────────────
	orderNamesJSON, _ := json.Marshal(orderNames)
	orderCostsJSON, _ := json.Marshal(orderCosts)
	statusLabelsJSON, _ := json.Marshal(statusLabels)
	statusCountsJSON, _ := json.Marshal(statusCounts)

	weeklyLabelsJSON, _ := json.Marshal(weeklyLabels)
	weeklyRevenueJSON, _ := json.Marshal(weeklyRevenue)
	weeklyVolumeJSON, _ := json.Marshal(weeklyVolume)

	monthlyLabelsJSON, _ := json.Marshal(monthlyLabels)
	monthlyRevenueJSON, _ := json.Marshal(monthlyRevenue)
	monthlyVolumeJSON, _ := json.Marshal(monthlyVolume)

	yearlyLabelsJSON, _ := json.Marshal(yearlyLabels)
	yearlyRevenueJSON, _ := json.Marshal(yearlyRevenue)
	yearlyVolumeJSON, _ := json.Marshal(yearlyVolume)

	return c.Render(http.StatusOK, "analytics.html", map[string]any{
		// head.html requires these two keys
		"user":            "Cashier Admin",
		"CanAcceptDineIn": models.CanAcceptDineIn(),

		// Page header
		"GeneratedAt": now.Format("Mon, 02 Jan 2006 · 15:04"),

		// KPI cards
		"TotalOrders":           totalOrders,
		"TotalDelivered":        totalDelivered,
		"TotalCanceled":         totalCanceled,
		"TotalRevenueFormatted": "UGX " + formatCommas(totalRevenue),

		// Order type split
		"DineInCount":   dineInCount,
		"TakeawayCount": takeawayCount,
		"DeliveryCount": deliveryCount,
		"DineInPct":     dineInPct,
		"TakeawayPct":   takeawayPct,
		"DeliveryPct":   deliveryPct,

		// Lists
		"AllOrders":    allOrderViews,
		"RecentOrders": recentOrders,
		"TopProducts":  topProducts,

		// Tables
		"TotalTables":     totalTables,
		"AvailableTables": availTables,
		"OccupiedTables":  occupTables,
		"PendingTables":   pendTables,
		"TotalProducts":   totalProducts,

		// Chart.js JSON blobs (template injects them as JS literals)
		"OrderNamesJSON":   template.JS(string(orderNamesJSON)),
		"OrderCostsJSON":   template.JS(string(orderCostsJSON)),
		"StatusLabelsJSON": template.JS(string(statusLabelsJSON)),
		"StatusCountsJSON": template.JS(string(statusCountsJSON)),

		"WeeklyLabelsJSON":  template.JS(string(weeklyLabelsJSON)),
		"WeeklyRevenueJSON": template.JS(string(weeklyRevenueJSON)),
		"WeeklyVolumeJSON":  template.JS(string(weeklyVolumeJSON)),

		"MonthlyLabelsJSON":  template.JS(string(monthlyLabelsJSON)),
		"MonthlyRevenueJSON": template.JS(string(monthlyRevenueJSON)),
		"MonthlyVolumeJSON":  template.JS(string(monthlyVolumeJSON)),

		"YearlyLabelsJSON":  template.JS(string(yearlyLabelsJSON)),
		"YearlyRevenueJSON": template.JS(string(yearlyRevenueJSON)),
		"YearlyVolumeJSON":  template.JS(string(yearlyVolumeJSON)),
	})
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// formatCommas turns 1410000 into "1,410,000".
func formatCommas(n int) string {
	s := fmt.Sprintf("%d", n)
	out := ""
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += ","
		}
		out += string(ch)
	}
	return out
}

// buildWeeklySeries buckets orders into the current week (Sun..Sat),
// returning 3-letter day labels, revenue totals and order-volume totals per day.
func buildWeeklySeries(orders []models.Order, now time.Time, loc *time.Location) ([]string, []int, []int) {
	dayLabels := []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}

	// Find the Sunday that starts the current week.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	offset := int(today.Weekday()) // Sunday = 0
	weekStart := today.AddDate(0, 0, -offset)

	revenue := make([]int, 7)
	volume := make([]int, 7)

	for _, o := range orders {
		dt := o.DateTime.In(loc)
		// Derive the bucket from the calendar day (avoids DST half-day rounding issues).
		dtDay := time.Date(dt.Year(), dt.Month(), dt.Day(), 0, 0, 0, 0, loc)
		idx := int(dtDay.Sub(weekStart).Hours() / 24)
		if idx < 0 || idx > 6 {
			continue
		}
		revenue[idx] += o.Cost
		volume[idx]++
	}

	return dayLabels, revenue, volume
}

// buildMonthlySeries buckets orders into the current month, day-by-day,
// correctly sizing February for leap vs. non-leap years.
func buildMonthlySeries(orders []models.Order, now time.Time, loc *time.Location) ([]string, []int, []int) {
	year, month, _ := now.Date()
	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()

	labels := make([]string, daysInMonth)
	revenue := make([]int, daysInMonth)
	volume := make([]int, daysInMonth)
	for i := 0; i < daysInMonth; i++ {
		labels[i] = fmt.Sprintf("%d", i+1)
	}

	for _, o := range orders {
		dt := o.DateTime.In(loc)
		if dt.Year() != year || dt.Month() != month {
			continue
		}
		idx := dt.Day() - 1
		if idx < 0 || idx >= daysInMonth {
			continue
		}
		revenue[idx] += o.Cost
		volume[idx]++
	}

	return labels, revenue, volume
}

// buildYearlySeries buckets orders into the current year, month-by-month.
func buildYearlySeries(orders []models.Order, now time.Time, loc *time.Location) ([]string, []int, []int) {
	monthLabels := []string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}
	year := now.Year()

	revenue := make([]int, 12)
	volume := make([]int, 12)

	for _, o := range orders {
		dt := o.DateTime.In(loc)
		if dt.Year() != year {
			continue
		}
		idx := int(dt.Month()) - 1
		revenue[idx] += o.Cost
		volume[idx]++
	}

	return monthLabels, revenue, volume
}

// buildProductImageMap returns a name→image-path map from the products table.
func buildProductImageMap() map[string]string {
	m := map[string]string{}
	rows, err := db.DB.Query(`SELECT name, image FROM products`)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var name, image string
		if rows.Scan(&name, &image) == nil {
			m[name] = image
		}
	}
	return m
}
