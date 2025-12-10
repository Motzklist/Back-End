package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)
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