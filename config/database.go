package config

import (
	"fmt"
	"log"
	"os"

	"stok-servisi/models"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnectDB() *gorm.DB {
	// Sadece DB_HOST önceden set edilmediyse .env dosyasını yükle (Lokal geliştirme için)
	if os.Getenv("DB_HOST") == "" {
		_ = godotenv.Load()
	}

	// Docker-compose'dan gelen DB_HOST varsa onu kullanır, yoksa fallback olarak "postgres-db" alır.
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "postgres-db"
	}

	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}

	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "postgres"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "stok_db"
	}

	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}

	sslMode := os.Getenv("DB_SSLMODE")
	if sslMode == "" {
		sslMode = "disable"
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		host, user, password, dbName, port, sslMode)

	log.Printf("Veritabanına bağlanılıyor: host=%s port=%s user=%s dbname=%s", host, port, user, dbName)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Veritabanına bağlanılamadı: %v", err)
	}

	log.Println("PostgreSQL veritabanı bağlantısı başarılı!")

	err = db.AutoMigrate(&models.Product{})
	if err != nil {
		log.Fatalf("AutoMigration hatası: %v", err)
	}

	return db
}