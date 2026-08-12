package routes

import (
    "stok-servisi/handlers"

    "github.com/gofiber/fiber/v2"
)

func SetupProductRoutes(api fiber.Router, productHandler *handlers.ProductHandler) {
    productGroup := api.Group("/products")

    productGroup.Get("/", productHandler.GetProducts)
    productGroup.Post("/", productHandler.CreateProduct)
    productGroup.Post("/:id/reduce", productHandler.ReduceStock)
    productGroup.Post("/:id/reduce-stock", productHandler.ReduceStock)
}
