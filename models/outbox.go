package models

import "time"

// Outbox Durum Sabitleri
const (
	OutboxStatusPending   = "PENDING"
	OutboxStatusProcessed = "PROCESSED"
	OutboxStatusFailed    = "FAILED"
)

// Olay Tipi Sabitleri
const (
	EventTypeStockReserved      = "StockReserved"
	EventTypeCriticalStockAlert = "CriticalStockAlert"
	EventTypeImageUploaded      = "ProductImageUploaded"
)

type OutboxEvent struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	AggregateType string     `gorm:"size:50;not null" json:"aggregate_type"`         // Örn: "product"
	AggregateID   string     `gorm:"size:100;not null" json:"aggregate_id"`          // Örn: "1" veya "ORD-101"
	EventType     string     `gorm:"size:50;not null" json:"event_type"`             // Örn: "CriticalStockAlert"
	Payload       string     `gorm:"type:text;not null" json:"payload"`               // JSON içeriği
	Status        string     `gorm:"size:20;default:'PENDING';index" json:"status"`  // Sorgu performansı için index eklendi
	CreatedAt     time.Time  `gorm:"autoCreateTime;index" json:"created_at"`
	ProcessedAt   *time.Time `json:"processed_at,omitempty"`
}

// TableName tablonun veritabanındaki ismini sabitler
func (OutboxEvent) TableName() string {
	return "outbox_events"
}