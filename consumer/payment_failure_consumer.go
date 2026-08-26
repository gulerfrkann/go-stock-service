package consumer

import (
	"context"
	"encoding/json"
	"go.uber.org/zap"
	"stok-servisi/config"
	"gorm.io/gorm"

	amqp "github.com/rabbitmq/amqp091-go"
)

// JSON'dan okuyacağımız veri yapısı
type PaymentFailedEvent struct {
	OrderID   string `json:"order_id"`
	ProductID int    `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Reason    string `json:"reason"`
}

type PaymentFailureConsumer struct {
	conn *amqp.Connection
	db   *gorm.DB // Veritabanı yeteneğini ekledik
}

// Artık başlatırken DB bağlantısını da istiyoruz
func NewPaymentFailureConsumer(conn *amqp.Connection, db *gorm.DB) *PaymentFailureConsumer {
	return &PaymentFailureConsumer{
		conn: conn,
		db:   db,
	}
}

func (c *PaymentFailureConsumer) Start(ctx context.Context) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}

	err = ch.ExchangeDeclare("saga_exchange", "topic", true, false, false, false, nil)
	if err != nil { return err }

	q, err := ch.QueueDeclare("stock_compensation_queue", true, false, false, false, nil)
	if err != nil { return err }

	err = ch.QueueBind(q.Name, "payment.failed", "saga_exchange", false, nil)
	if err != nil { return err }

	msgs, err := ch.Consume(q.Name, "payment_failure_worker", false, false, false, false, nil)
	if err != nil { return err }

	config.Logger.Info("🛡️ SAGA Pattern Aktif: Ödeme Hata (Compensation) kuyruğu dinleniyor...")

	go func() {
		for {
			select {
			case <-ctx.Done():
				ch.Close()
				return
			case msg := <-msgs:
				// 1. Gelen mesajı JSON'dan Struct'a çevir
				var event PaymentFailedEvent
				if err := json.Unmarshal(msg.Body, &event); err != nil {
					config.Logger.Error("SAGA JSON format hatası", zap.Error(err))
					msg.Nack(false, false) // Hatalı mesajı çöpe at
					continue
				}

				config.Logger.Warn("🔴 SAGA: Ödeme hatası yakalandı! Veritabanı iadesi başlatılıyor...", 
					zap.String("order_id", event.OrderID),
					zap.Int("product_id", event.ProductID),
				)

				// 2. Veritabanında stoğu geri ekle (Telafi İşlemi)
				// GORM kullanarak raw SQL ile stoğu artırıyoruz
				result := c.db.Exec("UPDATE products SET stock = stock + ? WHERE id = ?", event.Quantity, event.ProductID)
				
				if result.Error != nil {
					config.Logger.Error("SAGA Kritik Hata: Stok iade edilemedi!", zap.Error(result.Error))
					msg.Nack(false, true) // Hata olduysa mesajı sıraya geri koy, tekrar denesin
					continue
				}

				// 3. Başarı logu ve onay
				config.Logger.Info("✅ SAGA Başarılı: Stok fiziksel olarak iade edildi", 
					zap.Int("product_id", event.ProductID),
					zap.Int("iade_edilen_adet", event.Quantity),
				)
				
				msg.Ack(false)
			}
		}
	}()

	return nil
}