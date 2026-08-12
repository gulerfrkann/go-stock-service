package routes

import (
	"stok-servisi/handlers"

	"github.com/gofiber/fiber/v2"
)

func SetupHealthRoutes(app *fiber.App, healthHandler *handlers.HealthHandler) {
	health := app.Group("/healthz")

	health.Get("/live", healthHandler.Liveness)
	health.Get("/ready", healthHandler.Readiness)
}