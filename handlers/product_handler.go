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
