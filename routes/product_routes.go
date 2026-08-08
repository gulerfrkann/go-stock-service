package routes

import (
	"stok-servisi/handlers"

	"github.com/gofiber/fiber/v2"
)

func SetupProductRoutes(api fiber.Router, handler *handlers.ProductHandler) {
	products := api.Group("/products")

	products.Get("/", handler.GetAllProducts)
	products.Post("/", handler.CreateProduct)
	products.Post("/:id/reduce", handler.ReduceStock)
}