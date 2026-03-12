package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/stemsi/exstem-backend/internal/config"
	"github.com/stemsi/exstem-backend/internal/database"
	"github.com/stemsi/exstem-backend/internal/logger"
	"github.com/stemsi/exstem-backend/internal/service"
)

func main() {
	// ─── CLI Flags ──────────────────────────────────────────────────────
	syncType := flag.String("type", "", "Specify the type of synchronization: 'students', 'teachers', or 'all'")
	flag.Parse()

	if *syncType == "" {
		fmt.Println("Error: The -type flag is required.")
		fmt.Println("Usage: sync-stemsi -type=[students|teachers|all]")
		os.Exit(1)
	}

	// ─── Load Configuration ────────────────────────────────────────────
	cfg := config.Load()

	// ─── Initialize Logger ─────────────────────────────────────────────
	log := logger.Setup(cfg.LogLevel, cfg.LogFormat)

	ctx := context.Background()

	// ─── Connect to PostgreSQL ─────────────────────────────────────────
	pool, err := database.NewPostgresPool(ctx, cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer pool.Close()

	// ─── Initialize Service ────────────────────────────────────────────
	// We need AuthService to inject into SyncService to handle default password hashing
	authService := service.NewAuthService(cfg, nil) // Redis is not strictly necessary for simple hashing
	syncService := service.NewSyncService(pool, authService, log)

	fmt.Printf("=== Starting Data Synchronization (Type: %s) ===\n", *syncType)

	// ─── Execute Synchronization ───────────────────────────────────────
	if *syncType == "students" || *syncType == "all" {
		count, err := syncService.SyncStudents(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to synchronize students")
		} else {
			fmt.Printf("Successfully synchronized %d students.\n", count)
		}
	}

	if *syncType == "teachers" || *syncType == "all" {
		count, err := syncService.SyncTeachers(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to synchronize teachers")
		} else {
			fmt.Printf("Successfully synchronized %d teachers.\n", count)
		}
	}

	if *syncType != "students" && *syncType != "teachers" && *syncType != "all" {
		fmt.Printf("Error: Unsupported sync type '%s'. Use 'students', 'teachers', or 'all'.\n", *syncType)
		os.Exit(1)
	}

	fmt.Println("=== Synchronization Process Finished ===")
}
