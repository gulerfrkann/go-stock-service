package routes

import (
	"stok-servisi/handlers"

	"github.com/gofiber/fiber/v2"
)

func SetupPaymentRoutes(router fiber.Router, paymentHandler *handlers.PaymentHandler) {
	paymentGroup := router.Group("/payment")
	paymentGroup.Post("/process", paymentHandler.ProcessPayment)
}