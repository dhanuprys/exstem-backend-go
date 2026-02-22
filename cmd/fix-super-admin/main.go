package main

import (
	"context"
	"fmt"

	"github.com/stemsi/exstem-backend/internal/config"
	"github.com/stemsi/exstem-backend/internal/database"
	"github.com/stemsi/exstem-backend/internal/logger"
	"github.com/stemsi/exstem-backend/internal/repository"
)

func main() {
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
	roleRepo := repository.NewRoleRepository(pool)

	fmt.Println("=== Fix Super Admin Permissions ===")
	fmt.Println("This command will assign ALL available permissions to Role ID 1 (Superadmin).")

	// 1. Get all permission codes from the database
	rows, err := pool.Query(ctx, "SELECT code FROM permissions")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to query permissions")
	}
	defer rows.Close()

	var allPermissions []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			log.Fatal().Err(err).Msg("Failed to scan permission code")
		}
		allPermissions = append(allPermissions, code)
	}

	if err := rows.Err(); err != nil {
		log.Fatal().Err(err).Msg("Error iterating over permissions")
	}

	if len(allPermissions) == 0 {
		fmt.Println("Error: No permissions found in the database. Ensure migrations have run.")
		return
	}

	fmt.Printf("Found %d permissions in the database.\n", len(allPermissions))

	// 2. Clear existing permissions for Role ID 1
	if err := roleRepo.DeleteAllPermissionsFromRole(ctx, 1); err != nil {
		log.Fatal().Err(err).Msg("Failed to clear existing permissions for Role ID 1")
	}

	// 3. Assign all permissions to Role ID 1
	if err := roleRepo.AssignPermissionsToRole(ctx, 1, allPermissions); err != nil {
		log.Fatal().Err(err).Msg("Failed to assign permissions to Role ID 1")
	}

	fmt.Println("\nSuccess! Superadmin (Role ID 1) now has full access, including newly added permissions.")
}
