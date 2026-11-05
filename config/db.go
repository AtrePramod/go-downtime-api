package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"go-downtime-api/models"
)

// DB is the shared database connection used by the whole app.
var DB *gorm.DB

// ConnectDB opens a connection to PostgreSQL using values from environment variables.
func ConnectDB() {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	// TranslateError turns Postgres error codes into GORM errors such as
	// gorm.ErrDuplicatedKey, so handlers can check them with errors.Is.
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// Tune the connection pool. Go's default keeps only 2 idle connections,
	// so code that runs several queries at once (like the concurrent daily
	// report) would open and log in to new connections on every request.
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(20)                  // never more than 20 connections at once
	sqlDB.SetMaxIdleConns(10)                  // keep up to 10 ready for reuse
	sqlDB.SetConnMaxLifetime(30 * time.Minute) // recycle old connections now and then

	DB = db
	log.Println("connected to PostgreSQL")
}

// Migrate creates or updates the tables for all models.
// It only adds missing tables, columns, indexes and constraints; it never drops anything.
func Migrate() {
	if err := DB.AutoMigrate(&models.Machine{}, &models.DowntimeLog{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}
	log.Println("database migrated")
}
