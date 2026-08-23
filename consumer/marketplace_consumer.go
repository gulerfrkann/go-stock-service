package consumer

import (
	"context"
	"encoding/json"
	"log"

	"stok-servisi/adapter/marketplace"

	amqp "github.com/rabbitmq/amqp091-go"
)

type StockReservedPayload struct {
	ProductID      uint   `json:"product_id"`
	ReservedCount  int    `json:"reserved_count"`
	RemainingStock int    `json:"remaining_stock"`
	SourcePlatform string `json:"source_platform"`
}

type MarketplaceConsumer struct {
	channel     *amqp.Channel
	syncManager *marketplace.SyncManager
}

func NewMarketplaceConsumer(ch *amqp.Channel, sm *marketplace.SyncManager) *MarketplaceConsumer {
	return &MarketplaceConsumer{
		channel:     ch,
		syncManager: sm,
	}
}

func (c *MarketplaceConsumer) Start(ctx context.Context) error {
	queueName := "marketplace_sync_queue"

	q, err := c.channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,
	)
	if err != nil {
		return err
	}

	err = c.channel.QueueBind(
		q.Name,
		"stock.reserved",
		"stock_events",
		false,
		nil,
	)
	if err != nil {
		return err
	}

	msgs, err := c.channel.Consume(
		q.Name,
		"",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	log.Printf("🚀 [Marketplace Consumer] 'stock.reserved' olayları dinleniyor...")

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-msgs:
				if !ok {
					return
				}

				var payload StockReservedPayload
				if err := json.Unmarshal(d.Body, &payload); err != nil {
					log.Printf("❌ JSON Unmarshal hatası: %v", err)
					d.Nack(false, false)
					continue
				}

				log.Printf("📦 [Stok Rezervasyonu Yakalandı] Ürün ID: %d | Kalan: %d | Kaynak: %s",
					payload.ProductID, payload.RemainingStock, payload.SourcePlatform)

				c.syncManager.SyncAll(ctx, payload.ProductID, payload.RemainingStock, payload.SourcePlatform)

				d.Ack(false)
			}
		}
	}()

	return nil
}