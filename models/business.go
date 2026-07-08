package models

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/Yassinproweb/echo-pos/db"
)

// =======================
// PASSWORD HASHING
// =======================
//
// The project only depends on the Go standard library + go-sqlite3 (no
// network access is available to `go get` a package such as bcrypt), so
// passwords are hashed with a salted, iterated SHA-256 instead. Every
// password gets its own random salt, and the hash is stretched with 100,000
// rounds to make brute-forcing meaningfully slower than a single SHA-256
// pass. If this project ever gains network access to fetch modules, swapping
// this for golang.org/x/crypto/bcrypt is recommended.

const hashIterations = 100_000

func newSalt() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func hashPassword(password, salt string) string {
	sum := sha256.Sum256([]byte(salt + ":" + password))
	digest := sum[:]

	for i := 0; i < hashIterations; i++ {
		next := sha256.Sum256(append(digest, []byte(salt)...))
		digest = next[:]
	}

	return hex.EncodeToString(digest)
}

// HashNewPassword generates a fresh salt and returns (hash, salt).
func HashNewPassword(password string) (hash string, salt string, err error) {
	salt, err = newSalt()
	if err != nil {
		return "", "", err
	}
	return hashPassword(password, salt), salt, nil
}

// VerifyPassword checks a plaintext password against a stored hash/salt pair.
func VerifyPassword(password, hash, salt string) bool {
	if password == "" || hash == "" || salt == "" {
		return false
	}
	return hashPassword(password, salt) == hash
}

// =======================
// BUSINESS (single-tenant)
// =======================

type Business struct {
	BusinessName        string
	RestaurantName       string
	Location             string
	AdminPasswordHash    string
	AdminPasswordSalt    string
	CashierPasswordHash  string
	CashierPasswordSalt  string
}

// BusinessExists reports whether the business has already been registered.
func BusinessExists() bool {
	var count int
	err := db.DB.QueryRow(`SELECT COUNT(*) FROM business WHERE id = 1`).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// GetBusiness returns the single registered business, or nil if none exists.
func GetBusiness() (*Business, error) {
	var b Business

	err := db.DB.QueryRow(`
		SELECT
			business_name,
			restaurant_name,
			location,
			admin_password_hash,
			admin_password_salt,
			cashier_password_hash,
			cashier_password_salt
		FROM business
		WHERE id = 1
	`).Scan(
		&b.BusinessName,
		&b.RestaurantName,
		&b.Location,
		&b.AdminPasswordHash,
		&b.AdminPasswordSalt,
		&b.CashierPasswordHash,
		&b.CashierPasswordSalt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &b, nil
}

// RegisterBusiness creates the single business record. It fails if a
// business has already been registered — only one business/restaurant can
// be registered per installation.
func RegisterBusiness(businessName, restaurantName, location, adminPassword, cashierPassword string) error {
	if BusinessExists() {
		return fmt.Errorf("a business is already registered")
	}

	if businessName == "" || restaurantName == "" || location == "" {
		return fmt.Errorf("business name, restaurant name and location are required")
	}

	if len(adminPassword) < 4 {
		return fmt.Errorf("admin password must be at least 4 characters")
	}

	if len(cashierPassword) < 4 {
		return fmt.Errorf("cashier password must be at least 4 characters")
	}

	adminHash, adminSalt, err := HashNewPassword(adminPassword)
	if err != nil {
		return err
	}

	cashierHash, cashierSalt, err := HashNewPassword(cashierPassword)
	if err != nil {
		return err
	}

	_, err = db.DB.Exec(`
		INSERT INTO business (
			id,
			business_name,
			restaurant_name,
			location,
			admin_password_hash,
			admin_password_salt,
			cashier_password_hash,
			cashier_password_salt
		)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?)
	`,
		businessName,
		restaurantName,
		location,
		adminHash,
		adminSalt,
		cashierHash,
		cashierSalt,
	)

	return err
}

// UpdateBusinessDetails lets the admin update the business/restaurant name
// and location. Passwords are updated separately (see UpdateAdminPassword
// and UpdateCashierPassword) so that leaving a password field blank in the
// settings form never accidentally wipes it out.
func UpdateBusinessDetails(businessName, restaurantName, location string) error {
	if businessName == "" || restaurantName == "" || location == "" {
		return fmt.Errorf("business name, restaurant name and location are required")
	}

	_, err := db.DB.Exec(`
		UPDATE business
		SET
			business_name = ?,
			restaurant_name = ?,
			location = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, businessName, restaurantName, location)

	return err
}

// UpdateAdminPassword lets the admin change the admin password. Only the
// admin can call this (enforced at the route layer).
func UpdateAdminPassword(newPassword string) error {
	if len(newPassword) < 4 {
		return fmt.Errorf("admin password must be at least 4 characters")
	}

	hash, salt, err := HashNewPassword(newPassword)
	if err != nil {
		return err
	}

	_, err = db.DB.Exec(`
		UPDATE business
		SET admin_password_hash = ?, admin_password_salt = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, hash, salt)

	return err
}

// UpdateCashierPassword lets the admin change the single shared cashier
// password. Existing cashiers keep their registered names, but must use the
// new password on their next login. Only the admin can call this.
func UpdateCashierPassword(newPassword string) error {
	if len(newPassword) < 4 {
		return fmt.Errorf("cashier password must be at least 4 characters")
	}

	hash, salt, err := HashNewPassword(newPassword)
	if err != nil {
		return err
	}

	_, err = db.DB.Exec(`
		UPDATE business
		SET cashier_password_hash = ?, cashier_password_salt = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, hash, salt)

	return err
}

// VerifyAdminPassword checks a plaintext password against the registered
// admin password.
func VerifyAdminPassword(password string) (bool, error) {
	b, err := GetBusiness()
	if err != nil {
		return false, err
	}
	if b == nil {
		return false, fmt.Errorf("no business registered")
	}
	return VerifyPassword(password, b.AdminPasswordHash, b.AdminPasswordSalt), nil
}

// VerifyCashierPassword checks a plaintext password against the single
// shared cashier password set by the admin.
func VerifyCashierPassword(password string) (bool, error) {
	b, err := GetBusiness()
	if err != nil {
		return false, err
	}
	if b == nil {
		return false, fmt.Errorf("no business registered")
	}
	return VerifyPassword(password, b.CashierPasswordHash, b.CashierPasswordSalt), nil
}

// =======================
// CASHIERS
// =======================

type Cashier struct {
	Name string
}

// CashierExists reports whether a cashier with this name has already signed
// up (case-sensitive match on the exact name).
func CashierExists(name string) (bool, error) {
	var count int
	err := db.DB.QueryRow(`SELECT COUNT(*) FROM cashiers WHERE name = ?`, name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RegisterCashier records a brand-new cashier name the first time they sign
// up. It does not store a per-cashier password — all cashiers share the one
// cashier password set by the admin.
func RegisterCashier(name string) error {
	if name == "" {
		return fmt.Errorf("cashier name is required")
	}

	_, err := db.DB.Exec(`
		INSERT INTO cashiers (name) VALUES (?)
	`, name)

	return err
}

// FetchCashiers returns every registered cashier name.
func FetchCashiers() []Cashier {
	rows, err := db.DB.Query(`SELECT name FROM cashiers ORDER BY name ASC`)
	if err != nil {
		return []Cashier{}
	}
	defer rows.Close()

	var cashiers []Cashier
	for rows.Next() {
		var c Cashier
		if err := rows.Scan(&c.Name); err != nil {
			continue
		}
		cashiers = append(cashiers, c)
	}

	return cashiers
}
