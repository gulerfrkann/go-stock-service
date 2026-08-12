package handlers

import (
	"fmt"
	"math"
	"os"
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

func (h *ProductHandler) GetProducts(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	search := c.Query("search", "")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	products, total, err := h.service.GetProducts(page, limit, search)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Ürünler getirilemedi"})
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

func (h *ProductHandler) GetProductByID(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Geçersiz ürün ID'si"})
	}

	product, err := h.service.GetProductByID(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Ürün bulunamadı"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    product,
	})
}

func (h *ProductHandler) CreateProduct(c *fiber.Ctx) error {
	product := new(models.Product)
	if err := c.BodyParser(product); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Geçersiz istek gövdesi"})
	}

	if err := h.service.CreateProduct(product); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Ürün kaydedilemedi"})
	}

	return c.Status(fiber.StatusCreated).JSON(product)
}

type ReduceStockRequest struct {
	Quantity int `json:"quantity"`
}

func (h *ProductHandler) ReduceStock(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Geçersiz ürün ID'si"})
	}

	var req ReduceStockRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Geçersiz istek gövdesi"})
	}

	if req.Quantity <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Düşülecek miktar 0'dan büyük olmalıdır"})
	}

	remainingStock, err := h.service.ReduceStock(c.Context(), uint(id), req.Quantity)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":         true,
		"message":         "Stok başarıyla düşürüldü",
		"product_id":      id,
		"remaining_stock": remainingStock,
	})
}

// ReserveStock Idempotent Stok Rezervasyon Endpoint'i
func (h *ProductHandler) ReserveStock(c *fiber.Ctx) error {
	var req models.ReserveStockRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Geçersiz istek gövdesi",
		})
	}

	// İstek Doğrulama (Validation)
	if req.ProductID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Geçerli bir product_id belirtilmelidir",
		})
	}

	if req.Quantity <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Rezerve edilecek miktar 0'dan büyük olmalıdır",
		})
	}

	if req.OrderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Idempotency garantisi için order_id zorunludur",
		})
	}

	remainingStock, err := h.service.ReserveStock(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":         true,
		"message":         "Stok başarıyla rezerve edildi",
		"product_id":      req.ProductID,
		"order_id":        req.OrderID,
		"remaining_stock": remainingStock,
	})
}

// UploadImage Ürün Görseli Yükleme ve AI Outbox Tetikleme Ucu
func (h *ProductHandler) UploadImage(c *fiber.Ctx) error {
	idParam := c.Params("id")
	productID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Geçersiz ürün ID'si",
		})
	}

	file, err := c.FormFile("image")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Görsel dosyası zorunludur ('image' formu)",
		})
	}

	uploadDir := "./public/uploads"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Dizin oluşturulamadı",
		})
	}

	filename := fmt.Sprintf("%d_%s", productID, file.Filename)
	filePath := fmt.Sprintf("%s/%s", uploadDir, filename)

	if err := c.SaveFile(file, filePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Dosya kaydedilemedi",
		})
	}

	imageURL := fmt.Sprintf("/uploads/%s", filename)
	if err := h.service.UploadProductImage(uint(productID), imageURL, filePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Görsel başarıyla yüklendi, AI kataloglama olayı Outbox'a aktarıldı",
		"data": fiber.Map{
			"product_id": productID,
			"image_url":  imageURL,
		},
	})
}