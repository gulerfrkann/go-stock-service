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

// GetAllProducts godoc
// @Summary Tüm ürünleri listele
// @Description Veritabanındaki tüm ürünlerin listesini getirir.
// @Tags Products
// @Produce json
// @Success 200 {array} models.Product
// @Failure 500 {object} map[string]string
// @Router /products [get]
func (h *ProductHandler) GetAllProducts(c *fiber.Ctx) error {
	products, err := h.service.GetAllProducts()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Ürünler getirilemedi"})
	}
	return c.JSON(products)
}

// CreateProduct godoc
// @Summary Yeni ürün oluştur
// @Description Sisteme yeni bir ürün kaydeder.
// @Tags Products
// @Accept json
// @Produce json
// @Param product body models.Product true "Ürün Bilgisi"
// @Success 201 {object} models.Product
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /products [post]
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

// ReduceStock godoc
// @Summary Stok düşür
// @Description Belirtilen ID'li ürünün stoğunu verilen miktar kadar azaltır.
// @Tags Products
// @Accept json
// @Produce json
// @Param id path int true "Ürün ID"
// @Param body body models.ReduceStockRequest true "Düşülecek Miktar"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /products/{id}/reduce [post]
func (h *ProductHandler) ReduceStock(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Geçersiz ürün ID'si"})
	}

	var req models.ReduceStockRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Geçersiz istek gövdesi"})
	}

	product, err := h.service.ReduceStock(uint(id), req)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "Stok başarıyla güncellendi",
		"product": product,
	})
}