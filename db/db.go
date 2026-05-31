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
		log.Fatal(err)
	}

	_, err = DB.Exec(`
		PRAGMA foreign_keys = ON;
		PRAGMA journal_mode = WAL;
	`)

	if err != nil {
		log.Fatal(err)
	}

	createTables()
	SeedDB()

	log.Println("SQLite Connected")
}

func createTables() {

	query := `

	-- =========================
	-- PRODUCTS
	-- =========================

	CREATE TABLE IF NOT EXISTS products (
		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT UNIQUE NOT NULL,
		description TEXT NOT NULL,

		price REAL NOT NULL CHECK(price >= 0),

		image TEXT NOT NULL
	);

	-- =========================
	-- ORDERS
	-- =========================

	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT UNIQUE,

		type TEXT NOT NULL CHECK(
			type IN (
				'Takeaway',
				'Delivery',
				'DineIn'
			)
		),

		status TEXT NOT NULL CHECK(
			status IN (
				'Placed',
				'Preparing',
				'Ready',
				'Canceled',
				'Transit',
				'Waiting',
				'PickUp',
				'Delivered',
				'Taken',
				'Served'
			)
		) DEFAULT 'Placed',

		items INTEGER DEFAULT 0,
		cost REAL DEFAULT 0,

		cust_name TEXT NOT NULL,
		cust_number TEXT NOT NULL,

		destination TEXT,

		date_time DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- =========================
	-- TABLES
	-- =========================

	CREATE TABLE IF NOT EXISTS tables (
		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT UNIQUE,

		capacity INTEGER NOT NULL,

		state TEXT NOT NULL CHECK(
			state IN (
				'Available',
				'Occupied',
				'Pending'
			)
		) DEFAULT 'Available',

		current_order_name TEXT,

		FOREIGN KEY(current_order_name)
		REFERENCES orders(name)
		ON DELETE SET NULL
	);

	-- =========================
	-- ORDER ITEMS
	-- =========================

	CREATE TABLE IF NOT EXISTS order_items (
	 id INTEGER PRIMARY KEY AUTOINCREMENT,

	 order_name TEXT NOT NULL,
	 pdt_name TEXT NOT NULL,

	 quantity INTEGER NOT NULL,
	 unit_price REAL NOT NULL DEFAULT 0,

	 UNIQUE(order_name, pdt_name),

	 FOREIGN KEY(order_name)
	 REFERENCES orders(name)
	 ON DELETE CASCADE,

	 FOREIGN KEY(pdt_name)
	 REFERENCES products(name)
	);

	CREATE TRIGGER IF NOT EXISTS generate_order_name
	AFTER INSERT ON orders
	FOR EACH ROW
	WHEN NEW.name IS NULL
	BEGIN
    UPDATE orders
    SET name = '#ORD' || printf('%04d', (
        SELECT COALESCE(MAX(CAST(SUBSTR(name, 5) AS INTEGER)), 0) + 1
        FROM orders 
        WHERE name LIKE '#ORD____'
    ))
    WHERE id = NEW.id;
	END;

	-- =========================
	-- GENERATE TABLE NAME
	-- =========================

	CREATE TRIGGER IF NOT EXISTS generate_table_name
	AFTER INSERT ON tables
	FOR EACH ROW
	WHEN NEW.name IS NULL
	BEGIN
		UPDATE tables
		SET name =
			'#TBL' || printf('%03d', NEW.id)
		WHERE id = NEW.id;
	END;

	-- =========================
	-- AUTO UNIT PRICE
	-- =========================

	CREATE TRIGGER IF NOT EXISTS set_unit_price
	AFTER INSERT ON order_items
	FOR EACH ROW
	WHEN NEW.unit_price = 0
	BEGIN
		UPDATE order_items
		SET unit_price = (
			SELECT price
			FROM products
			WHERE name = NEW.pdt_name
		)
		WHERE id = NEW.id;
	END;

	-- =========================
	-- UPDATE ITEMS COUNT
	-- =========================

	CREATE TRIGGER IF NOT EXISTS update_order_items_count
	AFTER INSERT ON order_items
	FOR EACH ROW
	BEGIN
		UPDATE orders
		SET items = (
			SELECT COALESCE(
				SUM(quantity),
				0
			)
			FROM order_items
			WHERE order_name = NEW.order_name
		)
		WHERE name = NEW.order_name;
	END;

	-- =========================
	-- UPDATE ORDER COST
	-- =========================

	CREATE TRIGGER IF NOT EXISTS update_order_cost
	AFTER INSERT ON order_items
	FOR EACH ROW
	BEGIN

	 UPDATE orders
	 SET cost = (
	  SELECT COALESCE(
	   SUM(quantity * unit_price),
	   0
	  )
	  FROM order_items
	  WHERE order_name = NEW.order_name
	 )
	 WHERE name = NEW.order_name;

	END;

	CREATE TRIGGER IF NOT EXISTS update_order_cost_on_update
	AFTER UPDATE ON order_items
	FOR EACH ROW
	BEGIN

	 UPDATE orders
	 SET cost = (
	  SELECT COALESCE(
	   SUM(quantity * unit_price),
	   0
	  )
	  FROM order_items
	  WHERE order_name = NEW.order_name
	 )
	 WHERE name = NEW.order_name;

	END;

	CREATE TRIGGER IF NOT EXISTS update_order_cost_on_delete
	AFTER DELETE ON order_items
	FOR EACH ROW
	BEGIN

	 UPDATE orders
	 SET cost = (
	  SELECT COALESCE(
	   SUM(quantity * unit_price),
	   0
	  )
	  FROM order_items
	  WHERE order_name = OLD.order_name
	 )
	 WHERE name = OLD.order_name;

	END;

	-- =========================
	-- PREVENT DINEIN WITHOUT TABLES
	-- =========================

	CREATE TRIGGER IF NOT EXISTS prevent_dinein_without_tables
	BEFORE INSERT ON orders
	FOR EACH ROW
	WHEN NEW.type = 'DineIn'
		AND (
			SELECT COUNT(*)
			FROM tables
			WHERE state = 'Available'
		) = 0
	BEGIN
		SELECT RAISE(
			ABORT,
			'No tables available'
		);
	END;

	-- =========================
	-- AUTO ASSIGN TABLE
	-- =========================

	CREATE TRIGGER IF NOT EXISTS auto_assign_table_to_dinein
	AFTER INSERT ON orders
	FOR EACH ROW
	WHEN NEW.type = 'DineIn'
	BEGIN

		UPDATE orders
		SET destination = (
			SELECT name
			FROM tables
			WHERE state = 'Available'
			LIMIT 1
		)
		WHERE id = NEW.id;

		UPDATE tables
		SET
			current_order_name = NEW.name,
			state = 'Occupied'
		WHERE name = (
			SELECT destination
			FROM orders
			WHERE id = NEW.id
		);

	END;

	-- =========================
	-- SYNC TABLE STATE
	-- =========================

	CREATE TRIGGER IF NOT EXISTS sync_table_state_update
	AFTER UPDATE OF status ON orders
	FOR EACH ROW
	WHEN NEW.type = 'DineIn'
	BEGIN

		UPDATE tables
		SET
			current_order_name = CASE
				WHEN NEW.status IN ('Canceled', 'Served')
				THEN NULL
				ELSE NEW.name
			END,

			state = CASE
				WHEN NEW.status = 'Canceled'
				THEN 'Available'

				WHEN NEW.status = 'Served'
				THEN 'Pending'

				ELSE 'Occupied'
			END

		WHERE name = NEW.destination;

	END;
	`

	_, err := DB.Exec(query)

	if err != nil {
		log.Fatal(err)
	}
}

func SeedDB() {
	// =========================
	// PRODUCTS
	// =========================

	products := []struct {
		name        string
		description string
		price       int
		image       string
	}{
		{
			"Delicious Burger",
			"Prepared from the best wheat and vegetable oil plus eggs from locally bred poultry.",
			10000,
			"/assets/imgs/ff-burger.jpeg",
		},
		{
			"Roasted Chapati",
			"Prepared from the best wheat and vegetable oil plus eggs from locally bred poultry.",
			1000,
			"/assets/imgs/ff-chapati.jpg",
		},
		{
			"Vegetable Pizza",
			"Prepared from the best wheat and vegetable oil plus eggs from locally bred poultry.",
			32000,
			"/assets/imgs/ff-pizza.jpeg",
		},
		{
			"Spicy Rolex",
			"Prepared from the best wheat and vegetable oil plus eggs from locally bred poultry.",
			7000,
			"/assets/imgs/ff-rolex.jpeg",
		},
		{
			"Scrumbled Eggs",
			"Prepared from the best wheat and vegetable oil plus eggs from locally bred poultry.",
			5000,
			"/assets/imgs/ff-scrumbled_eggs.jpeg",
		},
		{
			"Tropical Fruitsalad",
			"A mix of most tropical fruits, citrus fruits, wild berries and grapes.",
			15000,
			"/assets/imgs/fr-fruit_salad.jpg",
		},
		{
			"Vegetable Salad",
			"A mix of most tropical vegetables cultivated here in Uganda fresh and clean for you.",
			11000,
			"/assets/imgs/fr-salads.jpg",
		},
		{
			"Apple Juice",
			"Perfectly blended from organic apples locally grown in Uganda with zero sugar added.",
			9000,
			"/assets/imgs/juice-apple.jpg",
		},
		{
			"Coconut Juice",
			"Perfectly blended from organic coconuts locally grown in Uganda with zero sugar added.",
			7000,
			"/assets/imgs/juice-coconut.jpg",
		},
		{
			"Mango Juice",
			"Perfectly blended from organic mangoes locally grown in Uganda with zero sugar added.",
			5000,
			"/assets/imgs/juice-mango.jpg",
		},
		{
			"Pineapple Juice",
			"Perfectly blended from organic pineapples locally grown in Uganda with zero sugar added.",
			3000,
			"/assets/imgs/juice-pineapple.jpg",
		},
		{
			"Strawberry Juice",
			"Perfectly blended from organic strawberries locally grown in Uganda with zero sugar added.",
			15000,
			"/assets/imgs/juice-strawberry.jpg",
		},
		{
			"Luwombo Beef",
			"Steamed beef with a delicious taste, wrapped in banana leaves to keep the natural aroma.",
			30000,
			"/assets/imgs/lw-beef.jpeg",
		},
		{
			"Luwombo Chicken",
			"Steamed chicken with a delicious taste, wrapped in banana leaves to keep the natural aroma.",
			35000,
			"/assets/imgs/lw-chicken.jpeg",
		},
		{
			"Luwombo Fish",
			"Steamed fish with a delicious taste, wrapped in banana leaves to keep the natural aroma.",
			32000,
			"/assets/imgs/lw-fish.jpg",
		},
		{
			"Luwombo Binyebwa",
			"Steamed g-nuts with a delicious taste, wrapped in banana leaves to keep the natural aroma.",
			13000,
			"/assets/imgs/lw-gnuts.jpeg",
		},
		{
			"Luwombo Goat",
			"Steamed goat's meat with a delicious taste, wrapped in banana leaves to keep the natural aroma.",
			35000,
			"/assets/imgs/lw-mbuzi.jpeg",
		},
		{
			"Akatogo Ka Muwogo",
			"An ancient meal during the times of famine a blend of cassava, beans, and green vegetables.",
			5000,
			"/assets/imgs/st-cassava_katogo.jpeg",
		},
		{
			"Deep-fried Chicken",
			"The taste of Ugandan chicken dipped in hot oil for a crispy taste loved by the locals.",
			8000,
			"/assets/imgs/st-chicken.jpg",
		},
		{
			"Deep-fried Fish",
			"A pure taste of locally farmed Ugandan tilapia deeped in organic cooking oil.",
			48000,
			"/assets/imgs/st-fish.jpeg",
		},
		{
			"Breakfast Katogo",
			"An ancient breakfast meal for wedding which includes matooke, meat, avocado, and green vegetables.",
			7000,
			"/assets/imgs/st-katogo.jpeg",
		},
		{
			"Ettooke Eriboobedde",
			"Steamed matooke/plantain wrapped in banana leaves to keep the natural aroma.",
			5000,
			"/assets/imgs/st-matooke.jpeg",
		},
		{
			"Pilau & Goat",
			"Yummy brown rice with goat's meat. Not Biriyani, it's prepared in a local way.",
			25000,
			"/assets/imgs/st-pilau.jpeg",
		},
		{
			"Boiled White Rice",
			"Boiled white rice with the best vegetable seasoning for a soothing natural aroma.",
			10000,
			"/assets/imgs/st-rice_boil.jpeg",
		},
	}

	for _, p := range products {

		_, err := DB.Exec(`
			INSERT OR IGNORE INTO products(
				name,
				description,
				price,
				image
			)
			VALUES(?, ?, ?, ?)
		`,
			p.name,
			p.description,
			p.price,
			p.image,
		)

		if err != nil {
			log.Println(err)
		}
	}

	// =========================
	// TABLES
	// =========================

	tables := []struct {
		name     string
		capacity int
		state    string
	}{
		{"#TBL001", 6, "Available"},
		{"#TBL002", 6, "Available"},
		{"#TBL003", 2, "Available"},
		{"#TBL004", 2, "Available"},
		{"#TBL005", 2, "Available"},
		{"#TBL006", 6, "Available"},
		{"#TBL007", 4, "Available"},
		{"#TBL008", 4, "Available"},
		{"#TBL009", 4, "Available"},
		{"#TBL010", 4, "Available"},
		{"#TBL011", 4, "Available"},
		{"#TBL012", 4, "Available"},
		{"#TBL013", 8, "Available"},
		{"#TBL014", 4, "Available"},
		{"#TBL015", 8, "Available"},
		{"#TBL016", 4, "Available"},
		{"#TBL017", 4, "Pending"},
		{"#TBL018", 2, "Pending"},
		{"#TBL019", 8, "Pending"},
		{"#TBL020", 4, "Pending"},
		{"#TBL021", 6, "Pending"},
	}

	for _, t := range tables {

		_, err := DB.Exec(`
			INSERT OR IGNORE INTO tables(
				name,
				capacity,
				state
			)
			VALUES(?, ?, ?)
		`,
			t.name,
			t.capacity,
			t.state,
		)

		if err != nil {
			log.Println(err)
		}
	}

	// =========================
	// ORDERS
	// =========================

	orders := []struct {
		name        string
		orderType   string
		status      string
		custName    string
		custNumber  string
		destination string
	}{
		{"#ORD0001", "DineIn", "Placed", "Yūsuf", "0704126781", ""},
		{"#ORD0002", "DineIn", "Preparing", "Amīnah", "0782459013", ""},
		{"#ORD0003", "DineIn", "Waiting", "Hamzah", "0756312489", ""},
		{"#ORD0004", "Delivery", "Ready", "Fātimah", "0779023146", "Kira, Wakiso"},
		{"#ORD0005", "Takeaway", "PickUp", "Abdullah", "0718452390", ""},
		{"#ORD0006", "Delivery", "Transit", "Khadījah", "0707784512", "Seeta, Mukono"},
		{"#ORD0007", "Takeaway", "Taken", "Ibrahim", "0749921638", ""},
		{"#ORD0008", "DineIn", "Served", "Zainab", "0783345210", ""},
		{"#ORD0009", "Delivery", "Canceled", "Mustafa", "0751186492", "Ntinda, Kampala"},
		{"#ORD0010", "DineIn", "Ready", "Yassin", "0748592974", ""},
		{"#ORD0011", "DineIn", "Served", "Zubayr", "0700092974", ""},
		{"#ORD0012", "DineIn", "Served", "Muhsin", "0748111974", ""},
		{"#ORD0013", "DineIn", "Served", "Hytham", "0748593333", ""},
		{"#ORD0014", "DineIn", "Ready", "Yāsir", "0748111114", ""},
		{"#ORD0015", "DineIn", "Served", "Najim", "0740666974", ""},
		{"#ORD0016", "DineIn", "Placed", "Ruways", "0748882974", ""},
		{"#ORD0017", "DineIn", "Placed", "Khadīj", "0748999974", ""},
		{"#ORD0018", "DineIn", "Preparing", "Muhammad", "0788892904", ""},
		{"#ORD0019", "DineIn", "Placed", "Maryam", "0748590100", ""},
		{"#ORD0020", "DineIn", "Served", "Akram", "0748592114", ""},
	}

	for _, o := range orders {

		_, err := DB.Exec(`
			INSERT OR IGNORE INTO orders(
				name,
				type,
				status,
				cost,
				cust_name,
				cust_number,
				destination
			)
			VALUES(?, ?, ?, ?, ?, ?, ?)
		`,
			o.name,
			o.orderType,
			o.status,
			0,
			o.custName,
			o.custNumber,
			o.destination,
		)

		if err != nil {
			log.Println(err)
		}
	}

	// =========================
	// ORDER ITEMS
	// =========================

	items := []struct {
		orderName string
		pdtName   string
		quantity  int
	}{
		{"#ORD0001", "Tropical Fruitsalad", 6},
		{"#ORD0001", "Pilau & Goat", 4},
		{"#ORD0001", "Ettooke Eriboobedde", 2},
		{"#ORD0001", "Luwombo Chicken", 2},
		{"#ORD0001", "Pineapple Juice", 6},

		{"#ORD0002", "Luwombo Chicken", 2},
		{"#ORD0002", "Pilau & Goat", 4},

		{"#ORD0003", "Tropical Fruitsalad", 1},
		{"#ORD0003", "Pineapple Juice", 2},

		{"#ORD0004", "Pilau & Goat", 1},
		{"#ORD0004", "Pineapple Juice", 2},

		{"#ORD0005", "Pilau & Goat", 3},

		{"#ORD0006", "Tropical Fruitsalad", 3},

		{"#ORD0007", "Spicy Rolex", 3},

		{"#ORD0008", "Pilau & Goat", 4},

		{"#ORD0009", "Luwombo Chicken", 1},
		{"#ORD0009", "Pineapple Juice", 1},
		{"#ORD0009", "Ettooke Eriboobedde", 1},

		{"#ORD0010", "Vegetable Salad", 1},
		{"#ORD0010", "Roasted Chapati", 3},
		{"#ORD0010", "Luwombo Binyebwa", 1},
		{"#ORD0010", "Strawberry Juice", 1},

		{"#ORD0011", "Akatogo Ka Muwogo", 4},
		{"#ORD0011", "Coconut Juice", 1},
		{"#ORD0011", "Apple Juice", 1},
		{"#ORD0011", "Mango Juice", 1},

		{"#ORD0012", "Breakfast Katogo", 1},
		{"#ORD0012", "Roasted Chapati", 1},
		{"#ORD0012", "Strawberry Juice", 1},

		{"#ORD0013", "Roasted Chapati", 10},
		{"#ORD0013", "Luwombo Goat", 5},
		{"#ORD0013", "Mango Juice", 5},

		{"#ORD0014", "Mango Juice", 1},
		{"#ORD0014", "Apple Juice", 1},
		{"#ORD0014", "Tropical Fruitsalad", 1},
		{"#ORD0014", "Coconut Juice", 1},
		{"#ORD0014", "Pineapple Juice", 1},

		{"#ORD0015", "Tropical Fruitsalad", 1},
		{"#ORD0015", "Boiled White Rice", 1},
		{"#ORD0015", "Luwombo Beef", 1},
		{"#ORD0015", "Strawberry Juice", 1},

		{"#ORD0016", "Boiled White Rice", 1},
		{"#ORD0016", "Luwombo Binyebwa", 1},
		{"#ORD0016", "Apple Juice", 1},

		{"#ORD0017", "Boiled White Rice", 1},
		{"#ORD0017", "Ettooke Eriboobedde", 1},
		{"#ORD0017", "Luwombo Chicken", 1},
		{"#ORD0017", "Strawberry Juice", 1},

		{"#ORD0018", "Vegetable Salad", 1},
		{"#ORD0018", "Ettooke Eriboobedde", 1},
		{"#ORD0018", "Luwombo Binyebwa", 1},

		{"#ORD0019", "Roasted Chapati", 1},
		{"#ORD0019", "Luwombo Binyebwa", 1},
		{"#ORD0019", "Mango Juice", 1},

		{"#ORD0020", "Vegetable Salad", 1},
		{"#ORD0020", "Pilau & Goat", 1},
		{"#ORD0020", "Coconut Juice", 1},
		{"#ORD0020", "Pineapple Juice", 1},
	}

	for _, i := range items {

		_, err := DB.Exec(`
			INSERT OR IGNORE INTO order_items(
				order_name,
				pdt_name,
				quantity
			)
			VALUES(?, ?, ?)
		`,
			i.orderName,
			i.pdtName,
			i.quantity,
		)

		if err != nil {
			log.Println(err)
		}
	}
}
