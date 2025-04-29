package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"sort"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	log.SetPrefix("[example-api] ")
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)

	// Get database path from environment or use default
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/emails.db"
	}

	log.Printf("Running migrations on database: %s", dbPath)

	// Ensure the database directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// Open database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Get all migration files
	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		log.Fatalf("Failed to list migration files: %v", err)
	}

	// Sort migration files to ensure they run in order
	sort.Strings(files)

	// Execute each migration
	for _, file := range files {
		log.Printf("Running migration: %s", file)

		// Read migration file
		migration, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Failed to read migration file %s: %v", file, err)
		}

		// Execute migration
		if _, err := db.Exec(string(migration)); err != nil {
			log.Fatalf("Failed to execute migration %s: %v", file, err)
		}
	}

	log.Println("All migrations completed successfully")
}
