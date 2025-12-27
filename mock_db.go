package main

import "fmt"

// Schools data (all possible schools)
var MockSchools = []School{
	{"1", "Ben Gurion"},
	{"2", "ORT"},
	{"3", "Brener"},
	{"4", "Herzel"},
	{"5", "Begin"},
}

// Grades data (9-12)
var MockGrades = []Grade{
	{"9", "9th Grade"},
	{"10", "10th Grade"},
	{"11", "11th Grade"},
	{"12", "12th Grade"},
}

// Classes data (1-8)
var MockClasses = []Class{
	{"1", "Class 1"},
	{"2", "Class 2"},
	{"3", "Class 3"},
	{"4", "Class 4"},
	{"5", "Class 5"},
	{"6", "Class 6"},
	{"7", "Class 7"},
	{"8", "Class 8"},
}

// Equipment data (This is complex and needs filtering logic)
// To simulate different lists, we'll use a map keyed by a combination string: SchoolID-GradeID-ClassID
var MockEquipmentLists = map[string][]Equipment{
	// Example: List for Ben Gurion (1), 9th Grade (9), Class 1 (1)
	"1-9-1": {
		{"101", "Notebook (Ruled)", 5},
		{"102", "Pencil", 12},
		{"103", "Math Textbook - Algebra I", 1},
	},
	// Example: List for ORT (2), 12th Grade (12), Class 5 (5)
	"2-12-5": {
		{"201", "Laptop (Required)", 1},
		{"202", "Engineering Calculator", 1},
		{"203", "Physics Textbook - Advanced", 1},
	},
	// Default list for all other combinations
	"default": {
		{"901", "Binder (3-ring)", 2},
		{"902", "Highlighters", 4},
	},
}

// --- Mock DB Functions ---

// Get all schools. Doesn't need filtering.
func GetSchools() []School {
	return MockSchools
}

// Get grades for a specific school. Since grades are the same for all schools,
// this function only validates the schoolID exists.
func GetGradesBySchoolID(schoolID string) []Grade {
	// In a real DB, you'd filter grades by school. Here, we just ensure the school is valid.
	for _, s := range MockSchools {
		if s.ID == schoolID {
			return MockGrades // School is valid, return all grades
		}
	}
	return nil // School not found
}

// Get classes for a specific grade.
func GetClassesByGradeID(schoolID, gradeID string) []Class {
	// Simple validation: Ensure school and grade IDs are valid before returning classes.
	if GetGradesBySchoolID(schoolID) == nil {
		return nil // Invalid school
	}
	// In this simple mock, we don't need gradeID for filtering classes (always 1-8).
	return MockClasses
}

// Get equipment list based on selection.
func GetEquipmentList(schoolID, gradeID, classID string) []Equipment {
	key := fmt.Sprintf("%s-%s-%s", schoolID, gradeID, classID)

	// Attempt to find a specific list
	if list, ok := MockEquipmentLists[key]; ok {
		return list
	}

	// Return a default list if no specific list is defined
	return MockEquipmentLists["default"]
}

// ======NEW======
// data for login page
var MockUsers = []User{
	{UserID: "1", Username: "avner", Password: "2004"},
	{UserID: "2", Username: "admin", Password: "1234"},
	{UserID: "3", Username: "noam", Password: "1919"},
}

// CartEntry structure for frontend compatibility
// (matches what the frontend expects)
type CartEntry struct {
	ID        string      `json:"id"`
	Timestamp int64       `json:"timestamp"`
	School    School      `json:"school"`
	Grade     Grade       `json:"grade"`
	Class     Class       `json:"class"`
	Items     []Equipment `json:"items"`
}

// data for cart
var MockCarts = map[string][]CartEntry{
	"1": {
		{
			ID:        "cart-1",
			Timestamp: 1700000000,
			School:    School{ID: "1", Name: "Ben Gurion"},
			Grade:     Grade{ID: "9", Name: "9th Grade"},
			Class:     Class{ID: "1", Name: "Class 1"},
			Items: []Equipment{
				{ID: "101", Name: "Notebook", Quantity: 2},
				{ID: "102", Name: "Engineering Calculator", Quantity: 1},
				{ID: "103", Name: "Physics Textbook - Advanced", Quantity: 1},
			},
		},
	},
	"2": {
		{
			ID:        "cart-2",
			Timestamp: 1700000001,
			School:    School{ID: "2", Name: "ORT"},
			Grade:     Grade{ID: "12", Name: "12th Grade"},
			Class:     Class{ID: "5", Name: "Class 5"},
			Items: []Equipment{
				{ID: "201", Name: "Laptop (Required)", Quantity: 1},
				{ID: "202", Name: "Engineering Calculator", Quantity: 1},
				{ID: "203", Name: "Physics Textbook - Beginners", Quantity: 1},
			},
		},
	},
}
