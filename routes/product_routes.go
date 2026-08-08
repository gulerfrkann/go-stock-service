package routes

import (
	"stok-servisi/handlers"

	_ "stok-servisi/docs" // Swag init komutunun üreteceği doküman paketi

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
)

func SetupProductRoutes(api fiber.Router, handler *handlers.ProductHandler) {
	// Swagger Arayüz Rotaları
	api.Get("/swagger/*", swagger.HandlerDefault)

	products := api.Group("/products")
	products.Get("/", handler.GetAllProducts)
	products.Post("/", handler.CreateProduct)
	products.Post("/:id/reduce", handler.ReduceStock)
}