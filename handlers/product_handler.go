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

// GetProducts Ürünleri Listeleme Endpoint'i
// @Summary Sayfalanmış ve filtrelenmiş ürün listesini döner
// @Description Sayfa numarası, limit ve arama parametrelerine göre ürünleri listeler.
// @Tags Products
// @Produce json
// @Param page query int false "Sayfa numarası (Varsayılan: 1)"
// @Param limit query int false "Sayfa başına ürün sayısı (Varsayılan: 20)"
// @Param search query string false "Arama kelimesi"
// @Success 200 {object} map[string]interface{} "Ürün listesi ve sayfalama bilgisi"
// @Failure 500 {object} map[string]interface{} "Sunucu hatası"
// @Router /api/products [get]
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

// GetProductByID ID ile Ürün Getirme Endpoint'i
// @Summary Belirtilen ID'ye sahip ürün detayını getirir
// @Description Ürün ID parametresi ile veritabanından ürün bilgilerini çeker.
// @Tags Products
// @Produce json
// @Param id path int true "Ürün ID"
// @Success 200 {object} map[string]interface{} "Ürün detayı"
// @Failure 400 {object} map[string]interface{} "Geçersiz ürün ID"
// @Failure 404 {object} map[string]interface{} "Ürün bulunamadı"
// @Router /api/products/{id} [get]
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

// CreateProduct Yeni Ürün Ekleme Endpoint'i
// @Summary Sisteme yeni ürün kaydeder
// @Description JSON body ile gelen ürün bilgilerini veritabanına ekler.
// @Tags Products
// @Accept json
// @Produce json
// @Param request body models.Product true "Ürün Bilgileri"
// @Success 201 {object} models.Product "Oluşturulan ürün"
// @Failure 400 {object} map[string]interface{} "Geçersiz istek gövdesi"
// @Failure 500 {object} map[string]interface{} "Kayıt hatası"
// @Router /api/products [post]
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

// ReduceStock Doğrudan Stok Düşürme Endpoint'i
// @Summary Belirtilen ürünün stok miktarını doğrudan düşürür
// @Description Redis Lua Script kullanarak yarış koşullarını (race conditions) engeller ve stok düşer.
// @Tags Stock
// @Accept json
// @Produce json
// @Param id path int true "Ürün ID"
// @Param request body ReduceStockRequest true "Düşülecek miktar"
// @Success 200 {object} map[string]interface{} "Başarılı stok düşüşü"
// @Failure 400 {object} map[string]interface{} "Yetersiz stok veya geçersiz istek"
// @Router /api/products/{id}/reduce-stock [post]
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
// @Summary Idempotent stok rezervasyonu yapar ve Outbox event tetikler
// @Description Gelen order_id ile mükerrer kontrolü (idempotency) yaparak stok rezerve eder ve pazar yeri sync için outbox kaydı oluşturur.
// @Tags Stock
// @Accept json
// @Produce json
// @Param request body models.ReserveStockRequest true "Rezervasyon İstek Parametreleri"
// @Success 200 {object} map[string]interface{} "Başarılı rezervasyon"
// @Failure 400 {object} map[string]interface{} "Geçersiz istek, mükerrer işlem veya yetersiz stok"
// @Router /api/stock/reserve [post]
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
// @Summary Ürüne görsel yükler ve AI Outbox olayı tetikler
// @Description Multipart form üzerinden yüklenen görseli sunucuya kaydeder ve ProductImageUploaded olayı oluşturur.
// @Tags Media
// @Accept multipart/form
// @Produce json
// @Param id path int true "Ürün ID"
// @Param image formData file true "Yüklenecek görsel dosyası"
// @Success 200 {object} map[string]interface{} "Görsel başarıyla yüklendi"
// @Failure 400 {object} map[string]interface{} "Geçersiz ID veya eksik dosya"
// @Failure 500 {object} map[string]interface{} "Kayıt hatası"
// @Router /api/products/{id}/image [post]
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

// GetRecommendations Ürün Tavsiyeleri ve Skorlama Ucu (AI & Cold Start Destekli)
// @Summary Ürüne özel TF-IDF ve benzerlik tavsiyelerini getirir
// @Description Redis önbelleğinden veya izole kategori benzerlik matrisinden ilgili ürün önerilerini döner.
// @Tags Recommendations
// @Produce json
// @Param id path int true "Ürün ID"
// @Success 200 {object} map[string]interface{} "Tavsiye listesi"
// @Failure 404 {object} map[string]interface{} "Ürün bulunamadı"
// @Router /api/products/{id}/recommendations [get]
func (h *ProductHandler) GetRecommendations(c *fiber.Ctx) error {
	idParam := c.Params("id")
	productID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Geçersiz ürün ID'si",
		})
	}

	recommendations, err := h.service.GetRecommendations(c.Context(), uint(productID))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":         true,
		"product_id":      productID,
		"recommendations": recommendations,
	})
}