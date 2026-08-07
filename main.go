package main

import (
	"log"

	"stok-servisi/config"
	"stok-servisi/handlers"
	"stok-servisi/repository"
	"stok-servisi/service"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// 1. Veritabanı Bağlantısı
	db := config.ConnectDB()

	// 2. Katmanların Bağlanması (Dependency Injection)
	productRepo := repository.NewProductRepository(db)
	productService := service.NewProductService(productRepo)
	productHandler := handlers.NewProductHandler(productService)

	// 3. Fiber Sunucusu ve Rotalar
	app := fiber.New()
	api := app.Group("/api/v1")

	api.Get("/products", productHandler.GetAllProducts)
	api.Post("/products", productHandler.CreateProduct)
	api.Post("/products/:id/reduce-stock", productHandler.ReduceStock)

	log.Fatal(app.Listen(":8080"))
}