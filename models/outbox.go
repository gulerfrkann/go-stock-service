package models

import "time"

type OutboxEvent struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	AggregateType string     `gorm:"size:50;not null" json:"aggregate_type"` // Örn: "product"
	AggregateID   string     `gorm:"size:100;not null" json:"aggregate_id"`  // Örn: "ORD-DOCKER-101"
	EventType     string     `gorm:"size:50;not null" json:"event_type"`     // Örn: "StockReserved"
	Payload       string     `gorm:"type:text;not null" json:"payload"`       // JSON içeriği
	Status        string     `gorm:"size:20;default:'PENDING'" json:"status"` // PENDING, PROCESSED, FAILED
	CreatedAt     time.Time  `json:"created_at"`
	ProcessedAt   *time.Time `json:"processed_at,omitempty"`
}