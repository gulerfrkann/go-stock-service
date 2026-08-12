package worker

import (
	"context"
	"time"

	"stok-servisi/config"
	"stok-servisi/models"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type OutboxWorker struct {
	db   *gorm.DB
	amqp *amqp.Connection
}

func NewOutboxWorker(db *gorm.DB, amqpConn *amqp.Connection) *OutboxWorker {
	return &OutboxWorker{db: db, amqp: amqpConn}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	channel, err := w.amqp.Channel()
	if err != nil {
		config.Logger.Error("RabbitMQ kanal açma hatası", zap.Error(err))
		return
	}
	defer channel.Close()

	// Exchange Deklare Edilmesi
	err = channel.ExchangeDeclare(
		"stock_events", // exchange ismi
		"topic",        // tipi
		true,           // durable
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,
	)
	if err != nil {
		config.Logger.Error("RabbitMQ Exchange deklarasyon hatası", zap.Error(err))
		return
	}

	for {
		select {
		case <-ctx.Done():
			config.Logger.Info("Outbox Worker durduruluyor...")
			return
		case <-ticker.C:
			w.processPendingEvents(channel)
		}
	}
}

func (w *OutboxWorker) processPendingEvents(ch *amqp.Channel) {
	var events []models.OutboxEvent
	err := w.db.Where("status = ?", "PENDING").Order("created_at asc").Limit(20).Find(&events).Error
	if err != nil || len(events) == 0 {
		return
	}

	for _, event := range events {
		err := ch.Publish(
			"stock_events",   // exchange
			"stock.reserved", // routing key
			false,            // mandatory
			false,            // immediate
			amqp.Publishing{
				ContentType: "application/json",
				Body:        []byte(event.Payload),
			},
		)

		if err != nil {
			config.Logger.Error("RabbitMQ mesaj yayınlama hatası", zap.Error(err), zap.Uint("event_id", event.ID))
			continue
		}

		now := time.Now()
		w.db.Model(&event).Updates(models.OutboxEvent{
			Status:      "PROCESSED",
			ProcessedAt: &now,
		})

		config.Logger.Info("Outbox Olayı RabbitMQ'ya Yayınlandı",
			zap.Uint("event_id", event.ID),
			zap.String("event_type", event.EventType),
			zap.String("aggregate_id", event.AggregateID),
		)
	}
}