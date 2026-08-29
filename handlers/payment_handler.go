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

// ProcessPayment godoc
// @Summary Katmanlı Mimari ve Gateway Entegrasyonlu Ödeme İşlemi
// @Description PaymentService ve PaymentGateway arayüzü üzerinden ödemeyi işler. Hata durumunda Saga rollback tetikler.
// @Tags Ödeme
// @Accept json
// @Produce json
// @Param request body models.PaymentRequest true "Ödeme Bilgileri"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/payment/process [post]
func (h *PaymentHandler) ProcessPayment(c *fiber.Ctx) error {
	var req models.PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Geçersiz istek gövdesi",
		})
	}

	// İş mantığını Service katmanına devrediyoruz
	err := h.paymentService.ProcessPayment(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":   "FAILED",
			"message":  "Ödeme başarısız oldu! Saga Pattern tetiklendi ve stok iade süreci başlatıldı.",
			"error":    err.Error(),
			"order_id": req.OrderID,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":   "SUCCESS",
		"message":  "Ödeme başarıyla tamamlandı ve onaylandı.",
		"order_id": req.OrderID,
		"amount":   req.Amount,
	})
}