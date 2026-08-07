package handlers

import (
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
		return c.Status(500).JSON(fiber.Map{"hata": "Ürünler getirilemedi"})
	}
	return c.JSON(products)
}

func (h *ProductHandler) CreateProduct(c *fiber.Ctx) error {
	product := new(models.Product)
	if err := c.BodyParser(product); err != nil {
		return c.Status(400).JSON(fiber.Map{"hata": "Geçersiz istek gövdesi"})
	}

	if err := h.service.CreateProduct(product); err != nil {
		return c.Status(500).JSON(fiber.Map{"hata": "Ürün kaydedilemedi"})
	}

	return c.Status(201).JSON(product)
}

func (h *ProductHandler) ReduceStock(c *fiber.Ctx) error {
	id := c.Params("id")
	var req models.ReduceStockRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"hata": "Geçersiz istek"})
	}

	product, err := h.service.ReduceStock(id, req.Adet)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"hata": err.Error()})
	}

	return c.JSON(fiber.Map{
		"mesaj": "Stok başarıyla güncellendi",
		"urun":  product,
	})
}