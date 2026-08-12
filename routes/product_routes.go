package routes

import (
	"stok-servisi/handlers"

	"github.com/gofiber/fiber/v2"
)

func SetupProductRoutes(api fiber.Router, handler *handlers.ProductHandler) {
	products := api.Group("/products")

	products.Get("/", handler.GetProducts)
	products.Post("/", handler.CreateProduct)
	products.Post("/reserve-stock", handler.ReserveStock) // /api/v1/products/reserve-stock
	products.Post("/:id/reduce-stock", handler.ReduceStock)
}