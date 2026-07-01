package main

// Admin API
//
// Implements the `/api/admin/*` endpoints the Admin Front-End consumes (via its
// Next.js `/api/*` proxy). All handlers require a valid `sessionid` session and
// return plain JSON entities using the camelCase field names the admin client
// expects (see the admin front-end `types/api.ts`). Reads/writes go straight to
// the Postgres schema defined in Database/init.sql.

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Response DTOs (camelCase to match the admin front-end contract)
// ---------------------------------------------------------------------------

type AdminGrade struct {
	ID       string `json:"id"`
	SchoolID string `json:"schoolId"`
	Name     string `json:"name"`
	NameHe   string `json:"nameHe,omitempty"`
}

type CatalogItem struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	NameHe string  `json:"nameHe,omitempty"`
	Price  float64 `json:"price"`
}

type RequirementItem struct {
	EquipmentID string  `json:"equipmentId"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
}

type GradeRequirements struct {
	GradeID  string            `json:"gradeId"`
	SchoolID string            `json:"schoolId"`
	Items    []RequirementItem `json:"items"`
}

type ParentUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type AdminOrderItem struct {
	EquipmentID     string  `json:"equipmentId"`
	EquipmentName   string  `json:"equipmentName"`
	Quantity        int     `json:"quantity"`
	PriceAtPurchase float64 `json:"priceAtPurchase"`
}

type AdminOrder struct {
	ID           string           `json:"id"`
	UserID       string           `json:"userId"`
	Username     string           `json:"username,omitempty"`
	SchoolID     string           `json:"schoolId,omitempty"`
	SchoolName   string           `json:"schoolName,omitempty"`
	GradeID      string           `json:"gradeId"`
	GradeName    string           `json:"gradeName,omitempty"`
	PurchaseDate string           `json:"purchaseDate"`
	TotalAmount  float64          `json:"totalAmount"`
	Items        []AdminOrderItem `json:"items"`
}

type MonthRevenue struct {
	Month   string  `json:"month"`
	Revenue float64 `json:"revenue"`
}

type TopEquipment struct {
	EquipmentID string  `json:"equipmentId"`
	Name        string  `json:"name"`
	Quantity    int     `json:"quantity"`
	Revenue     float64 `json:"revenue"`
}

type SchoolSpend struct {
	SchoolID   string  `json:"schoolId"`
	SchoolName string  `json:"schoolName"`
	Revenue    float64 `json:"revenue"`
}

type AnalyticsSummary struct {
	TotalRevenue   float64        `json:"totalRevenue"`
	TotalOrders    int            `json:"totalOrders"`
	ActiveCarts    int            `json:"activeCarts"`
	CatalogSize    int            `json:"catalogSize"`
	RevenueByMonth []MonthRevenue `json:"revenueByMonth"`
	TopEquipment   []TopEquipment `json:"topEquipment"`
	SpendBySchool  []SchoolSpend  `json:"spendBySchool"`
}

type ImportRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type ImportResult struct {
	Created int              `json:"created"`
	Updated int              `json:"updated"`
	Skipped int              `json:"skipped"`
	Errors  []ImportRowError `json:"errors"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// emptyToNull maps an empty string to a SQL NULL so optional translation
// columns stay NULL (and thus fall back to the base name via localizedName)
// rather than storing an empty string.
func emptyToNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// writeJSON encodes v as the response body with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// requireSession returns the authenticated user id, or writes a 401 and false.
func requireSession(w http.ResponseWriter, r *http.Request) (string, bool) {
	cookie, err := r.Cookie("sessionid")
	if err != nil {
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return "", false
	}
	userID, ok := sessions[cookie.Value]
	if !ok {
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return "", false
	}
	return userID, true
}

// pathID parses the `{id}` path value as an integer, or writes a 400 and false.
func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		JSONError(w, "Invalid id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// ---------------------------------------------------------------------------
// Schools
// ---------------------------------------------------------------------------

// listSchoolsHandler returns all schools with both the base and Hebrew names so
// the admin panel can display/manage every language at once (the public
// /api/schools endpoint only returns a single localized name).
func listSchoolsHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	rows, err := DB.Query("SELECT sid, sname, COALESCE(sname_he, '') FROM school ORDER BY sid")
	if err != nil {
		log.Printf("listSchools: %v", err)
		JSONError(w, "Failed to load schools", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	schools := []School{}
	for rows.Next() {
		var s School
		if err := rows.Scan(&s.ID, &s.Name, &s.NameHe); err != nil {
			log.Printf("listSchools(scan): %v", err)
			continue
		}
		schools = append(schools, s)
	}
	writeJSON(w, http.StatusOK, schools)
}

func createSchoolHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	var body struct {
		Name   string `json:"name"`
		NameHe string `json:"nameHe"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		JSONError(w, "School name is required", http.StatusBadRequest)
		return
	}
	nameHe := strings.TrimSpace(body.NameHe)
	var s School
	err := DB.QueryRow(
		"INSERT INTO school (sname, sname_he) VALUES ($1, $2) RETURNING sid, sname, COALESCE(sname_he, '')",
		name, emptyToNull(nameHe),
	).Scan(&s.ID, &s.Name, &s.NameHe)
	if err != nil {
		log.Printf("createSchool: %v", err)
		JSONError(w, "Failed to create school", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func getSchoolHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var s School
	err := DB.QueryRow("SELECT sid, sname, COALESCE(sname_he, '') FROM school WHERE sid = $1", id).Scan(&s.ID, &s.Name, &s.NameHe)
	if err == sql.ErrNoRows {
		JSONError(w, "School not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("getSchool: %v", err)
		JSONError(w, "Failed to load school", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func updateSchoolHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name   string  `json:"name"`
		NameHe *string `json:"nameHe"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		JSONError(w, "School name is required", http.StatusBadRequest)
		return
	}
	var s School
	var err error
	// Only touch sname_he when the caller actually sent a nameHe field, so
	// existing translations are preserved on name-only updates.
	if body.NameHe != nil {
		err = DB.QueryRow(
			"UPDATE school SET sname = $1, sname_he = $2 WHERE sid = $3 RETURNING sid, sname, COALESCE(sname_he, '')",
			name, emptyToNull(strings.TrimSpace(*body.NameHe)), id,
		).Scan(&s.ID, &s.Name, &s.NameHe)
	} else {
		err = DB.QueryRow(
			"UPDATE school SET sname = $1 WHERE sid = $2 RETURNING sid, sname, COALESCE(sname_he, '')",
			name, id,
		).Scan(&s.ID, &s.Name, &s.NameHe)
	}
	if err == sql.ErrNoRows {
		JSONError(w, "School not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("updateSchool: %v", err)
		JSONError(w, "Failed to update school", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func deleteSchoolHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	res, err := DB.Exec("DELETE FROM school WHERE sid = $1", id)
	if err != nil {
		log.Printf("deleteSchool: %v", err)
		JSONError(w, "Failed to delete school", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		JSONError(w, "School not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Grades
// ---------------------------------------------------------------------------

// listGradesHandler returns a school's grades with both the base and Hebrew
// names (the public /api/grades endpoint only returns a single localized name).
func listGradesHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	schoolID := r.URL.Query().Get("school_id")
	if schoolID == "" {
		JSONError(w, "Missing required query parameter: school_id", http.StatusBadRequest)
		return
	}
	if _, err := strconv.Atoi(schoolID); err != nil {
		JSONError(w, "school_id must be an integer", http.StatusBadRequest)
		return
	}
	rows, err := DB.Query("SELECT gid, sid, gname, COALESCE(gname_he, '') FROM grade WHERE sid = $1 ORDER BY gid", schoolID)
	if err != nil {
		log.Printf("listGrades: %v", err)
		JSONError(w, "Failed to load grades", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	grades := []AdminGrade{}
	for rows.Next() {
		var g AdminGrade
		if err := rows.Scan(&g.ID, &g.SchoolID, &g.Name, &g.NameHe); err != nil {
			log.Printf("listGrades(scan): %v", err)
			continue
		}
		grades = append(grades, g)
	}
	writeJSON(w, http.StatusOK, grades)
}

func createGradeHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	var body struct {
		SchoolID string `json:"schoolId"`
		Name     string `json:"name"`
		NameHe   string `json:"nameHe"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	schoolID, err := strconv.Atoi(strings.TrimSpace(body.SchoolID))
	if err != nil {
		JSONError(w, "A valid schoolId is required", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		JSONError(w, "Grade name is required", http.StatusBadRequest)
		return
	}
	var exists bool
	if err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM school WHERE sid = $1)", schoolID).Scan(&exists); err != nil {
		log.Printf("createGrade(check school): %v", err)
		JSONError(w, "Failed to create grade", http.StatusInternalServerError)
		return
	}
	if !exists {
		JSONError(w, "School not found", http.StatusNotFound)
		return
	}
	var g AdminGrade
	err = DB.QueryRow(
		"INSERT INTO grade (sid, gname, gname_he) VALUES ($1, $2, $3) RETURNING gid, sid, gname, COALESCE(gname_he, '')",
		schoolID, name, emptyToNull(strings.TrimSpace(body.NameHe)),
	).Scan(&g.ID, &g.SchoolID, &g.Name, &g.NameHe)
	if err != nil {
		log.Printf("createGrade: %v", err)
		JSONError(w, "Failed to create grade", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func getGradeHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var g AdminGrade
	err := DB.QueryRow("SELECT gid, sid, gname, COALESCE(gname_he, '') FROM grade WHERE gid = $1", id).Scan(&g.ID, &g.SchoolID, &g.Name, &g.NameHe)
	if err == sql.ErrNoRows {
		JSONError(w, "Grade not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("getGrade: %v", err)
		JSONError(w, "Failed to load grade", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func updateGradeHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name   string  `json:"name"`
		NameHe *string `json:"nameHe"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		JSONError(w, "Grade name is required", http.StatusBadRequest)
		return
	}
	var g AdminGrade
	var err error
	// Only touch gname_he when the caller sent a nameHe field, preserving any
	// existing translation on name-only updates.
	if body.NameHe != nil {
		err = DB.QueryRow(
			"UPDATE grade SET gname = $1, gname_he = $2 WHERE gid = $3 RETURNING gid, sid, gname, COALESCE(gname_he, '')",
			name, emptyToNull(strings.TrimSpace(*body.NameHe)), id,
		).Scan(&g.ID, &g.SchoolID, &g.Name, &g.NameHe)
	} else {
		err = DB.QueryRow(
			"UPDATE grade SET gname = $1 WHERE gid = $2 RETURNING gid, sid, gname, COALESCE(gname_he, '')",
			name, id,
		).Scan(&g.ID, &g.SchoolID, &g.Name, &g.NameHe)
	}
	if err == sql.ErrNoRows {
		JSONError(w, "Grade not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("updateGrade: %v", err)
		JSONError(w, "Failed to update grade", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func deleteGradeHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	res, err := DB.Exec("DELETE FROM grade WHERE gid = $1", id)
	if err != nil {
		log.Printf("deleteGrade: %v", err)
		JSONError(w, "Failed to delete grade", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		JSONError(w, "Grade not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Grade requirements (a grade's equipment list)
// ---------------------------------------------------------------------------

// loadRequirements builds the GradeRequirements payload for a grade, or returns
// (_, false) if the grade does not exist.
func loadRequirements(gradeID int) (GradeRequirements, bool, error) {
	var schoolID string
	err := DB.QueryRow("SELECT sid FROM grade WHERE gid = $1", gradeID).Scan(&schoolID)
	if err == sql.ErrNoRows {
		return GradeRequirements{}, false, nil
	}
	if err != nil {
		return GradeRequirements{}, false, err
	}

	rows, err := DB.Query(`
		SELECT e.eid, e.ename, e.price, r.quantity
		FROM requirement r
		JOIN equipment e ON r.eid = e.eid
		WHERE r.gid = $1
		ORDER BY e.eid
	`, gradeID)
	if err != nil {
		return GradeRequirements{}, false, err
	}
	defer rows.Close()

	items := []RequirementItem{}
	for rows.Next() {
		var it RequirementItem
		if err := rows.Scan(&it.EquipmentID, &it.Name, &it.Price, &it.Quantity); err != nil {
			return GradeRequirements{}, false, err
		}
		items = append(items, it)
	}
	return GradeRequirements{GradeID: strconv.Itoa(gradeID), SchoolID: schoolID, Items: items}, true, nil
}

func getRequirementsHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	reqs, found, err := loadRequirements(id)
	if err != nil {
		log.Printf("getRequirements: %v", err)
		JSONError(w, "Failed to load requirements", http.StatusInternalServerError)
		return
	}
	if !found {
		JSONError(w, "Grade not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, reqs)
}

func putRequirementsHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Items []struct {
			EquipmentID string `json:"equipmentId"`
			Quantity    int    `json:"quantity"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var gradeExists bool
	if err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM grade WHERE gid = $1)", id).Scan(&gradeExists); err != nil {
		log.Printf("putRequirements(check grade): %v", err)
		JSONError(w, "Failed to update requirements", http.StatusInternalServerError)
		return
	}
	if !gradeExists {
		JSONError(w, "Grade not found", http.StatusNotFound)
		return
	}

	// Collapse duplicate equipment ids (last one wins), validating as we go.
	byEquipment := map[int]int{}
	order := []int{}
	for _, item := range body.Items {
		eid, err := strconv.Atoi(strings.TrimSpace(item.EquipmentID))
		if err != nil {
			JSONError(w, "Invalid equipmentId", http.StatusBadRequest)
			return
		}
		if item.Quantity <= 0 {
			JSONError(w, "Quantity must be greater than zero", http.StatusBadRequest)
			return
		}
		var exists bool
		if err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM equipment WHERE eid = $1)", eid).Scan(&exists); err != nil {
			log.Printf("putRequirements(check equipment): %v", err)
			JSONError(w, "Failed to update requirements", http.StatusInternalServerError)
			return
		}
		if !exists {
			JSONError(w, "Unknown equipment "+item.EquipmentID, http.StatusBadRequest)
			return
		}
		if _, seen := byEquipment[eid]; !seen {
			order = append(order, eid)
		}
		byEquipment[eid] = item.Quantity
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Printf("putRequirements(begin): %v", err)
		JSONError(w, "Failed to update requirements", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec("DELETE FROM requirement WHERE gid = $1", id); err != nil {
		tx.Rollback()
		log.Printf("putRequirements(delete): %v", err)
		JSONError(w, "Failed to update requirements", http.StatusInternalServerError)
		return
	}
	for _, eid := range order {
		if _, err := tx.Exec("INSERT INTO requirement (gid, eid, quantity) VALUES ($1, $2, $3)", id, eid, byEquipment[eid]); err != nil {
			tx.Rollback()
			log.Printf("putRequirements(insert): %v", err)
			JSONError(w, "Failed to update requirements", http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("putRequirements(commit): %v", err)
		JSONError(w, "Failed to update requirements", http.StatusInternalServerError)
		return
	}

	reqs, _, err := loadRequirements(id)
	if err != nil {
		log.Printf("putRequirements(reload): %v", err)
		JSONError(w, "Failed to load requirements", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, reqs)
}

// ---------------------------------------------------------------------------
// Equipment catalog
// ---------------------------------------------------------------------------

func listCatalogHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	rows, err := DB.Query("SELECT eid, ename, COALESCE(ename_he, ''), price FROM equipment ORDER BY eid")
	if err != nil {
		log.Printf("listCatalog: %v", err)
		JSONError(w, "Failed to load equipment", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []CatalogItem{}
	for rows.Next() {
		var e CatalogItem
		if err := rows.Scan(&e.ID, &e.Name, &e.NameHe, &e.Price); err != nil {
			log.Printf("listCatalog(scan): %v", err)
			continue
		}
		items = append(items, e)
	}
	writeJSON(w, http.StatusOK, items)
}

func createCatalogHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	var body struct {
		Name   string  `json:"name"`
		NameHe string  `json:"nameHe"`
		Price  float64 `json:"price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		JSONError(w, "Equipment name is required", http.StatusBadRequest)
		return
	}
	if body.Price < 0 {
		JSONError(w, "Price must be zero or greater", http.StatusBadRequest)
		return
	}
	var e CatalogItem
	err := DB.QueryRow(
		"INSERT INTO equipment (ename, ename_he, price) VALUES ($1, $2, $3) RETURNING eid, ename, COALESCE(ename_he, ''), price",
		name, emptyToNull(strings.TrimSpace(body.NameHe)), body.Price,
	).Scan(&e.ID, &e.Name, &e.NameHe, &e.Price)
	if err != nil {
		log.Printf("createCatalog: %v", err)
		JSONError(w, "Failed to create equipment", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func getCatalogHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var e CatalogItem
	err := DB.QueryRow("SELECT eid, ename, COALESCE(ename_he, ''), price FROM equipment WHERE eid = $1", id).Scan(&e.ID, &e.Name, &e.NameHe, &e.Price)
	if err == sql.ErrNoRows {
		JSONError(w, "Equipment not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("getCatalog: %v", err)
		JSONError(w, "Failed to load equipment", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func updateCatalogHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name   *string  `json:"name"`
		NameHe *string  `json:"nameHe"`
		Price  *float64 `json:"price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Load current values, then apply whichever fields were provided.
	var e CatalogItem
	err := DB.QueryRow("SELECT eid, ename, COALESCE(ename_he, ''), price FROM equipment WHERE eid = $1", id).Scan(&e.ID, &e.Name, &e.NameHe, &e.Price)
	if err == sql.ErrNoRows {
		JSONError(w, "Equipment not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("updateCatalog(load): %v", err)
		JSONError(w, "Failed to update equipment", http.StatusInternalServerError)
		return
	}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			JSONError(w, "Equipment name is required", http.StatusBadRequest)
			return
		}
		e.Name = name
	}
	if body.NameHe != nil {
		e.NameHe = strings.TrimSpace(*body.NameHe)
	}
	if body.Price != nil {
		if *body.Price < 0 {
			JSONError(w, "Price must be zero or greater", http.StatusBadRequest)
			return
		}
		e.Price = *body.Price
	}
	if _, err := DB.Exec("UPDATE equipment SET ename = $1, ename_he = $2, price = $3 WHERE eid = $4", e.Name, emptyToNull(e.NameHe), e.Price, id); err != nil {
		log.Printf("updateCatalog: %v", err)
		JSONError(w, "Failed to update equipment", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func deleteCatalogHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	res, err := DB.Exec("DELETE FROM equipment WHERE eid = $1", id)
	if err != nil {
		log.Printf("deleteCatalog: %v", err)
		JSONError(w, "Failed to delete equipment", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		JSONError(w, "Equipment not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Parent users
// ---------------------------------------------------------------------------

func listParentsHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	rows, err := DB.Query("SELECT uid, uname FROM users ORDER BY uid")
	if err != nil {
		log.Printf("listParents: %v", err)
		JSONError(w, "Failed to load users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	parents := []ParentUser{}
	for rows.Next() {
		var p ParentUser
		if err := rows.Scan(&p.ID, &p.Username); err != nil {
			log.Printf("listParents(scan): %v", err)
			continue
		}
		parents = append(parents, p)
	}
	writeJSON(w, http.StatusOK, parents)
}

func createParentHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(body.Username)
	if username == "" {
		JSONError(w, "Username is required", http.StatusBadRequest)
		return
	}
	if body.Password == "" {
		JSONError(w, "Password is required", http.StatusBadRequest)
		return
	}
	var taken bool
	if err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE uname = $1)", username).Scan(&taken); err != nil {
		log.Printf("createParent(check): %v", err)
		JSONError(w, "Failed to create user", http.StatusInternalServerError)
		return
	}
	if taken {
		JSONError(w, "A user with that username already exists", http.StatusBadRequest)
		return
	}
	var p ParentUser
	err := DB.QueryRow(
		"INSERT INTO users (uname, password) VALUES ($1, $2) RETURNING uid, uname",
		username, body.Password,
	).Scan(&p.ID, &p.Username)
	if err != nil {
		log.Printf("createParent: %v", err)
		JSONError(w, "Failed to create user", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func updateParentHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Username *string `json:"username"`
		Password *string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var p ParentUser
	err := DB.QueryRow("SELECT uid, uname FROM users WHERE uid = $1", id).Scan(&p.ID, &p.Username)
	if err == sql.ErrNoRows {
		JSONError(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("updateParent(load): %v", err)
		JSONError(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	if body.Username != nil {
		username := strings.TrimSpace(*body.Username)
		if username == "" {
			JSONError(w, "Username is required", http.StatusBadRequest)
			return
		}
		var taken bool
		if err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE uname = $1 AND uid <> $2)", username, id).Scan(&taken); err != nil {
			log.Printf("updateParent(check): %v", err)
			JSONError(w, "Failed to update user", http.StatusInternalServerError)
			return
		}
		if taken {
			JSONError(w, "A user with that username already exists", http.StatusBadRequest)
			return
		}
		if _, err := DB.Exec("UPDATE users SET uname = $1 WHERE uid = $2", username, id); err != nil {
			log.Printf("updateParent(uname): %v", err)
			JSONError(w, "Failed to update user", http.StatusInternalServerError)
			return
		}
		p.Username = username
	}
	if body.Password != nil && *body.Password != "" {
		if _, err := DB.Exec("UPDATE users SET password = $1 WHERE uid = $2", *body.Password, id); err != nil {
			log.Printf("updateParent(password): %v", err)
			JSONError(w, "Failed to update user", http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusOK, p)
}

func deleteParentHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	res, err := DB.Exec("DELETE FROM users WHERE uid = $1", id)
	if err != nil {
		log.Printf("deleteParent: %v", err)
		JSONError(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		JSONError(w, "User not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Orders (read-only history)
// ---------------------------------------------------------------------------

func orderItemsFor(orderID string) ([]AdminOrderItem, float64, error) {
	rows, err := DB.Query(`
		SELECT oi.eid, e.ename, oi.quantity, oi.price_at_purchase
		FROM order_item oi
		JOIN equipment e ON oi.eid = e.eid
		WHERE oi.oid = $1
		ORDER BY oi.oiid
	`, orderID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []AdminOrderItem{}
	var total float64
	for rows.Next() {
		var it AdminOrderItem
		if err := rows.Scan(&it.EquipmentID, &it.EquipmentName, &it.Quantity, &it.PriceAtPurchase); err != nil {
			return nil, 0, err
		}
		total += float64(it.Quantity) * it.PriceAtPurchase
		items = append(items, it)
	}
	return items, total, nil
}

func listOrdersHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}

	query := `
		SELECT o.oid, o.uid, u.uname, g.gid, g.gname, s.sid, s.sname, o.purchase_date
		FROM orders o
		JOIN users u ON o.uid = u.uid
		JOIN grade g ON o.gid = g.gid
		JOIN school s ON g.sid = s.sid
	`
	var clauses []string
	var args []any
	add := func(cond string, val string) {
		args = append(args, val)
		clauses = append(clauses, cond+" $"+strconv.Itoa(len(args)))
	}
	q := r.URL.Query()
	if v := q.Get("school_id"); v != "" {
		add("s.sid =", v)
	}
	if v := q.Get("grade_id"); v != "" {
		add("g.gid =", v)
	}
	if v := q.Get("user_id"); v != "" {
		add("o.uid =", v)
	}
	if v := q.Get("from"); v != "" {
		add("o.purchase_date >=", v)
	}
	if v := q.Get("to"); v != "" {
		add("o.purchase_date <=", v)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY o.purchase_date DESC"

	rows, err := DB.Query(query, args...)
	if err != nil {
		log.Printf("listOrders: %v", err)
		JSONError(w, "Failed to load orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	orders := []AdminOrder{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			log.Printf("listOrders(scan): %v", err)
			continue
		}
		orders = append(orders, o)
	}
	writeJSON(w, http.StatusOK, orders)
}

// scanOrder reads one order header row and joins in its line items.
func scanOrder(rows interface{ Scan(...any) error }) (AdminOrder, error) {
	var o AdminOrder
	var purchase time.Time
	if err := rows.Scan(&o.ID, &o.UserID, &o.Username, &o.GradeID, &o.GradeName, &o.SchoolID, &o.SchoolName, &purchase); err != nil {
		return AdminOrder{}, err
	}
	o.PurchaseDate = purchase.Format("2006-01-02 15:04:05")
	items, total, err := orderItemsFor(o.ID)
	if err != nil {
		return AdminOrder{}, err
	}
	o.Items = items
	o.TotalAmount = total
	return o, nil
}

func getOrderHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	row := DB.QueryRow(`
		SELECT o.oid, o.uid, u.uname, g.gid, g.gname, s.sid, s.sname, o.purchase_date
		FROM orders o
		JOIN users u ON o.uid = u.uid
		JOIN grade g ON o.gid = g.gid
		JOIN school s ON g.sid = s.sid
		WHERE o.oid = $1
	`, id)
	o, err := scanOrder(row)
	if err == sql.ErrNoRows {
		JSONError(w, "Order not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("getOrder: %v", err)
		JSONError(w, "Failed to load order", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// ---------------------------------------------------------------------------
// Analytics
// ---------------------------------------------------------------------------

func analyticsSummaryHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}

	summary := AnalyticsSummary{
		RevenueByMonth: []MonthRevenue{},
		TopEquipment:   []TopEquipment{},
		SpendBySchool:  []SchoolSpend{},
	}

	if err := DB.QueryRow("SELECT COALESCE(SUM(quantity * price_at_purchase), 0) FROM order_item").Scan(&summary.TotalRevenue); err != nil {
		log.Printf("analytics(revenue): %v", err)
		JSONError(w, "Failed to load analytics", http.StatusInternalServerError)
		return
	}
	if err := DB.QueryRow("SELECT COUNT(*) FROM orders").Scan(&summary.TotalOrders); err != nil {
		log.Printf("analytics(orders): %v", err)
		JSONError(w, "Failed to load analytics", http.StatusInternalServerError)
		return
	}
	if err := DB.QueryRow("SELECT COUNT(*) FROM cart_entry").Scan(&summary.ActiveCarts); err != nil {
		log.Printf("analytics(carts): %v", err)
		JSONError(w, "Failed to load analytics", http.StatusInternalServerError)
		return
	}
	if err := DB.QueryRow("SELECT COUNT(*) FROM equipment").Scan(&summary.CatalogSize); err != nil {
		log.Printf("analytics(catalog): %v", err)
		JSONError(w, "Failed to load analytics", http.StatusInternalServerError)
		return
	}

	monthRows, err := DB.Query(`
		SELECT to_char(o.purchase_date, 'YYYY-MM') AS month,
		       COALESCE(SUM(oi.quantity * oi.price_at_purchase), 0) AS revenue
		FROM orders o
		JOIN order_item oi ON oi.oid = o.oid
		GROUP BY month
		ORDER BY month
	`)
	if err != nil {
		log.Printf("analytics(byMonth): %v", err)
		JSONError(w, "Failed to load analytics", http.StatusInternalServerError)
		return
	}
	defer monthRows.Close()
	for monthRows.Next() {
		var m MonthRevenue
		if err := monthRows.Scan(&m.Month, &m.Revenue); err != nil {
			continue
		}
		summary.RevenueByMonth = append(summary.RevenueByMonth, m)
	}

	topRows, err := DB.Query(`
		SELECT oi.eid, e.ename,
		       SUM(oi.quantity) AS qty,
		       SUM(oi.quantity * oi.price_at_purchase) AS revenue
		FROM order_item oi
		JOIN equipment e ON oi.eid = e.eid
		GROUP BY oi.eid, e.ename
		ORDER BY revenue DESC
		LIMIT 5
	`)
	if err != nil {
		log.Printf("analytics(top): %v", err)
		JSONError(w, "Failed to load analytics", http.StatusInternalServerError)
		return
	}
	defer topRows.Close()
	for topRows.Next() {
		var t TopEquipment
		if err := topRows.Scan(&t.EquipmentID, &t.Name, &t.Quantity, &t.Revenue); err != nil {
			continue
		}
		summary.TopEquipment = append(summary.TopEquipment, t)
	}

	schoolRows, err := DB.Query(`
		SELECT s.sid, s.sname,
		       COALESCE(SUM(oi.quantity * oi.price_at_purchase), 0) AS revenue
		FROM orders o
		JOIN grade g ON o.gid = g.gid
		JOIN school s ON g.sid = s.sid
		JOIN order_item oi ON oi.oid = o.oid
		GROUP BY s.sid, s.sname
		ORDER BY revenue DESC
	`)
	if err != nil {
		log.Printf("analytics(bySchool): %v", err)
		JSONError(w, "Failed to load analytics", http.StatusInternalServerError)
		return
	}
	defer schoolRows.Close()
	for schoolRows.Next() {
		var s SchoolSpend
		if err := schoolRows.Scan(&s.SchoolID, &s.SchoolName, &s.Revenue); err != nil {
			continue
		}
		summary.SpendBySchool = append(summary.SpendBySchool, s)
	}

	writeJSON(w, http.StatusOK, summary)
}

// ---------------------------------------------------------------------------
// CSV import (school,grade,equipment,price,quantity)
// ---------------------------------------------------------------------------

func importHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSession(w, r); !ok {
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		JSONError(w, "Invalid upload", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		JSONError(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // tolerate ragged rows; we validate column count ourselves
	records, err := reader.ReadAll()
	if err != nil {
		JSONError(w, "Failed to parse CSV", http.StatusBadRequest)
		return
	}

	result := ImportResult{Errors: []ImportRowError{}}

	start := 0
	if len(records) > 0 && len(records[0]) > 0 {
		header := strings.ToLower(strings.Join(records[0], ","))
		if strings.Contains(header, "school") && strings.Contains(header, "equipment") {
			start = 1
		}
	}

	tx, err := DB.Begin()
	if err != nil {
		log.Printf("import(begin): %v", err)
		JSONError(w, "Failed to import", http.StatusInternalServerError)
		return
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	for i := start; i < len(records); i++ {
		rowNumber := i + 1
		cols := records[i]
		if len(cols) < 5 {
			result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Expected 5 columns: school,grade,equipment,price,quantity"})
			continue
		}
		schoolName := strings.TrimSpace(cols[0])
		gradeName := strings.TrimSpace(cols[1])
		equipmentName := strings.TrimSpace(cols[2])
		priceRaw := strings.TrimSpace(cols[3])
		quantityRaw := strings.TrimSpace(cols[4])

		if schoolName == "" || gradeName == "" || equipmentName == "" {
			result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "School, grade and equipment are required"})
			continue
		}
		price, err := strconv.ParseFloat(priceRaw, 64)
		if err != nil || price < 0 {
			result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Invalid price \"" + priceRaw + "\""})
			continue
		}
		quantity, err := strconv.Atoi(quantityRaw)
		if err != nil || quantity <= 0 {
			result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Invalid quantity \"" + quantityRaw + "\""})
			continue
		}

		// Upsert school.
		var schoolID int
		if err := tx.QueryRow("SELECT sid FROM school WHERE sname = $1", schoolName).Scan(&schoolID); err == sql.ErrNoRows {
			if err := tx.QueryRow("INSERT INTO school (sname) VALUES ($1) RETURNING sid", schoolName).Scan(&schoolID); err != nil {
				result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Failed to create school"})
				continue
			}
		} else if err != nil {
			result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Database error"})
			continue
		}

		// Upsert grade within that school.
		var gradeID int
		if err := tx.QueryRow("SELECT gid FROM grade WHERE sid = $1 AND gname = $2", schoolID, gradeName).Scan(&gradeID); err == sql.ErrNoRows {
			if err := tx.QueryRow("INSERT INTO grade (sid, gname) VALUES ($1, $2) RETURNING gid", schoolID, gradeName).Scan(&gradeID); err != nil {
				result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Failed to create grade"})
				continue
			}
		} else if err != nil {
			result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Database error"})
			continue
		}

		// Upsert equipment by name, keeping catalog price in sync with the sheet.
		var equipmentID int
		if err := tx.QueryRow("SELECT eid FROM equipment WHERE ename = $1", equipmentName).Scan(&equipmentID); err == sql.ErrNoRows {
			if err := tx.QueryRow("INSERT INTO equipment (ename, price) VALUES ($1, $2) RETURNING eid", equipmentName, price).Scan(&equipmentID); err != nil {
				result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Failed to create equipment"})
				continue
			}
		} else if err != nil {
			result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Database error"})
			continue
		} else {
			if _, err := tx.Exec("UPDATE equipment SET price = $1 WHERE eid = $2", price, equipmentID); err != nil {
				result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Failed to update equipment price"})
				continue
			}
		}

		// Upsert the requirement line for this grade+equipment.
		var existing bool
		if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM requirement WHERE gid = $1 AND eid = $2)", gradeID, equipmentID).Scan(&existing); err != nil {
			result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Database error"})
			continue
		}
		if existing {
			if _, err := tx.Exec("UPDATE requirement SET quantity = $1 WHERE gid = $2 AND eid = $3", quantity, gradeID, equipmentID); err != nil {
				result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Failed to update requirement"})
				continue
			}
			result.Updated++
		} else {
			if _, err := tx.Exec("INSERT INTO requirement (gid, eid, quantity) VALUES ($1, $2, $3)", gradeID, equipmentID, quantity); err != nil {
				result.Errors = append(result.Errors, ImportRowError{Row: rowNumber, Message: "Failed to create requirement"})
				continue
			}
			result.Created++
		}
	}

	result.Skipped = len(result.Errors)

	if err := tx.Commit(); err != nil {
		log.Printf("import(commit): %v", err)
		JSONError(w, "Failed to import", http.StatusInternalServerError)
		return
	}
	committed = true

	writeJSON(w, http.StatusOK, result)
}
