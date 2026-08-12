package models

import (
	"time"

	"gorm.io/gorm"
)

// Product Veritabanı modeli
type Product struct {
	gorm.Model
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

// ReduceStockRequest Doğrudan stok düşme isteği
type ReduceStockRequest struct {
	Quantity int `json:"quantity" validate:"required,gt=0"`
}

// ReserveStockRequest Stok rezervasyon isteği (Idempotent)
type ReserveStockRequest struct {
	ProductID      uint   `json:"product_id" validate:"required"`
	Quantity       int    `json:"quantity" validate:"required,gt=0"`
	OrderID        string `json:"order_id" validate:"required"`
	ExpirationSecs int    `json:"expiration_secs,omitempty"` // Opsiyonel, gönderilmezse varsayılan süre kullanılır
}

// StockReservation Rezerve edilen stoğun detayları
type StockReservation struct {
	OrderID   string    `json:"order_id"`
	ProductID uint      `json:"product_id"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
}