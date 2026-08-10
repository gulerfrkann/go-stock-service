package handlers

import (
	"math"
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

// GetProducts godoc
// @Summary      Ürünleri Listele ve Ara
// @Description  965k ürün arasında hızlı arama (GIN Trigram) ve sayfalama yapar.
// @Tags         Products
// @Produce      json
// @Param        page    query     int     false  "Sayfa Numarası (Varsayılan: 1)"
// @Param        limit   query     int     false  "Sayfa Başına Ürün Miktarı (Varsayılan: 20, Maks: 100)"
// @Param        search  query     string  false  "Ürün Adında Arama Terimi"
// @Success      200     {object}  map[string]interface{}
// @Failure      500     {object}  map[string]string
// @Router       /products [get]
func (h *ProductHandler) GetProducts(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	search := c.Query("search", "")

	// Güvenlik ve varsayılan değer sınırlandırmaları
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	products, total, err := h.service.GetProducts(page, limit, search)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Ürünler getirilemedi"})
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	return c.JSON(fiber.Map{
		"data":        products,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

// CreateProduct godoc
// @Summary      Yeni Ürün Oluştur
// @Description  Sisteme yeni bir ürün kaydeder ve ilgili Redis önbelleklerini temizler.
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        product body models.Product true "Ürün Bilgisi"
// @Success      201     {object} models.Product
// @Failure      400     {object} map[string]string
// @Failure      500     {object} map[string]string
// @Router       /products [post]
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
// @Summary      Stok Düşür
// @Description  Belirtilen ID'li ürünün stoğunu verilen miktar kadar azaltır.
// @Tags         Products
// @Accept       json
// @Produce      json
// @Param        id   path int                       true "Ürün ID"
// @Param        body body models.ReduceStockRequest true "Düşülecek Miktar"
// @Success      200  {object} map[string]interface{}
// @Failure      400  {object} map[string]string
// @Router       /products/{id}/reduce [post]
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