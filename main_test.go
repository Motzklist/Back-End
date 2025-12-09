// main_test.go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---- Helper to decode JSON ----

func decodeJSON[T any](t *testing.T, body *httptest.ResponseRecorder, out *T) {
	t.Helper()
	if err := json.Unmarshal(body.Body.Bytes(), out); err != nil {
		t.Fatalf("failed to decode JSON: %v\nbody=%s", err, body.Body.String())
	}
}

// ---- Handler tests ----

func TestGetSchoolsHandler_OK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/schools", nil)
	rr := httptest.NewRecorder()

	// wrap with CORS, like in main()
	handler := enableCORS(getSchoolsHandler)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	// Check CORS header
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected CORS header '*', got %q", got)
	}

	var schools []School
	decodeJSON(t, rr, &schools)

	if len(schools) == 0 {
		t.Fatalf("expected at least one school, got 0")
	}
}

func TestGetGradesHandler_MissingParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/grades", nil)
	rr := httptest.NewRecorder()

	handler := enableCORS(getGradesHandler)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for missing school_id, got %d", rr.Code)
	}
}

func TestGetGradesHandler_ValidSchool(t *testing.T) {
	// "1" is valid according to MockSchools in mock_db.go
	req := httptest.NewRequest(http.MethodGet, "/api/grades?school_id=1", nil)
	rr := httptest.NewRecorder()

	handler := enableCORS(getGradesHandler)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var grades []Grade
	decodeJSON(t, rr, &grades)
	if len(grades) == 0 {
		t.Fatalf("expected at least one grade for school_id=1")
	}
}

func TestGetGradesHandler_InvalidSchool(t *testing.T) {
	// school_id=999 should return nil from GetGradesBySchoolID
	req := httptest.NewRequest(http.MethodGet, "/api/grades?school_id=999", nil)
	rr := httptest.NewRecorder()

	handler := enableCORS(getGradesHandler)
	handler.ServeHTTP(rr, req)

	// current implementation will encode `nil` as JSON "null" with 200 OK.
	// We at least check it doesn't crash and returns 200.
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 even for invalid school (mocked), got %d", rr.Code)
	}
}

func TestGetClassesHandler_MissingParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/classes?school_id=1", nil) // missing grade_id
	rr := httptest.NewRecorder()

	handler := enableCORS(getClassesHandler)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for missing params, got %d", rr.Code)
	}
}

func TestGetClassesHandler_OK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/classes?school_id=1&grade_id=9", nil)
	rr := httptest.NewRecorder()

	handler := enableCORS(getClassesHandler)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var classes []Class
	decodeJSON(t, rr, &classes)
	if len(classes) == 0 {
		t.Fatalf("expected at least one class")
	}
}

func TestGetEquipmentListsHandler_MissingParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/equipment?school_id=1&grade_id=9", nil) // missing class_id
	rr := httptest.NewRecorder()

	handler := enableCORS(getEquipmentListsHandler)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for missing params, got %d", rr.Code)
	}
}

func TestGetEquipmentListsHandler_DefaultList(t *testing.T) {
	// This combination is not explicitly listed in MockEquipmentLists, so we hit "default"
	req := httptest.NewRequest(http.MethodGet, "/api/equipment?school_id=1&grade_id=9&class_id=2", nil)
	rr := httptest.NewRecorder()

	handler := enableCORS(getEquipmentListsHandler)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var equipment []Equipment
	decodeJSON(t, rr, &equipment)
	if len(equipment) == 0 {
		t.Fatalf("expected at least one equipment item")
	}
}
func TestGetSchools(t *testing.T) {
	schools := GetSchools()
	if len(schools) == 0 {
		t.Fatalf("expected non-empty schools list")
	}
}

func TestGetGradesBySchoolID_Valid(t *testing.T) {
	grades := GetGradesBySchoolID("1") // "1" exists in MockSchools
	if len(grades) == 0 {
		t.Fatalf("expected grades for valid school ID")
	}
}

func TestGetGradesBySchoolID_Invalid(t *testing.T) {
	grades := GetGradesBySchoolID("999")
	if grades != nil {
		t.Fatalf("expected nil for invalid school ID, got %+v", grades)
	}
}

func TestGetClassesByGradeID_Valid(t *testing.T) {
	classes := GetClassesByGradeID("1", "9")
	if len(classes) == 0 {
		t.Fatalf("expected classes for valid school/grade")
	}
}

func TestGetClassesByGradeID_InvalidSchool(t *testing.T) {
	classes := GetClassesByGradeID("999", "9")
	if classes != nil {
		t.Fatalf("expected nil for invalid school ID")
	}
}

func TestGetEquipmentList_SpecificKey(t *testing.T) {
	// "1-9-1" is explicitly defined in MockEquipmentLists
	list := GetEquipmentList("1", "9", "1")
	if len(list) == 0 {
		t.Fatalf("expected non-empty list for 1-9-1")
	}
}

func TestGetEquipmentList_DefaultKey(t *testing.T) {
	// some combination that is not explicitly defined → should hit "default"
	list := GetEquipmentList("123", "456", "789")
	if len(list) == 0 {
		t.Fatalf("expected non-empty default list")
	}
}