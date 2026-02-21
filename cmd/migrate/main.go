package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stemsi/exstem-backend/internal/config"
)

func main() {
	var migrationDir string
	flag.StringVar(&migrationDir, "path", "migrations", "Path to migration files")
	flag.Parse()

	// Load config
	cfg := config.Load()
	dbURL := cfg.DatabaseURL
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	sourceURL := fmt.Sprintf("file://%s", migrationDir)

	m, err := migrate.New(sourceURL, dbURL)
	if err != nil {
		log.Fatalf("Migration failed to initialize: %v", err)
	}

	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		return
	}

	command := args[0]
	switch command {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Up failed: %v", err)
		}
		fmt.Println("Migrated up successfully")
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Down failed: %v", err)
		}
		fmt.Println("Migrated down successfully")
	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("Version failed: %v", err)
		}
		fmt.Printf("Version: %d, Dirty: %t\n", version, dirty)
	case "force":
		if len(args) < 2 {
			log.Fatal("force requires version argument")
		}
		v, err := strconv.Atoi(args[1])
		if err != nil {
			log.Fatalf("Invalid version: %v", err)
		}
		if err := m.Force(v); err != nil {
			log.Fatalf("Force failed: %v", err)
		}
		fmt.Printf("Forced version to %d\n", v)
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: migrate [flags] <command>")
	fmt.Println("Commands: up, down, version, force <version>")
	fmt.Println("Flags:")
	flag.PrintDefaults()
}
