package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
)

const (
	cookieName      = "pos_session"
	sessionDuration = 12 * time.Hour
	secretKeyPath   = "session.key"

	RoleAdmin   = "admin"
	RoleCashier = "cashier"

	// contextKey is the key used with c.Set/c.Get to pass the parsed
	// session from RequireLogin/RequireAdmin to the route handler.
	contextKey = "session"
)

var secretKey []byte

func init() {
	secretKey = loadOrCreateSecret()
}

// loadOrCreateSecret loads the HMAC signing key from disk, or generates and
// persists a new random one on first run. Persisting it means restarting the
// server doesn't immediately log everyone out.
func loadOrCreateSecret() []byte {
	if data, err := os.ReadFile(secretKeyPath); err == nil {
		if decoded, derr := hex.DecodeString(strings.TrimSpace(string(data))); derr == nil && len(decoded) > 0 {
			return decoded
		}
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		// Extremely unlikely, but fall back to a fixed key rather than
		// crash the whole app — sessions just won't survive a restart.
		return []byte("insecure-fallback-key-change-me")
	}

	_ = os.WriteFile(secretKeyPath, []byte(hex.EncodeToString(key)), 0600)
	return key
}

// Session represents a logged-in user: either the single admin, or a named
// cashier.
type Session struct {
	Role           string
	CashierName    string
	CashierContact string
}

func (s Session) IsAdmin() bool {
	return s.Role == RoleAdmin
}

func sign(payload string) string {
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// IssueSession sets a signed cookie identifying the logged-in role/cashier.
func IssueSession(c *echo.Context, role, cashierName string, cashierContact string) {
	expires := time.Now().Add(sessionDuration).Unix()
	payload := fmt.Sprintf("%s|%s|%s|%d", role, cashierName, cashierContact, expires)
	sig := sign(payload)
	value := base64.URLEncoding.EncodeToString([]byte(payload)) + "." + sig

	c.SetCookie(&http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionDuration),
	})
}

// ClearSession removes the session cookie (logout).
func ClearSession(c *echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// ReadSession parses and validates the session cookie, returning nil if
// there isn't one, it's malformed, tampered with, or expired.
func ReadSession(c *echo.Context) *Session {
	cookie, err := c.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return nil
	}

	parts := strings.SplitN(cookie.Value, ".", 2)
	if len(parts) != 2 {
		return nil
	}

	payloadRaw, sig := parts[0], parts[1]

	payloadBytes, err := base64.URLEncoding.DecodeString(payloadRaw)
	if err != nil {
		return nil
	}
	payload := string(payloadBytes)

	if !hmac.Equal([]byte(sign(payload)), []byte(sig)) {
		return nil
	}

	fields := strings.SplitN(payload, "|", 4)
	if len(fields) != 4 {
		return nil
	}

	expires, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil || time.Now().Unix() > expires {
		return nil
	}

	return &Session{Role: fields[0], CashierContact: fields[2]}
}

// GetSession retrieves the session that RequireLogin/RequireAdmin already
// parsed and stashed on the request context.
func GetSession(c *echo.Context) *Session {
	if v := c.Get(contextKey); v != nil {
		if s, ok := v.(*Session); ok {
			return s
		}
	}
	return nil
}

// IsAdminSession is a small convenience wrapper for templates/handlers that
// just need a yes/no answer to "is the current user the admin?" without
// having to nil-check GetSession themselves.
func IsAdminSession(c *echo.Context) bool {
	sess := GetSession(c)
	return sess != nil && sess.IsAdmin()
}

// ActorName returns a display name for whoever is currently logged in —
// "Admin" for the admin, or the cashier's own name. Used to stamp orders and
// receipts with who created them.
func ActorName(c *echo.Context) string {
	sess := GetSession(c)
	if sess == nil {
		return ""
	}
	if sess.IsAdmin() {
		return "Admin"
	}
	return sess.CashierName
}

// =======================
// MIDDLEWARE
// =======================

// RequireLogin ensures a valid session exists (admin or cashier) before
// allowing a request through to /pos and its sub-pages. Cashiers can view
// everything under /pos; per-action admin-only restrictions are layered on
// top with RequireAdmin.
func RequireLogin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		sess := ReadSession(c)
		if sess == nil {
			return c.Redirect(http.StatusSeeOther, "/login")
		}
		c.Set(contextKey, sess)
		return next(c)
	}
}

// RequireAdmin ensures the logged-in user is specifically the admin. Used
// for business/password settings and deleting canceled orders.
func RequireAdmin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		sess := ReadSession(c)
		if sess == nil {
			return c.Redirect(http.StatusSeeOther, "/register")
		}
		if !sess.IsAdmin() {
			return c.String(http.StatusForbidden, "Only the admin can perform this action.")
		}
		c.Set(contextKey, sess)
		return next(c)
	}
}
