package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// ==========================================
// 1. Helper Logic Tests (Session ID)
// ==========================================

func TestSessionID_Length(t *testing.T) {
	id := generateSessionID()
	if len(id) != 32 {
		t.Errorf("Expected length 32, got %d", len(id))
	}
}

func TestSessionID_Uniqueness(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()
	if id1 == id2 {
		t.Error("Generated identical session IDs")
	}
}

func TestSessionID_Chars(t *testing.T) {
	id := generateSessionID()
	if strings.Contains(id, " ") {
		t.Error("Session ID contains spaces")
	}
}

// ==========================================
// 2. Middleware & CORS Tests
// ==========================================

// TestCORS_OriginHeader removed per request

func TestCORS_MethodsHeader(t *testing.T) {
	req, _ := http.NewRequest("OPTIONS", "/api/schools", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(enableCORS(func(w http.ResponseWriter, r *http.Request) {}))
	handler.ServeHTTP(rr, req)

	methods := rr.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "POST") {
		t.Error("CORS methods should include POST")
	}
}

func TestCORS_OptionsStatus(t *testing.T) {
	req, _ := http.NewRequest("OPTIONS", "/api/schools", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(enableCORS(func(w http.ResponseWriter, r *http.Request) {}))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("OPTIONS should return 200, got %v", rr.Code)
	}
}

// ==========================================
// 3. Schools API Tests
// ==========================================

func TestSchools_Status(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/schools", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getSchoolsHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %v", rr.Code)
	}
}

func TestSchools_IsJSON(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/schools", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getSchoolsHandler)
	handler.ServeHTTP(rr, req)
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Error("Expected application/json content type")
	}
}

func TestSchools_NotEmpty(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/schools", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getSchoolsHandler)
	handler.ServeHTTP(rr, req)
	if rr.Body.Len() == 0 {
		t.Error("Response body is empty")
	}
}

func TestSchools_ContainsBenGurion(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/schools", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getSchoolsHandler)
	handler.ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "Ben Gurion") {
		t.Error("Expected 'Ben Gurion' in response")
	}
}

// ==========================================
// 4. Grades API Tests
// ==========================================

func TestGrades_ValidRequest(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/grades?school_id=1", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getGradesHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %v", rr.Code)
	}
}

func TestGrades_MissingParams(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/grades", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getGradesHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

func TestGrades_EmptyParam(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/grades?school_id=", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getGradesHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty param, got %v", rr.Code)
	}
}

func TestGrades_ResponseList(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/grades?school_id=1", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getGradesHandler)
	handler.ServeHTTP(rr, req)
	var grades []Grade
	if err := json.NewDecoder(rr.Body).Decode(&grades); err != nil {
		t.Error("Failed to decode grades JSON")
	}
	if len(grades) == 0 {
		t.Error("Returned empty grades list")
	}
}

func TestGrades_Contains12thGrade(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/grades?school_id=1", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getGradesHandler)
	handler.ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "12th Grade") {
		t.Error("Expected '12th Grade' in response")
	}
}

// ==========================================
// 5. Equipment API Tests
// ==========================================

func TestEquipment_Specific(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/equipment?school_id=1&grade_id=9", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getEquipmentListsHandler)
	handler.ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "Notebook (Ruled)") {
		t.Error("Missing specific item for school 1 grade 9")
	}
}

func TestEquipment_Default(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/equipment?school_id=99&grade_id=99", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getEquipmentListsHandler)
	handler.ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "Binder") {
		t.Error("Missing default item")
	}
}

func TestEquipment_MissingSchool(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/equipment?grade_id=9", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getEquipmentListsHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

func TestEquipment_MissingGrade(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/equipment?school_id=1", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getEquipmentListsHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

func TestEquipment_Structure(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/equipment?school_id=1&grade_id=9", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getEquipmentListsHandler)
	handler.ServeHTTP(rr, req)
	var resp EquipmentListResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Error("Invalid JSON structure")
	}
	if len(resp.Items) == 0 {
		t.Error("Items list is empty")
	}
}

// ==========================================
// 6. Login Tests
// ==========================================

func TestLogin_ValidUser(t *testing.T) {
	body := `{"username": "avner", "password": "2004"}`
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(postLoginHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Login failed for valid user, got %v", rr.Code)
	}
}

func TestLogin_ValidAdmin(t *testing.T) {
	body := `{"username": "admin", "password": "1234"}`
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(postLoginHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Login failed for admin, got %v", rr.Code)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	body := `{"username": "avner", "password": "wrong"}`
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(postLoginHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %v", rr.Code)
	}
}

func TestLogin_UnknownUser(t *testing.T) {
	body := `{"username": "ghost", "password": "boo"}`
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(postLoginHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %v", rr.Code)
	}
}

func TestLogin_EmptyBody(t *testing.T) {
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBufferString(""))
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(postLoginHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty body, got %v", rr.Code)
	}
}

func TestLogin_MalformedJSON(t *testing.T) {
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBufferString(`{"user":...`))
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(postLoginHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for bad JSON, got %v", rr.Code)
	}
}

func TestLogin_SetsCookie(t *testing.T) {
	body := `{"username": "avner", "password": "2004"}`
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(postLoginHandler)
	handler.ServeHTTP(rr, req)
	
	found := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == "sessionid" {
			found = true
			delete(sessions, c.Value) // Cleanup
		}
	}
	if !found {
		t.Error("Session cookie not set on login")
	}
}

func TestLogin_WrongMethod(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/login", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(postLoginHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed, got %v", rr.Code)
	}
}

// ==========================================
// 7. Auth Status Tests
// ==========================================

func TestAuthStatus_NoCookie(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/auth/status", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(authStatusHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %v", rr.Code)
	}
}

func TestAuthStatus_InvalidCookie(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/auth/status", nil)
	req.AddCookie(&http.Cookie{Name: "sessionid", Value: "fake-123"})
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(authStatusHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %v", rr.Code)
	}
}

func TestAuthStatus_ValidSession(t *testing.T) {
	sid := "test-session-auth"
	sessions[sid] = "1"
	defer delete(sessions, sid)

	req, _ := http.NewRequest("GET", "/api/auth/status", nil)
	req.AddCookie(&http.Cookie{Name: "sessionid", Value: sid})
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(authStatusHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %v", rr.Code)
	}
}

func TestAuthStatus_ReturnsUsername(t *testing.T) {
	sid := "test-session-name"
	sessions[sid] = "1"
	defer delete(sessions, sid)

	req, _ := http.NewRequest("GET", "/api/auth/status", nil)
	req.AddCookie(&http.Cookie{Name: "sessionid", Value: sid})
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(authStatusHandler)
	handler.ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "avner") {
		t.Error("Response did not contain username")
	}
}

// ==========================================
// 8. Logout Tests
// ==========================================

func TestLogout_Status(t *testing.T) {
	req, _ := http.NewRequest("POST", "/api/logout", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(logoutHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %v", rr.Code)
	}
}

func TestLogout_ClearsCookie(t *testing.T) {
	req, _ := http.NewRequest("POST", "/api/logout", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(logoutHandler)
	handler.ServeHTTP(rr, req)
	
	cleared := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == "sessionid" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("Session cookie not cleared (MaxAge < 0)")
	}
}

func TestLogout_RemovesFromMap(t *testing.T) {
	sid := "test-logout-map"
	sessions[sid] = "1"
	req, _ := http.NewRequest("POST", "/api/logout", nil)
	req.AddCookie(&http.Cookie{Name: "sessionid", Value: sid})
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(logoutHandler)
	handler.ServeHTTP(rr, req)
	
	if _, exists := sessions[sid]; exists {
		t.Error("Session ID still in map after logout")
	}
}

// ==========================================
// 9. Cart Tests
// ==========================================

func TestCart_Get_NoUser(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/cart", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getPostCartHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

func TestCart_Get_Valid(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/cart?userid=1", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getPostCartHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %v", rr.Code)
	}
}

func TestCart_Get_ReturnArray(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/cart?userid=1", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getPostCartHandler)
	handler.ServeHTTP(rr, req)
	if !strings.HasPrefix(strings.TrimSpace(rr.Body.String()), "[") {
		t.Error("Expected JSON array")
	}
}

func TestCart_Post_Valid(t *testing.T) {
	body := `[{"id":"cart-1", "items":[]}]`
	req, _ := http.NewRequest("POST", "/api/cart?userid=1", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getPostCartHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %v", rr.Code)
	}
}

func TestCart_Post_NoUser(t *testing.T) {
	req, _ := http.NewRequest("POST", "/api/cart", bytes.NewBufferString("[]"))
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getPostCartHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

func TestCart_Post_BadJSON(t *testing.T) {
	req, _ := http.NewRequest("POST", "/api/cart?userid=1", bytes.NewBufferString("[{..."))
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getPostCartHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", rr.Code)
	}
}

func TestCart_Delete_Method(t *testing.T) {
	req, _ := http.NewRequest("DELETE", "/api/cart?userid=1", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getPostCartHandler)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %v", rr.Code)
	}
}

// ==========================================
// FINAL SUMMARY
// ==========================================

func TestMain(m *testing.M) {
	code := m.Run()

	fmt.Println("\n--------------------------------------------------")
	if code == 0 {
		fmt.Println("\033[32m[SUCCESS] ALL TESTS PASSED! \033[0m")
		fmt.Println("\033[32m(See individual checks above for details)\033[0m")
	} else {
		fmt.Println("\033[31m[FAILURE] SOME TESTS FAILED! \033[0m")
	}
	fmt.Println("--------------------------------------------------")

	os.Exit(code)
}