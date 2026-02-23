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

	gradeLevel := "XII"
	majorCode := "TKJ"
	groupNumber := 2

	// Check if class exists
	var classID int

	var existingClass model.Class
	err = pool.QueryRow(ctx, "SELECT id, grade_level, major_code, group_number FROM classes WHERE grade_level = $1 AND major_code = $2 AND group_number = $3", gradeLevel, majorCode, groupNumber).Scan(&existingClass.ID, &existingClass.GradeLevel, &existingClass.MajorCode, &existingClass.GroupNumber)

	if err != nil {
		if err == pgx.ErrNoRows {
			fmt.Println("Class XII TKJ 2 not found. Creating it...")
			newClass := &model.Class{
				GradeLevel:  gradeLevel,
				MajorCode:   majorCode,
				GroupNumber: groupNumber,
			}
			if err := classService.Create(ctx, newClass); err != nil {
				log.Fatal().Err(err).Msg("Failed to create class")
			}
			classID = newClass.ID
			fmt.Printf("Created class with ID: %d\n", classID)
		} else {
			log.Fatal().Err(err).Msg("Failed to check existing class")
		}
	} else {
		classID = existingClass.ID
		fmt.Printf("Found existing class with ID: %d\n", classID)
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
			NIS:          nis,
			NISN:         nisn,
			Name:         name,
			Gender:       "Laki-laki",
			Religion:     "Islam",
			PasswordHash: "stemsijaya",
			ClassID:      classID,
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
