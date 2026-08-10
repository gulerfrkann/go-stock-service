package main

import (
	"log"
	"os"

	"stok-servisi/config"
	"stok-servisi/handlers"
	"stok-servisi/repository"
	"stok-servisi/routes"
	"stok-servisi/service"

	"github.com/gofiber/fiber/v2"
)

// @title Stok Servisi API
// @version 1.0
// @description Go Fiber ve GORM ile geliştirilmiş stok yönetimi mikroservis API dokümantasyonu.
// @host localhost:8080
// @BasePath /api/v1
func main() {
	// Veritabanı ve Redis Bağlantıları
	db := config.ConnectDB()
	rdb := config.ConnectRedis() // Redis bağlantısını açıyoruz

	// Bağımlılıkların Oluşturulması (Dependency Injection)
	productRepo := repository.NewProductRepository(db)
	productService := service.NewProductService(productRepo, rdb) // rdb servise aktarıldı
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