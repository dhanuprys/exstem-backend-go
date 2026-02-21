package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/stemsi/exstem-backend/internal/config"
	"github.com/stemsi/exstem-backend/internal/database"
	"github.com/stemsi/exstem-backend/internal/logger"
	"github.com/stemsi/exstem-backend/internal/model"
	"github.com/stemsi/exstem-backend/internal/repository"
	"github.com/stemsi/exstem-backend/internal/service"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
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
	adminRepo := repository.NewAdminRepository(pool)
	roleRepo := repository.NewRoleRepository(pool)
	adminService := service.NewAdminService(adminRepo, roleRepo)

	// ─── CLI Input ─────────────────────────────────────────────────────
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== Create New Admin User ===")

	// Name
	fmt.Print("Enter Name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		fmt.Println("Error: Name is required")
		return
	}

	// Email
	fmt.Print("Enter Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)
	if email == "" {
		fmt.Println("Error: Email is required")
		return
	}

	// Password
	fmt.Print("Enter Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Println("\nError reading password")
		return
	}
	password := string(bytePassword)
	fmt.Println() // Newline after password input
	if len(password) < 6 {
		fmt.Println("Error: Password must be at least 6 characters")
		return
	}

	// Role ID
	fmt.Print("Enter Role ID (default 1): ")
	roleIDStr, _ := reader.ReadString('\n')
	roleIDStr = strings.TrimSpace(roleIDStr)
	roleID := 1
	if roleIDStr != "" {
		p, err := strconv.Atoi(roleIDStr)
		if err != nil {
			fmt.Println("Error: Role ID must be a number")
			return
		}
		roleID = p
	}

	// ─── Logic ─────────────────────────────────────────────────────────

	// Hash Password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), cfg.BcryptCost)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to hash password")
	}

	newAdmin := &model.Admin{
		Email:        email,
		Name:         name,
		PasswordHash: string(hashedPassword),
		RoleID:       roleID,
	}

	// Create Admin
	if err := adminService.Create(ctx, newAdmin); err != nil {
		log.Fatal().Err(err).Msg("Failed to create admin")
	}

	fmt.Printf("\nSuccess! Admin '%s' (%s) created with ID: %d\n", newAdmin.Name, newAdmin.Email, newAdmin.ID)
}
