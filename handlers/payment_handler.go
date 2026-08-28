package handlers

import (
	"encoding/json"
	"stok-servisi/models"

	"github.com/gofiber/fiber/v2"
	amqp "github.com/rabbitmq/amqp091-go"
)

type PaymentHandler struct {
	amqpConn *amqp.Connection
}

func NewPaymentHandler(amqpConn *amqp.Connection) *PaymentHandler {
	return &PaymentHandler{amqpConn: amqpConn}
}

// ProcessPayment godoc
// @Summary Simüle edilmiş ödeme işlemi ve Saga tetikleyicisi
// @Description Gelen isteğe göre ödemeyi gerçekleştirir. Eğer should_fail true ise ödemeyi reddeder ve Saga pattern ile stok telafisini (rollback) tetikler.
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

	// Eğer ödeme başarısız simüle edildiyse (Saga Rollback Senaryosu)
	if req.ShouldFail {
		ch, err := h.amqpConn.Channel()
		if err == nil {
			defer ch.Close()

			eventData, _ := json.Marshal(fiber.Map{
				"order_id":   req.OrderID,
				"product_id": req.ProductID,
				"quantity":   req.Quantity,
				"reason":     "Simüle edilmiş ödeme reddi / Yetersiz bakiye",
			})

			ch.Publish(
				"stock_exchange",
				"payment.failed",
				false,
				false,
				amqp.Publishing{
					ContentType: "application/json",
					Body:        eventData,
				},
			)
		}

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":   "FAILED",
			"message":  "Ödeme başarısız oldu! Saga Pattern tetiklendi ve stok iade süreci başlatıldı.",
			"order_id": req.OrderID,
		})
	}

	// Ödeme Başarılı Senaryosu
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":   "SUCCESS",
		"message":  "Ödeme başarıyla tamamlandı.",
		"order_id": req.OrderID,
		"amount":   req.Amount,
	})
}