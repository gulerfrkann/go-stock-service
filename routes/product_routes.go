package routes

import (
	"stok-servisi/handlers"

	"github.com/gofiber/fiber/v2"
)

func SetupProductRoutes(api fiber.Router, handler *handlers.ProductHandler) {
	products := api.Group("/products")

	products.Post("/", handler.CreateProduct)
	products.Get("/", handler.GetProducts)
	products.Get("/:id", handler.GetProductByID)           // GET isteği için zorunlu rota
	products.Post("/reserve-stock", handler.ReserveStock)
	products.Post("/:id/upload-image", handler.UploadImage)
}