package config

import (
	"log"

	"stok-servisi/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() *gorm.DB {
	dsn := "host=localhost user=postgres password=postgres dbname=stok_db port=5432 sslmode=disable"
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Veritabanına bağlanılamadı: %v", err)
	}

	log.Println("PostgreSQL veritabanı bağlantısı başarılı!")

	err = DB.AutoMigrate(&models.Product{})
	if err != nil {
		log.Fatalf("AutoMigration hatası: %v", err)
	}

	return DB
}