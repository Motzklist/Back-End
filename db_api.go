package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {
	connStr := "host=database user=user password=password dbname=motzklist_db sslmode=disable"
	var err error

	// Try to connect 5 times with a 2-second sleep between attempts
	for i := 0; i < 5; i++ {
		DB, err = sql.Open("postgres", connStr)
		if err == nil {
			err = DB.Ping()
			if err == nil {
				fmt.Println("Connected to Database successfully!")
				return
			}
		}
		log.Printf("Database not ready... backing off (attempt %d/5)", i+1)
		time.Sleep(2 * time.Second)
	}

	log.Fatal("Could not connect to database after 5 attempts:", err)
}

// --- Implementation ---

func getSchools() []School {
	log.Println("Got to getSchools function")
	rows, err := DB.Query("SELECT sid, sname FROM school")
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

func getGrades(schoolID string) []Grade {
	log.Println("Got to getGrades function")
	sid, _ := strconv.Atoi(schoolID)
	rows, err := DB.Query("SELECT gid, gname FROM grade WHERE sid = $1", sid)
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

func getEquipment(schoolID string, gradeID string) []Equipment {
	log.Println("Got to getEquipment function")
	gid, _ := strconv.Atoi(gradeID)

	query := `
		SELECT e.eid, e.ename, e.price, r.quantity
		FROM equipment e
		JOIN requirement r ON e.eid = r.eid
		WHERE r.gid = $1
	`
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

func getCartByUserID(userID string) []CartEntry {
	log.Println("Got to getCartByUserID function")
	uid, _ := strconv.Atoi(userID)
	var cart []CartEntry
	queryEntry := `
		SELECT ce.ceid, g.gid, g.gname, s.sid, s.sname
		FROM cartEntry ce
		JOIN grade g ON ce.gid = g.gid
		JOIN school s ON g.sid = s.sid
		WHERE ce.uid = $1
	`
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

		ce.Items = getCartItemsFromApply(entryID)

		cart = append(cart, ce)
	}
	return cart
}

func getCartItemsFromApply(ceidStr string) []Equipment {
	log.Println("Got to getCartItemsFromApply function")
	ceid, _ := strconv.Atoi(ceidStr)

	query := `
		SELECT e.eid, e.ename, COUNT(a.eid) as qty
		FROM apply a
		JOIN equipment e ON a.eid = e.eid
		WHERE a.ceid = $1
		GROUP BY e.eid, e.ename
	`
	rows, err := DB.Query(query, ceid)
	if err != nil {
		log.Println("Error reading apply table:", err)
		return []Equipment{}
	}
	defer rows.Close()

	var items []Equipment
	for rows.Next() {
		var item Equipment
		if err := rows.Scan(&item.ID, &item.Name, &item.Quantity); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items
}

func saveCart(userID string, cart []CartEntry) {
	log.Println("Got to saveCart function")
	uid, _ := strconv.Atoi(userID)
	tx, err := DB.Begin()
	if err != nil {
		log.Println("Error starting transaction:", err)
		return
	}

	_, err = tx.Exec("DELETE FROM cartEntry WHERE uid = $1", uid)
	if err != nil {
		tx.Rollback()
		log.Println("Error clearing old cart:", err)
		return
	}

	for _, entry := range cart {
		var newCeid int
		gid, _ := strconv.Atoi(entry.Grade.ID)

		err := tx.QueryRow("INSERT INTO cartEntry (gid, uid) VALUES ($1, $2) RETURNING ceid", gid, uid).Scan(&newCeid)
		if err != nil {
			tx.Rollback()
			log.Println("Error inserting cartEntry:", err)
			return
		}

		for _, item := range entry.Items {
			eid, _ := strconv.Atoi(item.ID)

			for i := 0; i < item.Quantity; i++ {
				_, err := tx.Exec("INSERT INTO apply (ceid, eid) VALUES ($1, $2)", newCeid, eid)
				if err != nil {
					tx.Rollback()
					log.Println("Error inserting to apply:", err)
					return
				}
			}
		}
	}

	if err = tx.Commit(); err != nil {
		log.Println("Error committing transaction:", err)
	}
}
