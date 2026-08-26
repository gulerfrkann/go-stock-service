package models

import (
	"time"
)

// Product Veritabanı modeli
type Product struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	Price            float64 `json:"price"`
	Stock            int     `json:"stock"`
	ImageURL         string  `json:"image_url"`
	Category         string  `json:"category"`
	SEOTitle         string  `json:"seo_title"`
	CareInstructions string  `json:"care_instructions"`
	AICataloged      bool    `json:"ai_cataloged" gorm:"default:false"`
}

// ReduceStockRequest Doğrudan stok düşme isteği
type ReduceStockRequest struct {
	Quantity int `json:"quantity" validate:"required,gt=0"`
}

// ReserveStockRequest Stok rezervasyon isteği (Idempotent)
type ReserveStockRequest struct {
	ProductID        uint   `json:"product_id" validate:"required"`
	Quantity         int    `json:"quantity" validate:"required,gt=0"`
	OrderID          string `json:"order_id" validate:"required"`
	ExpirationSecs   int    `json:"expiration_secs,omitempty"` // Opsiyonel, gönderilmezse varsayılan süre kullanılır
}

// StockReservation Rezerve edilen stoğun detayları
type StockReservation struct {
	OrderID     string    `json:"order_id"`
	ProductID   uint      `json:"product_id"`
	Quantity    int       `json:"quantity"`
	CreatedAt   time.Time `json:"created_at"`
}

// ProductImageUploadedEvent Outbox üzerinden RabbitMQ'ya atılacak AI görsel işleme olayının içeriği
type ProductImageUploadedEvent struct {
	ProductID uint   `json:"product_id"`
	ImageURL  string `json:"image_url"`
	ImagePath string `json:"image_path"`
}