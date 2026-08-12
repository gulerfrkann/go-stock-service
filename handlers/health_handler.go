package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewHealthHandler(db *gorm.DB, redis *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, redis: redis}
}

func (h *HealthHandler) Liveness(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "UP"})
}

func (h *HealthHandler) Readiness(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.PingContext(ctx) != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "DOWN",
			"reason": "Database connection failed",
		})
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "DOWN",
			"reason": "Redis connection failed",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":   "UP",
		"database": "OK",
		"redis":    "OK",
	})
}