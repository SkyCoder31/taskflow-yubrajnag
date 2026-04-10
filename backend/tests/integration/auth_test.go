package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegister_Success(t *testing.T) {
	router := setupRouter(t)

	body := `{"name":"Alice","email":"alice@example.com","password":"strongpass123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	token := resp["data"]["token"]
	if token == "" {
		t.Fatal("expected token in response, got empty")
	}

	// Token should have 3 parts (header.payload.signature)
	parts := bytes.Count([]byte(token), []byte("."))
	if parts != 2 {
		t.Fatalf("expected JWT with 2 dots, got %d", parts)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	router := setupRouter(t)

	body := `{"name":"Bob","email":"duplicate@example.com","password":"strongpass123"}`

	// First registration — should succeed
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/auth/register", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first register: expected 201, got %d: %s", w1.Code, w1.Body.String())
	}

	// Second registration with same email — should fail with 409
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/auth/register", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("duplicate register: expected 409, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestLogin_Success(t *testing.T) {
	router := setupRouter(t)

	// Register first
	regBody := `{"name":"Charlie","email":"charlie@example.com","password":"strongpass123"}`
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/auth/register", bytes.NewBufferString(regBody))
	req1.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", w1.Code, w1.Body.String())
	}

	// Login with the same credentials
	loginBody := `{"email":"charlie@example.com","password":"strongpass123"}`
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/auth/login", bytes.NewBufferString(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp map[string]map[string]string
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp["data"]["token"] == "" {
		t.Fatal("expected token in login response")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	router := setupRouter(t)

	// Register
	regBody := `{"name":"Diana","email":"diana@example.com","password":"correctpass123"}`
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/auth/register", bytes.NewBufferString(regBody))
	req1.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", w1.Code)
	}

	// Login with wrong password — should get 401 (not 404, to prevent enumeration)
	loginBody := `{"email":"diana@example.com","password":"wrongpass123"}`
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/auth/login", bytes.NewBufferString(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: expected 401, got %d: %s", w2.Code, w2.Body.String())
	}
}
