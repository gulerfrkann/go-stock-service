package consumer

import (
	"context"
	"go.uber.org/zap"
	"stok-servisi/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

type PaymentFailureConsumer struct {
	conn *amqp.Connection
}

func NewPaymentFailureConsumer(conn *amqp.Connection) *PaymentFailureConsumer {
	return &PaymentFailureConsumer{
		conn: conn,
	}
}

func (c *PaymentFailureConsumer) Start(ctx context.Context) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return err
	}

	// 1. Mevcut sistemi bozmayan YENİ bir Exchange (Yönlendirici)
	err = ch.ExchangeDeclare(
		"saga_exchange", // Sadece Saga (Dağıtık işlem) olayları için
		"topic",
		true, false, false, false, nil,
	)
	if err != nil {
		return err
	}

	// 2. Ödeme hatalarının düşeceği YENİ stok telafi kuyruğu
	q, err := ch.QueueDeclare(
		"stock_compensation_queue",
		true, false, false, false, nil,
	)
	if err != nil {
		return err
	}

	// 3. Kuyruğu routing key ile bağla (Sadece payment.failed mesajlarını alacak)
	err = ch.QueueBind(
		q.Name,
		"payment.failed", // Ödeme servisi patladığında bu anahtarla mesaj atacak
		"saga_exchange",
		false, nil,
	)
	if err != nil {
		return err
	}

	// 4. Kuyruğu dinlemeye başla
	msgs, err := ch.Consume(
		q.Name,
		"payment_failure_worker", // Tüketici adı
		false, // auto-ack kapalı (İşlem bitmeden mesaj silinmez)
		false, false, false, nil,
	)
	if err != nil {
		return err
	}

	config.Logger.Info("🛡️ SAGA Pattern Aktif: Ödeme Hata (Compensation) kuyruğu dinleniyor...")

	go func() {
		for {
			select {
			case <-ctx.Done():
				ch.Close()
				return
			case msg := <-msgs:
				// Burada daha sonra veritabanına gidip düşülen stoğu +1 olarak geri ekleyeceğiz
				config.Logger.Warn("🔴 SAGA: Ödeme hatası yakalandı! Stok telafisi (iade) başlatılıyor...", 
					zap.String("routing_key", msg.RoutingKey),
					zap.ByteString("payload", msg.Body),
				)
				
				// İşlem başarıyla bittiğinde RabbitMQ'ya onayı ver
				msg.Ack(false)
			}
		}
	}()

	return nil
}