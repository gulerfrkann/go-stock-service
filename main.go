package main

import (
	"log"
	"os"

	"stok-servisi/config"
	"stok-servisi/handlers"
	"stok-servisi/models" // Model paketini ekledik
	"stok-servisi/repository"
	"stok-servisi/routes"
	"stok-servisi/service"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// Veritabanı ve Redis Bağlantıları
	db := config.ConnectDB()
	rdb := config.ConnectRedis()

	// Veritabanı Tablo Migrasyonu (Tablo yoksa otomatik oluşturur)
	if err := db.AutoMigrate(&models.Product{}); err != nil {
		log.Fatalf("Veritabanı migrasyon hatası: %v", err)
	}

	// Bağımlılıkların Oluşturulması (Dependency Injection)
	productRepo := repository.NewProductRepository(db)
	productService := service.NewProductService(productRepo, rdb)
	productHandler := handlers.NewProductHandler(productService)

	app := fiber.New()
	app.Static("/", "./public")

	// API Versiyon Grubu
	api := app.Group("/api/v1")

	// Rotaların Tanımlanması
	routes.SetupProductRoutes(api, productHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Fatal(app.Listen(":" + port))
}