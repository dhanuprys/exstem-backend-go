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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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

	fmt.Println("=== Seeding 50 Students ===")

	gradeLevel := "XII"
	majorCode := "TKJ"
	groupNumber := 2

	// Check if class exists
	var classID int

	// Fast way to find the class
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

	names := []string{
		"Budi Santoso", "Siti Aminah", "Andi Pratama", "Rina Wati", "Joko Susilo",
		"Ayu Lestari", "Dodi Kusuma", "Eka Putri", "Fahri Hamzah", "Gita Savitri",
		"Hendra Gunawan", "Ika Sari", "Jamal Mirdad", "Kiki Fatmala", "Lukman Hakim",
		"Maya Septiana", "Nanda Pratama", "Oki Setiana", "Putri Dian", "Qori Maharani",
		"Rafi Ahmad", "Siska Saraswati", "Toni Setiawan", "Umi Kalsum", "Vina Panduwinata",
		"Wahyu Hidayat", "Xena Maharani", "Yudi Pratama", "Zaki Anwar", "Alifia Zahra",
		"Bagas Saputra", "Citra Kirana", "Dimas Anggara", "Elisa Novita", "Fikri Maulana",
		"Gali Rakasiwi", "Hani Hanifah", "Iqbal Ramadhan", "Jasmine Azzahra", "Kevin Sanjaya",
		"Larasati Dewi", "Miko Pambudi", "Nia Ramadhani", "Oscar Lawalata", "Puput Melati",
		"Reza Rahadian", "Sari Nila", "Tigor Siahaan", "Utari Maharani", "Vicky Prasetyo",
	}

	successCount := 0
	for i := 0; i < 50; i++ {
		nisn := fmt.Sprintf("user%d", i+1)
		nis := fmt.Sprintf("%05d", i+1)

		student := &model.Student{
			NIS:          nis,
			NISN:         nisn,
			Name:         names[i],
			Gender:       "Laki-laki", // Default for seed
			Religion:     "Hindu",     // Default for seed
			PasswordHash: "stemsijaya",
			ClassID:      classID,
		}

		// If i % 2 == 1, assign as Perempuan
		if i%2 != 0 {
			student.Gender = "Perempuan"
		}

		err := studentService.Create(ctx, student)
		if err != nil {
			fmt.Printf("Error creating student %s (NISN: %s): %v\n", student.Name, student.NISN, err)
		} else {
			successCount++
			if (i+1)%10 == 0 {
				fmt.Printf("Created %d students...\n", i+1)
			}
		}
	}

	fmt.Printf("\nSeed completed! Successfully added %d/50 students.\n", successCount)
}
