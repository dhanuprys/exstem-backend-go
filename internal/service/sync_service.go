package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/stemsi/exstem-backend/internal/helper"
	"golang.org/x/crypto/bcrypt"
)

type SyncService struct {
	pool        *pgxpool.Pool
	log         zerolog.Logger
	authService *AuthService
}

func NewSyncService(pool *pgxpool.Pool, authService *AuthService, log zerolog.Logger) *SyncService {
	return &SyncService{
		pool:        pool,
		log:         log,
		authService: authService,
	}
}

// ----------------------------------------------------------------------
// External API Structs
// ----------------------------------------------------------------------

type StudentAPIResponse struct {
	Status string              `json:"status"`
	Total  int                 `json:"total"`
	Data   []StudentAPIPayload `json:"data"`
}

type StudentAPIPayload struct {
	Nis      string `json:"nis"`
	Nisn     string `json:"nisn"`
	Nama     string `json:"nama"`
	Kelamin  string `json:"kelamin"`
	Kelas    string `json:"kelas"`
	Jurusan  string `json:"jurusan"`
	Kelompok string `json:"kelompok"`
	Agama    string `json:"agama"`
}

type TeacherAPIResponse struct {
	Status string              `json:"status"`
	Total  int                 `json:"total"`
	Data   []TeacherAPIPayload `json:"data"`
}

type TeacherAPIPayload struct {
	Nip  string `json:"nip"`
	Nama string `json:"nama"`
}

// ----------------------------------------------------------------------
// Sync Methods
// ----------------------------------------------------------------------

func (s *SyncService) SyncStudents(ctx context.Context) (int, error) {
	s.log.Info().Msg("Starting student synchronization from external API...")

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://siswa.smkn3singaraja.sch.id/api_siswa.php?token=dhanuitusangatgantengsekali", nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("api returned status: %d", resp.StatusCode)
	}

	var apiResp StudentAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if apiResp.Status != "success" {
		return 0, fmt.Errorf("api returned non-success status: %s", apiResp.Status)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	syncedCount := 0
	for _, data := range apiResp.Data {
		// Clean inputs
		nis := strings.TrimSpace(data.Nis)
		if nis == "" {
			continue // Skip invalid entries
		}
		nisn := strings.TrimSpace(data.Nisn)
		if nisn == "" {
			nisn = nis // Fallback if nisn is empty in source
		}
		name := strings.TrimSpace(data.Nama)
		gender := "L"
		if strings.ToUpper(strings.TrimSpace(data.Kelamin)) == "P" {
			gender = "P"
		}
		religion := strings.TrimSpace(data.Agama)
		if religion == "" {
			religion = "-"
		}

		gradeLevel := strings.TrimSpace(data.Kelas)
		majorCode := strings.TrimSpace(data.Jurusan)
		if gradeLevel == "" || majorCode == "" {
			s.log.Warn().Str("nis", nis).Msg("Skipping student with empty grade or major")
			continue
		}

		// Handle Kelompok (Group Number)
		kelompokStr := strings.TrimSpace(data.Kelompok)
		if kelompokStr == "" {
			kelompokStr = "1"
		}
		groupNumber, err := strconv.Atoi(kelompokStr)
		if err != nil || groupNumber <= 0 {
			groupNumber = 1
		}

		// Generate random secure password for potentially new students
		password, err := helper.GenerateStudentPassword()
		if err != nil {
			password = "123456" // Defensive fallback
		}

		// 1. Ensure Major Exists
		var majorID int
		err = tx.QueryRow(ctx, "SELECT id FROM majors WHERE code = $1", majorCode).Scan(&majorID)
		if err != nil {
			if err == pgx.ErrNoRows {
				// Create Major
				err = tx.QueryRow(ctx, `
					INSERT INTO majors (code, long_name) VALUES ($1, $2) RETURNING id
				`, majorCode, majorCode).Scan(&majorID)
				if err != nil {
					s.log.Error().Err(err).Str("major_code", majorCode).Msg("Failed to auto-create major")
					continue
				}
				s.log.Info().Str("major_code", majorCode).Msg("Auto-created missing major")
			} else {
				s.log.Error().Err(err).Msg("Database error querying majors")
				continue
			}
		}

		// 2. Ensure Class Exists
		var classID int
		err = tx.QueryRow(ctx, `
			SELECT id FROM classes WHERE grade_level = $1 AND major_code = $2 AND group_number = $3
		`, gradeLevel, majorCode, groupNumber).Scan(&classID)
		
		if err != nil {
			if err == pgx.ErrNoRows {
				// Create Class
				err = tx.QueryRow(ctx, `
					INSERT INTO classes (grade_level, major_code, group_number) 
					VALUES ($1, $2, $3) RETURNING id
				`, gradeLevel, majorCode, groupNumber).Scan(&classID)
				if err != nil {
					s.log.Error().Err(err).Str("nis", nis).Msg("Failed to auto-create class")
					continue
				}
				s.log.Info().Str("grade", gradeLevel).Str("major", majorCode).Int("group", groupNumber).Msg("Auto-created missing class")
			} else {
				s.log.Error().Err(err).Msg("Database error querying classes")
				continue
			}
		}

		// 3. Upsert Student
		_, err = tx.Exec(ctx, `
			INSERT INTO students (nis, nisn, name, gender, religion, password, class_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (nis) DO UPDATE SET
				nisn = EXCLUDED.nisn,
				name = EXCLUDED.name,
				gender = EXCLUDED.gender,
				religion = EXCLUDED.religion,
				class_id = EXCLUDED.class_id,
				updated_at = NOW()
		`, nis, nisn, name, gender, religion, password, classID)

		if err != nil {
			s.log.Error().Err(err).Str("nis", nis).Msg("Failed to upsert student")
			continue
		}
		syncedCount++
	}

	// 4. Cleanup abandoned classes
	s.log.Info().Msg("Executing class garbage collection...")
	tag, err := tx.Exec(ctx, `
		DELETE FROM classes c
		WHERE NOT EXISTS (
			SELECT 1 FROM students s WHERE s.class_id = c.id
		)
	`)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to clean up abandoned classes")
	} else {
		s.log.Info().Int64("deleted_classes", tag.RowsAffected()).Msg("Cleaned abandoned classes")
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Info().Int("synced_students", syncedCount).Msg("Student synchronization completed successfully")
	return syncedCount, nil
}

func (s *SyncService) SyncTeachers(ctx context.Context) (int, error) {
	s.log.Info().Msg("Starting teacher synchronization from external API...")

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://guru.smkn3singaraja.sch.id/api_guru.php?token=dhanusangatgantengsekali", nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("api returned status: %d", resp.StatusCode)
	}

	var apiResp TeacherAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if apiResp.Status != "success" {
		return 0, fmt.Errorf("api returned non-success status: %s", apiResp.Status)
	}

	// Fetch Role ID 2 (Guru/Teacher) prior to triggering transaction to prevent Postgres Poisoning if unavailable
	var guruRoleID int
	err = s.pool.QueryRow(ctx, "SELECT id FROM roles WHERE name = 'Teacher'").Scan(&guruRoleID)
	if err != nil {
		guruRoleID = 2 
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	syncedCount := 0
	for _, data := range apiResp.Data {
		name := strings.TrimSpace(data.Nama)
		nip := strings.TrimSpace(data.Nip)
		
		username := nip
		if username == "" {
			slug := strings.ToLower(strings.ReplaceAll(name, " ", "."))
			if len(slug) > 50 {
				slug = slug[:50]
			}
			username = slug
		}
		
		if username == "" {
			continue // Skip if ultimately unidentifiable
		}

		// Calculate password based on NIP
		password := "12345678"
		if len(nip) >= 8 {
			password = nip[:8]
		} else if len(nip) > 0 {
			password = nip // fallback to raw NIP if shorter than 8 chars
		}

		hashedPassword, err := s.authService.HashPassword(password)
		if err != nil {
			hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			hashedPassword = string(hash)
		}

		email := username + "@guru.smkn3singaraja.sch.id" // Fallback email generation

		// Wrap the UPSERT in a savepoint to prevent a single failure from poisoning the entire tx
		_, errSavepoint := tx.Exec(ctx, "SAVEPOINT sync_teacher")
		if errSavepoint != nil {
			s.log.Error().Err(errSavepoint).Msg("Failed to create savepoint")
			continue
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO admins (username, email, name, password_hash, role_id)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (username) DO UPDATE SET
				name = EXCLUDED.name,
				updated_at = NOW()
		`, username, email, name, hashedPassword, guruRoleID)

		if err != nil {
			tx.Exec(ctx, "ROLLBACK TO SAVEPOINT sync_teacher")
			s.log.Error().Err(err).Str("username", username).Msg("Failed to upsert teacher")
			continue
		}
		
		tx.Exec(ctx, "RELEASE SAVEPOINT sync_teacher")
		syncedCount++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Info().Int("synced_teachers", syncedCount).Msg("Teacher synchronization completed successfully")
	return syncedCount, nil
}
