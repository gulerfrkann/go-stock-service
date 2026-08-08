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

func main() {
	db := config.ConnectDB()

	productRepo := repository.NewProductRepository(db)
	productService := service.NewProductService(productRepo)
	productHandler := handlers.NewProductHandler(productService)

	app := fiber.New()

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