package routes

import (
	"stok-servisi/handlers"
	"github.com/gofiber/fiber/v2"
	amqp "github.com/rabbitmq/amqp091-go"
)

func SetupPaymentRoutes(router fiber.Router, amqpConn *amqp.Connection) {
	paymentHandler := handlers.NewPaymentHandler(amqpConn)
	router.Post("/payment/process", paymentHandler.ProcessPayment)
}