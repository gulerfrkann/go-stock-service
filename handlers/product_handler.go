package handlers

import (
	"strconv"

	"stok-servisi/models"
	"stok-servisi/service"

	"github.com/gofiber/fiber/v2"
)

type ProductHandler struct {
	service service.ProductService
}

func NewProductHandler(service service.ProductService) *ProductHandler {
	return &ProductHandler{service: service}
}

func (h *ProductHandler) GetAllProducts(c *fiber.Ctx) error {
	products, err := h.service.GetAllProducts()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Ürünler getirilemedi"})
	}
	return c.JSON(products)
}

func (h *ProductHandler) CreateProduct(c *fiber.Ctx) error {
	product := new(models.Product)
	if err := c.BodyParser(product); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Geçersiz istek gövdesi"})
	}

	if err := h.service.CreateProduct(product); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Ürün kaydedilemedi"})
	}

	return c.Status(201).JSON(product)
}

func (h *ProductHandler) ReduceStock(c *fiber.Ctx) error {
	// 1. URL'den gelen string ID'yi uint tipine dönüştürüyoruz
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Geçersiz ürün ID'si"})
	}

	var req models.ReduceStockRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Geçersiz istek gövdesi"})
	}

	// 2. uint(id) ve req struct'ının kendisini servise gönderiyoruz
	product, err := h.service.ReduceStock(uint(id), req)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "Stok başarıyla güncellendi",
		"product": product,
	})
}