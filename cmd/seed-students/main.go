package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stemsi/exstem-backend/internal/config"
	"github.com/stemsi/exstem-backend/internal/database"
	"github.com/stemsi/exstem-backend/internal/logger"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
	"github.com/stemsi/exstem-backend/internal/service"
)

func main() {
	cfg := config.Load()
	log := logger.Setup(cfg.LogLevel, cfg.LogFormat)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	pool, err := database.NewPostgresPool(ctx, cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer pool.Close()

	classRepo := repository.NewClassRepository(pool)
	studentRepo := repository.NewStudentRepository(pool)

	classService := service.NewClassService(classRepo)
	studentService := service.NewStudentService(studentRepo)

	const totalStudents = 3000

	fmt.Printf("=== Seeding %d Students ===\n", totalStudents)

	classesToCreate := []struct {
		Level string
		Major string
		Group int
	}{
		{"XII", "TKJ", 1},
		{"XII", "TKJ", 2},
		{"XII", "TKJ", 3},
		{"X", "DKV", 1},
		{"X", "DKV", 2},
		{"X", "DKV", 3},
		{"XI", "TE", 1},
		{"XI", "TE", 2},
		{"XII", "TSM", 1},
		{"XII", "TSM", 2},
	}

	var classIDs []int

	for _, c := range classesToCreate {
		var existingClass model.Class
		err = pool.QueryRow(ctx, "SELECT id FROM classes WHERE grade_level = $1 AND major_code = $2 AND group_number = $3", c.Level, c.Major, c.Group).Scan(&existingClass.ID)

		if err != nil {
			if err == pgx.ErrNoRows {
				fmt.Printf("Class %s %s %d not found. Creating it...\n", c.Level, c.Major, c.Group)
				newClass := &model.Class{
					GradeLevel:  c.Level,
					MajorCode:   c.Major,
					GroupNumber: c.Group,
				}
				if err := classService.Create(ctx, newClass); err != nil {
					log.Fatal().Err(err).Msg("Failed to create class")
				}
				classIDs = append(classIDs, newClass.ID)
			} else {
				log.Fatal().Err(err).Msg("Failed to check existing class")
			}
		} else {
			classIDs = append(classIDs, existingClass.ID)
		}
	}

	// 50 base first names and 14 last names = 700 unique combinations
	firstNames := []string{
		"Budi", "Siti", "Andi", "Rina", "Joko",
		"Ayu", "Dodi", "Eka", "Fahri", "Gita",
		"Hendra", "Ika", "Jamal", "Kiki", "Lukman",
		"Maya", "Nanda", "Oki", "Putri", "Qori",
		"Rafi", "Siska", "Toni", "Umi", "Vina",
		"Wahyu", "Xena", "Yudi", "Zaki", "Alifia",
		"Bagas", "Citra", "Dimas", "Elisa", "Fikri",
		"Gali", "Hani", "Iqbal", "Jasmine", "Kevin",
		"Laras", "Miko", "Nia", "Oscar", "Puput",
		"Reza", "Sari", "Tigor", "Utari", "Vicky",
	}

	lastNames := []string{
		"Santoso", "Aminah", "Pratama", "Wati", "Susilo",
		"Lestari", "Kusuma", "Savitri", "Gunawan", "Hakim",
		"Septiana", "Maharani", "Saraswati", "Hidayat",
	}

	successCount := 0
	for i := 0; i < totalStudents; i++ {
		nisn := fmt.Sprintf("user%d", i+1)
		nis := fmt.Sprintf("%05d", i+1)
		name := fmt.Sprintf("%s %s", firstNames[i%len(firstNames)], lastNames[i%len(lastNames)])

		student := &model.Student{
			NIS:      nis,
			NISN:     nisn,
			Name:     name,
			Gender:   "Laki-laki",
			Religion: "Islam",
			Password: "stemsijaya",
			ClassID:  classIDs[i%len(classIDs)],
		}

		if i%2 != 0 {
			student.Gender = "Perempuan"
		}

		err := studentService.Create(ctx, student)
		if err != nil {
			fmt.Printf("Error creating student %s (NISN: %s): %v\n", student.Name, student.NISN, err)
		} else {
			successCount++
			if (i+1)%100 == 0 {
				fmt.Printf("Created %d students...\n", i+1)
			}
		}
	}

	fmt.Printf("\nSeed completed! Successfully added %d/%d students.\n", successCount, totalStudents)
}
