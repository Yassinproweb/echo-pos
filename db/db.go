package db

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func ConnectDB() {
	var err error
	DB, err = sql.Open("sqlite3", "./pos.db")
	if err != nil {
		log.Fatal("Failed to connect to SQLite database:", err)
	}

	// Enable WAL mode for better concurrency
	_, err = DB.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		log.Fatal("Failed to enable WAL mode:", err)
	}

	// Create products table
	_, err = DB.Exec(`CREATE TABLE IF NOT EXISTS products (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		price REAL NOT NULL CHECK (price >= 0),
		description TEXT NOT NULL,
		image TEXT NOT NULL
	)`)
	if err != nil {
		log.Fatal("Failed to create products table:", err)
	}

	// Create tables table with capacity and state
	_, err = DB.Exec(`CREATE TABLE IF NOT EXISTS tables (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		table_name TEXT NOT NULL UNIQUE,
		capacity INTEGER NOT NULL CHECK (capacity >= 0),
		state TEXT NOT NULL CHECK (state IN ('Available', 'Occupied', 'Pending')) DEFAULT 'Available',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		CHECK (id <= 999)
	)`)
	if err != nil {
		log.Fatal("Failed to create tables table:", err)
	}

	// Create trigger for tables table to generate table_name
	_, err = DB.Exec(`CREATE TRIGGER IF NOT EXISTS generate_table_name
		AFTER INSERT ON tables
		FOR EACH ROW
		WHEN NEW.table_name IS NULL
		BEGIN
			UPDATE tables
			SET table_name = '#TBR' || printf('%03d', NEW.id)
			WHERE id = NEW.id;
		END;`)
	if err != nil {
		log.Fatal("Failed to create tables trigger:", err)
	}

	// orders table with items column and date_time as DATETIME
	_, err = DB.Exec(`CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_name TEXT NOT NULL UNIQUE,
		type TEXT NOT NULL CHECK (type IN ('Takeaway', 'Delivery', 'DineIn')),
		status TEXT NOT NULL CHECK (status IN ('Placed', 'Preparing', 'Ready', 'Canceled', 'Transit', 'Delivered', 'Taken', 'Served')) DEFAULT 'Placed',
		items INTEGER NOT NULL DEFAULT 0 CHECK (items >= 0),
		cost REAL NOT NULL CHECK (cost >= 0),
		cust_name TEXT NOT NULL,
		cust_number TEXT,
		destination TEXT,
		date_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		table_name TEXT,
		FOREIGN KEY (table_name) REFERENCES tables(table_name),
		CHECK (id <= 9999)
	)`)
	if err != nil {
		log.Fatal("Failed to create orders table:", err)
	}

	// Create trigger for orders table to generate order_name
	_, err = DB.Exec(`CREATE TRIGGER IF NOT EXISTS generate_order_name
		AFTER INSERT ON orders
		FOR EACH ROW
		WHEN NEW.order_name IS NULL
		BEGIN
			UPDATE orders
			SET order_name = '#ORD' || printf('%04d', NEW.id)
			WHERE id = NEW.id;
		END;`)
	if err != nil {
		log.Fatal("Failed to create orders trigger:", err)
	}

	// Create trigger to enforce table_name only for DineIn orders
	_, err = DB.Exec(`CREATE TRIGGER IF NOT EXISTS enforce_dinein_table
		AFTER INSERT ON orders
		FOR EACH ROW
		WHEN NEW.table_name IS NOT NULL AND NEW.type != 'DineIn'
		BEGIN
			SELECT RAISE(ABORT, 'Only DineIn orders can have a table_name');
		END;`)
	if err != nil {
		log.Fatal("Failed to create enforce_dinein_table trigger:", err)
	}

	// Create order_items table with pdt_name referencing products.name
	_, err = DB.Exec(`CREATE TABLE IF NOT EXISTS order_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_name INTEGER NOT NULL,
		pdt_name TEXT NOT NULL,
		quantity INTEGER NOT NULL CHECK (quantity >= 0),
		unit_price REAL NOT NULL CHECK (unit_price >= 0),
		FOREIGN KEY (order_name) REFERENCES orders(order_name),
		FOREIGN KEY (pdt_name) REFERENCES products(name)
	)`)
	if err != nil {
		log.Fatal("Failed to create order_items table:", err)
	}

	// Create trigger to set unit_price from products.price on insert
	_, err = DB.Exec(`CREATE TRIGGER IF NOT EXISTS set_unit_price
		AFTER INSERT ON order_items
		FOR EACH ROW
		WHEN NEW.unit_price IS NULL OR NEW.unit_price = 0
		BEGIN
			UPDATE order_items
			SET unit_price = (SELECT price FROM products WHERE name = NEW.pdt_name)
			WHERE id = NEW.id;
		END;`)
	if err != nil {
		log.Fatal("Failed to create set_unit_price trigger:", err)
	}

	// Create trigger to update orders.items after INSERT on order_items
	_, err = DB.Exec(`CREATE TRIGGER IF NOT EXISTS update_items_insert
		AFTER INSERT ON order_items
		FOR EACH ROW
		BEGIN
			UPDATE orders
			SET items = (
				SELECT SUM(quantity)
				FROM order_items
				WHERE order_id = NEW.order_id
			)
			WHERE id = NEW.order_id;
		END;`)
	if err != nil {
		log.Fatal("Failed to create update_items_insert trigger:", err)
	}

	// Create trigger to update orders.items after UPDATE on order_items.quantity
	_, err = DB.Exec(`CREATE TRIGGER IF NOT EXISTS update_items_update
		AFTER UPDATE OF quantity ON order_items
		FOR EACH ROW
		BEGIN
			UPDATE orders
			SET items = (
				SELECT SUM(quantity)
				FROM order_items
				WHERE order_id = NEW.order_id
			)
			WHERE id = NEW.order_id;
		END;`)
	if err != nil {
		log.Fatal("Failed to create update_items_update trigger:", err)
	}

	// Create trigger to update orders.items after DELETE on order_items
	_, err = DB.Exec(`CREATE TRIGGER IF NOT EXISTS update_items_delete
		AFTER DELETE ON order_items
		FOR EACH ROW
		BEGIN
			UPDATE orders
			SET items = (
				SELECT COALESCE(SUM(quantity), 0)
				FROM order_items
				WHERE order_id = OLD.order_id
			)
			WHERE id = OLD.order_id;
		END;`)
	if err != nil {
		log.Fatal("Failed to create update_items_delete trigger:", err)
	}

	// Create trigger to set orders.status = 'Served' and tables.state = 'Occupied' for DineIn orders
	_, err = DB.Exec(`CREATE TRIGGER IF NOT EXISTS set_order_and_table_state
		AFTER INSERT ON order_items
		FOR EACH ROW
		BEGIN
			UPDATE orders
			SET status = 'Served'
			WHERE id = NEW.order_id
			AND type = 'DineIn'
			AND status = 'Ready'
			AND table_id IS NOT NULL;
			UPDATE tables
			SET state = 'Occupied'
			WHERE id = (SELECT table_id FROM orders WHERE id = NEW.order_id AND type = 'DineIn' AND status = 'Served');
		END;`)
	if err != nil {
		log.Fatal("Failed to create set_order_and_table_state trigger:", err)
	}

	// Create trigger to set tables.state = 'Available' when a DineIn order is completed or canceled
	_, err = DB.Exec(`CREATE TRIGGER IF NOT EXISTS set_table_state_available
		AFTER UPDATE OF status ON orders
		FOR EACH ROW
		WHEN NEW.status IN ('Canceled', 'Served')
		BEGIN
			UPDATE tables
			SET state = 'Available'
			WHERE id = NEW.table_id
			AND NOT EXISTS (
				SELECT 1
				FROM orders o
				WHERE o.table_id = NEW.table_id
				AND o.type = 'DineIn'
				AND o.status = 'Served'
			);
		END;`)
	if err != nil {
		log.Fatal("Failed to create set_table_state_available trigger:", err)
	}

	log.Println("Connected Successfully to SQLite Database")
}
