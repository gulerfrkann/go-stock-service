package handlers

import (
	"stok-servisi/models"
	"stok-servisi/service"

	"github.com/gofiber/fiber/v2"
)

type PaymentHandler struct {
	paymentService *service.PaymentService
}

func NewPaymentHandler(paymentService *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentService: paymentService}
}

func (h *PaymentHandler) ProcessPayment(c *fiber.Ctx) error {
	var req models.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Geçersiz istek gövdesi",
		})
	}

	// Servis katmanından hem hata durumunu hem de hesaplanan güvenli tutarı alıyoruz
	securedAmount, err := h.paymentService.ProcessPayment(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":   "FAILED",
			"message":  "Ödeme başarısız oldu! Saga Pattern tetiklendi ve stok iade süreci başlatıldı.",
			"error":    err.Error(),
			"order_id": req.OrderID,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":     "SUCCESS",
		"message":    "Ödeme başarıyla tamamlandı ve sunucu tarafında fiyat doğrulandı.",
		"order_id":   req.OrderID,
		"paid_amount": securedAmount,
	})
}