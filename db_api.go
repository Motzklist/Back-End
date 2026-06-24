package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func InitDB() {
	connStr := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s sslmode=%s",
		getenvDefault("DB_HOST", "database"),
		getenvDefault("DB_USER", "user"),
		getenvDefault("DB_PASSWORD", "user"),
		getenvDefault("DB_NAME", "motzklist_db"),
		getenvDefault("DB_SSLMODE", "disable"),
	)
	var err error

	// Try to connect 5 times with a 2-second sleep between attempts
	for i := 0; i < 5; i++ {
		DB, err = sql.Open("postgres", connStr)
		if err == nil {
			err = DB.Ping()
			if err == nil {
				fmt.Println("Connected to Database successfully!")
				// runMigrations()
				return
			}
		}
		log.Printf("Database not ready... backing off (attempt %d/5)", i+1)
		time.Sleep(2 * time.Second)
	}

	log.Fatal("Could not connect to database after 5 attempts:", err)
}

// runMigrations applies idempotent schema tweaks at startup. The base schema is
// created from Database/init.sql only on a fresh data volume, so additive
// columns introduced later are added here to keep existing databases in sync.
// func runMigrations() {
// 	stmts := []string{
// 		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS stripe_session_id TEXT`,
// 		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS stripe_payment_intent TEXT`,
// 		// Multi-language support: Hebrew translation columns alongside the base
// 		// (English) name columns. Nullable so untranslated rows fall back to the
// 		// base name (see localizedName).
// 		`ALTER TABLE school ADD COLUMN IF NOT EXISTS sname_he TEXT`,
// 		`ALTER TABLE grade ADD COLUMN IF NOT EXISTS gname_he TEXT`,
// 		`ALTER TABLE equipment ADD COLUMN IF NOT EXISTS ename_he TEXT`,
// 	}
// 	for _, stmt := range stmts {
// 		if _, err := DB.Exec(stmt); err != nil {
// 			log.Printf("Migration failed (%s): %v", stmt, err)
// 		}
// 	}
// 	backfillHebrewSeedNames()
// }

// backfillHebrewSeedNames fills in Hebrew translations for the well-known seeded
// rows on databases that were created before the *_he columns existed (the
// seed.sql translations only run on a fresh data volume). Each statement is
// guarded so it only writes when the translation is still missing, making this
// idempotent and safe to run on every startup without clobbering admin edits.
// func backfillHebrewSeedNames() {
// 	stmts := []string{
// 		`UPDATE school SET sname_he = 'בן גוריון' WHERE sname = 'Ben Gurion' AND (sname_he IS NULL OR sname_he = '')`,
// 		`UPDATE school SET sname_he = 'אורט'      WHERE sname = 'ORT'        AND (sname_he IS NULL OR sname_he = '')`,
// 		`UPDATE school SET sname_he = 'ברנר'      WHERE sname = 'Brener'     AND (sname_he IS NULL OR sname_he = '')`,
// 		`UPDATE school SET sname_he = 'הרצל'      WHERE sname = 'Herzel'     AND (sname_he IS NULL OR sname_he = '')`,
// 		`UPDATE school SET sname_he = 'בגין'      WHERE sname = 'Begin'      AND (sname_he IS NULL OR sname_he = '')`,
// 		`UPDATE grade SET gname_he = 'כיתה ט''' WHERE gname = '9th Grade'  AND (gname_he IS NULL OR gname_he = '')`,
// 		`UPDATE grade SET gname_he = 'כיתה י''' WHERE gname = '10th Grade' AND (gname_he IS NULL OR gname_he = '')`,
// 		`UPDATE grade SET gname_he = 'כיתה י"א' WHERE gname = '11th Grade' AND (gname_he IS NULL OR gname_he = '')`,
// 		`UPDATE grade SET gname_he = 'כיתה י"ב' WHERE gname = '12th Grade' AND (gname_he IS NULL OR gname_he = '')`,
// 		`UPDATE equipment SET ename_he = 'מחברת (שורות)'           WHERE ename = 'Notebook (Ruled)'            AND (ename_he IS NULL OR ename_he = '')`,
// 		`UPDATE equipment SET ename_he = 'עיפרון'                  WHERE ename = 'Pencil'                      AND (ename_he IS NULL OR ename_he = '')`,
// 		`UPDATE equipment SET ename_he = 'ספר מתמטיקה - אלגברה א''' WHERE ename = 'Math Textbook - Algebra I'   AND (ename_he IS NULL OR ename_he = '')`,
// 		`UPDATE equipment SET ename_he = 'מחשב נייד (חובה)'         WHERE ename = 'Laptop (Required)'           AND (ename_he IS NULL OR ename_he = '')`,
// 		`UPDATE equipment SET ename_he = 'מחשבון הנדסי'            WHERE ename = 'Engineering Calculator'      AND (ename_he IS NULL OR ename_he = '')`,
// 		`UPDATE equipment SET ename_he = 'ספר פיזיקה - מתקדם'       WHERE ename = 'Physics Textbook - Advanced'  AND (ename_he IS NULL OR ename_he = '')`,
// 		`UPDATE equipment SET ename_he = 'קלסר (3 טבעות)'          WHERE ename = 'Binder (3-ring)'             AND (ename_he IS NULL OR ename_he = '')`,
// 		`UPDATE equipment SET ename_he = 'מרקרים'                 WHERE ename = 'Highlighters'                AND (ename_he IS NULL OR ename_he = '')`,
// 	}
// 	for _, stmt := range stmts {
// 		if _, err := DB.Exec(stmt); err != nil {
// 			log.Printf("Hebrew seed backfill failed (%s): %v", stmt, err)
// 		}
// 	}
// }

// parseLang reads the requested UI language from the `lang` query parameter and
// normalizes it to a language the backend can serve. Only "he" has translation
// columns today; everything else (including an absent param) means English.
func parseLang(r *http.Request) string {
	if r.URL.Query().Get("lang") == "he" {
		return "he"
	}
	return "en"
}

// localizedName builds a SQL scalar expression that selects the name column for
// the given language, falling back to the base column when the translation is
// missing/empty. base and he MUST be trusted column identifiers (never user
// input) since they are interpolated directly into the query.
func localizedName(base, he, lang string) string {
	if lang == "he" {
		return fmt.Sprintf("COALESCE(NULLIF(%s, ''), %s)", he, base)
	}
	return base
}

// --- Implementation ---

func getSchools(lang string) []School {
	log.Println("Got to getSchools function")
	query := fmt.Sprintf("SELECT sid, %s FROM school", localizedName("sname", "sname_he", lang))
	rows, err := DB.Query(query)
	if err != nil {
		log.Println("Error getting schools:", err)
		return []School{}
	}
	defer rows.Close()

	var schools []School
	for rows.Next() {
		var s School
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			log.Println(err)
			continue
		}
		schools = append(schools, s)
	}
	return schools
}

func getGrades(schoolID string, lang string) []Grade {
	log.Println("Got to getGrades function")
	sid, _ := strconv.Atoi(schoolID)
	query := fmt.Sprintf("SELECT gid, %s FROM grade WHERE sid = $1", localizedName("gname", "gname_he", lang))
	rows, err := DB.Query(query, sid)
	if err != nil {
		log.Println("Error getting grades:", err)
		return []Grade{}
	}
	defer rows.Close()

	var grades []Grade
	for rows.Next() {
		var g Grade
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			log.Println(err)
			continue
		}
		grades = append(grades, g)
	}
	return grades
}

func getEquipment(schoolID string, gradeID string, lang string) []Equipment {
	log.Println("Got to getEquipment function")
	gid, _ := strconv.Atoi(gradeID)

	query := fmt.Sprintf(`
		SELECT e.eid, %s, e.price, r.quantity
		FROM equipment e
		JOIN requirement r ON e.eid = r.eid
		WHERE r.gid = $1
	`, localizedName("e.ename", "e.ename_he", lang))
	rows, err := DB.Query(query, gid)
	if err != nil {
		log.Println("Error getting equipment:", err)
		return []Equipment{}
	}
	defer rows.Close()

	var equipmentList []Equipment
	for rows.Next() {
		var e Equipment
		if err := rows.Scan(&e.ID, &e.Name, &e.Price, &e.Quantity); err != nil {
			log.Println(err)
			continue
		}
		equipmentList = append(equipmentList, e)
	}
	return equipmentList
}

func getUserIDByCredentials(userName, password string) string {
	log.Println("Got to getUserIDByCredentials function")
	var uid string
	query := "SELECT uid FROM users WHERE uname = $1 AND password = $2"

	err := DB.QueryRow(query, userName, password).Scan(&uid)
	if err != nil {
		return ""
	}
	return uid
}

func getUsernameFromUserID(userID string) string {
	log.Println("Got to getUsernameFromUserID function")
	var uname string
	uid, _ := strconv.Atoi(userID)

	query := "SELECT uname FROM users WHERE uid = $1"
	err := DB.QueryRow(query, uid).Scan(&uname)
	if err != nil {
		return ""
	}
	return uname
}

func getCartByUserID(userID string, lang string) []CartEntry {
	log.Println("Got to getCartByUserID function")
	uid, _ := strconv.Atoi(userID)
	var cart []CartEntry
	queryEntry := fmt.Sprintf(`
		SELECT ce.ceid, g.gid, %s, s.sid, %s
		FROM cart_entry ce
		JOIN grade g ON ce.gid = g.gid
		JOIN school s ON g.sid = s.sid
		WHERE ce.uid = $1
	`, localizedName("g.gname", "g.gname_he", lang), localizedName("s.sname", "s.sname_he", lang))
	rows, err := DB.Query(queryEntry, uid)
	if err != nil {
		log.Println("Error getting cart entries:", err)
		return []CartEntry{}
	}
	defer rows.Close()

	for rows.Next() {
		var ce CartEntry
		var entryID string

		if err := rows.Scan(&entryID, &ce.Grade.ID, &ce.Grade.Name, &ce.School.ID, &ce.School.Name); err != nil {
			continue
		}
		ce.ID = entryID

		ce.Items = getCartItemsFromApply(entryID, lang)

		cart = append(cart, ce)
	}
	return cart
}

func getCartItemsFromApply(ceidStr string, lang string) []Equipment {
	log.Println("Got to getCartItemsFromApply function")
	ceid, _ := strconv.Atoi(ceidStr)

	query := fmt.Sprintf(`
		SELECT e.eid, %s, e.price, COUNT(ci.eid) as qty
		FROM cart_item ci
		JOIN equipment e ON ci.eid = e.eid
		WHERE ci.ceid = $1
		GROUP BY e.eid, e.ename, e.ename_he, e.price
	`, localizedName("e.ename", "e.ename_he", lang))
	rows, err := DB.Query(query, ceid)
	if err != nil {
		log.Println("Error reading apply table:", err)
		return []Equipment{}
	}
	defer rows.Close()

	var items []Equipment
	for rows.Next() {
		var item Equipment
		if err := rows.Scan(&item.ID, &item.Name, &item.Price, &item.Quantity); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func saveCart(userID string, cart []CartEntry) error {
	log.Println("Got to saveCart function")
	uid, _ := strconv.Atoi(userID)
	tx, err := DB.Begin()
	if err != nil {
		log.Println("Error starting transaction:", err)
		return fmt.Errorf("starting transaction: %w", err)
	}

	_, err = tx.Exec("DELETE FROM cart_entry WHERE uid = $1", uid)
	if err != nil {
		tx.Rollback()
		log.Println("Error clearing old cart:", err)
		return fmt.Errorf("clearing old cart: %w", err)
	}

	for _, entry := range cart {
		var newCeid int
		gid, _ := strconv.Atoi(entry.Grade.ID)

		err := tx.QueryRow("INSERT INTO cart_entry (gid, uid) VALUES ($1, $2) RETURNING ceid", gid, uid).Scan(&newCeid)
		if err != nil {
			tx.Rollback()
			log.Println("Error inserting cart_entry:", err)
			return fmt.Errorf("inserting cart_entry: %w", err)
		}

		for _, item := range entry.Items {
			eid, _ := strconv.Atoi(item.ID)

			for i := 0; i < item.Quantity; i++ {
				_, err := tx.Exec("INSERT INTO cart_item (ceid, eid) VALUES ($1, $2)", newCeid, eid)
				if err != nil {
					tx.Rollback()
					log.Println("Error inserting to cart_item:", err)
					return fmt.Errorf("inserting to cart_item: %w", err)
				}
			}
		}
	}

	if err = tx.Commit(); err != nil {
		log.Println("Error committing transaction:", err)
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

// NEW - adding purchase history
func getUserOrderHistory(userID string, lang string) []Order {
	log.Println("Got to getUserOrderHistory function")
	uid, _ := strconv.Atoi(userID)
	var orders []Order

	queryOrders := `
       SELECT oid, gid, purchase_date, total_amount
       FROM orders
       WHERE uid = $1
       ORDER BY purchase_date DESC;
    `
	rows, err := DB.Query(queryOrders, uid)
	if err != nil {
		log.Println("Error getting orders history:", err)
		return []Order{}
	}
	defer rows.Close()

	for rows.Next() {
		var o Order
		var t time.Time

		if err := rows.Scan(&o.ID, &o.GradeID, &t, &o.TotalAmount); err != nil {
			log.Println("Error scanning order row:", err)
			continue
		}

		o.PurchaseDate = t.Format("2006-01-02 15:04:05")

		o.Items = getOrderItems(o.ID, lang)

		orders = append(orders, o)
	}

	return orders
}

func getOrderItems(orderID string, lang string) []OrderItem {
	oid, _ := strconv.Atoi(orderID)
	var items []OrderItem

	queryItems := fmt.Sprintf(`
       SELECT
          %s,
          oi.quantity,
          oi.price_at_purchase,
          (oi.quantity * oi.price_at_purchase) as total_item_cost
       FROM order_item oi
       JOIN equipment e ON oi.eid = e.eid
       WHERE oi.oid = $1;
    `, localizedName("e.ename", "e.ename_he", lang))
	rows, err := DB.Query(queryItems, oid)
	if err != nil {
		log.Println("Error getting order items:", err)
		return []OrderItem{}
	}
	defer rows.Close()

	for rows.Next() {
		var item OrderItem

		if err := rows.Scan(&item.EquipmentName, &item.Quantity, &item.Price, &item.TotalPrice); err != nil {
			log.Println("Error scanning order item row:", err)
			continue
		}
		items = append(items, item)
	}

	return items
}

func getOrderHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("sessionid")
	if err != nil {
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID, exists := sessions[cookie.Value]
	if !exists {
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	history := getUserOrderHistory(userID, parseLang(r))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(history); err != nil {
		log.Printf("Failed to encode history response: %v", err)
	}
}
