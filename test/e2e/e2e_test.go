//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/stemsi/exstem-backend/internal/model"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultBaseURL = "http://localhost:8050/api/v1"
	defaultDBURL   = "postgres://postgres:postgres@localhost:5555/exstem?sslmode=disable"
	adminEmail     = "e2e_admin@example.com"
	adminPass      = "password123"
	studentNISN    = "e2e_student"
	studentPass    = "password123"
	studentName    = "E2E Student"
)

var (
	baseURL        string
	dbURL          string
	initialClassID int
	adminToken     string
	studentToken   string
	examID         string
)

func TestMain(m *testing.M) {
	// Load .env if present (ignore error)
	_ = godotenv.Load("../../.env")

	// Set config from env or defaults
	baseURL = os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	dbURL = os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = defaultDBURL
	}

	// 1. Setup Database (Clean or Seed Admin)
	if err := setupInitialAdmin(); err != nil {
		fmt.Printf("Setup failed: %v\n", err)
		os.Exit(1)
	}

	// 2. Run Tests
	code := m.Run()

	// 3. Cleanup optional
	os.Exit(code)
}

func setupInitialAdmin() error {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("db connect: %w", err)
	}
	defer conn.Close(ctx)

	// Cleanup previous test data (order matters due to FK)
	tables := []string{"student_answers", "exam_sessions", "questions", "exam_target_rules", "exams", "students", "classes", "admins"}
	for _, table := range tables {
		if _, err := conn.Exec(ctx, fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			return fmt.Errorf("cleanup %s: %w", table, err)
		}
	}

	// Create initial admin
	hash, _ := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)

	// Insert Role (Super Admin) if not exists
	var roleID int
	err = conn.QueryRow(ctx, `INSERT INTO roles (name) VALUES ('Super Admin') ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name RETURNING id`).Scan(&roleID)
	if err != nil {
		return fmt.Errorf("insert role: %w", err)
	}

	// Give all permissions to Super Admin role
	_, err = conn.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) 
		SELECT $1, id FROM permissions 
		ON CONFLICT DO NOTHING`, roleID)
	if err != nil {
		return fmt.Errorf("insert permissions: %w", err)
	}

	// Insert Admin
	_, err = conn.Exec(ctx, `INSERT INTO admins (name, email, password_hash, role_id) 
		VALUES ('E2E Admin', $1, $2, $3)
		ON CONFLICT (email) DO UPDATE SET password_hash = $2`, adminEmail, string(hash), roleID)
	if err != nil {
		return fmt.Errorf("insert admin: %w", err)
	}

	// Create sample class if not exists (Class 1) or get ID
	err = conn.QueryRow(ctx, `INSERT INTO classes (grade_level, major_code, group_number) VALUES ('X', 'RPL', 1) 
		ON CONFLICT (grade_level, major_code, group_number) DO UPDATE SET grade_level=EXCLUDED.grade_level 
		RETURNING id`).Scan(&initialClassID)
	if err != nil {
		return fmt.Errorf("insert/get class: %w", err)
	}

	// Apply schema migrations for missing columns (QuestionType, ScoreValue)
	_, err = conn.Exec(ctx, `
		ALTER TABLE questions ADD COLUMN IF NOT EXISTS question_type VARCHAR(50) NOT NULL DEFAULT 'MULTIPLE_CHOICE';
		ALTER TABLE questions ADD COLUMN IF NOT EXISTS score_value INT NOT NULL DEFAULT 1;
	`)
	if err != nil {
		return fmt.Errorf("migrate questions: %w", err)
	}

	return nil
}

func TestE2EFlow(t *testing.T) {
	// Step 1: Login as Admin
	t.Run("AdminLogin", func(t *testing.T) {
		reqBody := map[string]string{
			"email":    adminEmail,
			"password": adminPass,
		}
		resp, err := post("/auth/admin/login", reqBody, "")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d: %s", resp.StatusCode, readBody(resp))
		}

		var body struct {
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		decodeJSON(t, resp, &body)
		adminToken = body.Data.Token
		if adminToken == "" {
			t.Fatal("token missing")
		}
		t.Logf("Admin Token received")
	})

	// Step 2: Create Student (Admin)
	t.Run("CreateStudent", func(t *testing.T) {
		reqBody := model.CreateStudentRequest{
			NISN:     studentNISN,
			Name:     studentName,
			Password: studentPass,
			ClassID:  initialClassID,
		}
		resp, err := post("/admin/students", reqBody, adminToken)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// If student already exists (409 or similar handled by existing logic?), we check status
		// Here we assume cleaner DB or proper upsert handling, but let's see.
		// If 500 duplicate key, test might fail. For now, assume fresh run or conflict error.
		// Actually duplicate key returns 500 unless handled.
		// Let's assume the standard flow works.
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			// If conflict, we might get 500 currently. Let's see.
			// Ideally we handle it. But for E2E we expect creation success.
			// If this fails on re-run, delete student manualy.
			t.Fatalf("status %d: %s", resp.StatusCode, readBody(resp))
		}
		t.Logf("Student Created")
	})

	// Step 2b: Create Duplicate Student (Expect 409)
	t.Run("CreateDuplicateStudent", func(t *testing.T) {
		reqBody := model.CreateStudentRequest{
			NISN:     studentNISN,
			Name:     studentName,
			Password: studentPass,
			ClassID:  initialClassID,
		}
		resp, err := post("/admin/students", reqBody, adminToken)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Expected status 409 Conflict, got %d. Body: %s", resp.StatusCode, readBody(resp))
		} else {
			t.Logf("Duplicate Student Rejected Correctly (409)")
		}
	})

	// Step 3: Login as Student
	t.Run("StudentLogin", func(t *testing.T) {
		reqBody := map[string]string{
			"nisn":     studentNISN,
			"password": studentPass,
		}
		resp, err := post("/auth/student/login", reqBody, "")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d: %s", resp.StatusCode, readBody(resp))
		}

		var body struct {
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		decodeJSON(t, resp, &body)
		studentToken = body.Data.Token
		if studentToken == "" {
			t.Fatal("student token missing")
		}
		t.Logf("Student Token received")
	})

	// Step 4: Create Exam (Admin)
	t.Run("CreateExam", func(t *testing.T) {
		start := time.Now().Add(1 * time.Hour)
		end := start.Add(2 * time.Hour)
		reqBody := model.CreateExamRequest{
			Title:           "E2E Test Exam",
			ScheduledStart:  &start,
			ScheduledEnd:    &end,
			DurationMinutes: 60,
			EntryToken:      "TOKEN123",
		}
		resp, err := post("/admin/exams", reqBody, adminToken)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status %d: %s", resp.StatusCode, readBody(resp))
		}

		var body struct {
			Data struct {
				Exam model.Exam `json:"exam"`
			} `json:"data"`
		}
		decodeJSON(t, resp, &body)
		examID = body.Data.Exam.ID.String()
		if examID == "" {
			t.Fatal("exam ID missing")
		}
		t.Logf("Exam Created: %s", examID)
	})

	// Step 5: Add Target Rule (Admin)
	t.Run("AddTargetRule", func(t *testing.T) {
		reqBody := model.AddTargetRuleRequest{
			TargetType:  "CLASS",
			TargetValue: strconv.Itoa(initialClassID),
		}
		resp, err := post(fmt.Sprintf("/admin/exams/%s/target-rules", examID), reqBody, adminToken)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status %d: %s", resp.StatusCode, readBody(resp))
		}
		t.Logf("Target Rule Added")
	})

	// Step 6: Add Question (Admin)
	t.Run("AddQuestion", func(t *testing.T) {
		optionsJSON, _ := json.Marshal([]string{"3", "4", "5", "6"})
		reqBody := model.AddQuestionRequest{
			QuestionText:  "What is 2+2?",
			QuestionType:  "MULTIPLE_CHOICE",
			Options:       json.RawMessage(optionsJSON),
			CorrectOption: "1", // Index 1 -> "4"
			OrderNum:      1,
			ScoreValue:    10,
		}
		// Note: Question model options structure depends on implementation.
		// model.Question has Options json.RawMessage.
		// AddQuestionRequest defines Options as json.RawMessage too?
		// Let's check model.Question.
		// Assuming simple array works if validated.

		resp, err := post(fmt.Sprintf("/admin/exams/%s/questions", examID), reqBody, adminToken)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status %d: %s", resp.StatusCode, readBody(resp))
		}
		t.Logf("Question Added")
	})

	// Step 7: Publish Exam (Admin)
	t.Run("PublishExam", func(t *testing.T) {
		resp, err := post(fmt.Sprintf("/admin/exams/%s/publish", examID), nil, adminToken)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d: %s", resp.StatusCode, readBody(resp))
		}
		t.Logf("Exam Published")
	})

	// Step 8: check lobby
	t.Run("CheckLobby", func(t *testing.T) {
		resp, err := get("/student/lobby", studentToken)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d: %s", resp.StatusCode, readBody(resp))
		}

		var body struct {
			Data struct {
				Exams []struct {
					ID string `json:"id"`
				} `json:"exams"`
			} `json:"data"`
		}
		decodeJSON(t, resp, &body)

		found := false
		for _, e := range body.Data.Exams {
			if e.ID == examID {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("Exam not found in lobby (check target rules)")
		}
		t.Logf("Exam found in lobby")
	})

	// Step 9: Join Exam (Student)
	t.Run("JoinExam", func(t *testing.T) {
		reqBody := model.JoinExamRequest{
			EntryToken: "TOKEN123",
		}
		resp, err := post(fmt.Sprintf("/student/exams/%s/join", examID), reqBody, studentToken)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should receive 201 Created or 200 OK
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d: %s", resp.StatusCode, readBody(resp))
		}
		t.Logf("Joined Exam")
	})

	// Step 10: Verify Permissions (Student tries Admin action)
	t.Run("VerifyPermissionFails", func(t *testing.T) {
		resp, err := post("/admin/exams", nil, studentToken)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 403/401, got %d", resp.StatusCode)
		}
	})

	// Step 11: Get Exam Results (Admin)
	t.Run("GetExamResults", func(t *testing.T) {
		// 1. Get all results
		resp, err := get(fmt.Sprintf("/admin/exams/%s/results", examID), adminToken)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d: %s", resp.StatusCode, readBody(resp))
		}

		var body struct {
			Data struct {
				Results []struct {
					StudentID int    `json:"student_id"`
					Name      string `json:"name"`
				} `json:"results"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("json decode: %v", err)
		}

		// Check if student is in results
		found := false
		for _, r := range body.Data.Results {
			if r.Name == studentName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Student %s not found in exam results", studentName)
		}

		// 2. Filter by Class ID
		respFilter, err := get(fmt.Sprintf("/admin/exams/%s/results?class_id=%d", examID, initialClassID), adminToken)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer respFilter.Body.Close()

		if respFilter.StatusCode != http.StatusOK {
			t.Fatalf("filter status %d: %s", respFilter.StatusCode, readBody(respFilter))
		}

		// 3. Filter by Wrong Class ID
		respEmpty, err := get(fmt.Sprintf("/admin/exams/%s/results?class_id=999", examID), adminToken)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer respEmpty.Body.Close()

		var bodyEmpty struct {
			Data struct {
				Results []struct{} `json:"results"`
			} `json:"data"`
		}
		json.NewDecoder(respEmpty.Body).Decode(&bodyEmpty)
		if len(bodyEmpty.Data.Results) > 0 {
			t.Errorf("Expected empty results for wrong class_id, got %d", len(bodyEmpty.Data.Results))
		}
	})
}

// Helpers

func post(path string, body interface{}, token string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBytes, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(jsonBytes)
	}

	req, err := http.NewRequest("POST", baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

func get(path string, token string) (*http.Response, error) {
	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	return client.Do(req)
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func decodeJSON(t *testing.T, resp *http.Response, v interface{}) {
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("json decode: %v", err)
	}
}
