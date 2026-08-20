package worker

import (
	"encoding/json"
	

	"stok-servisi/config"
	"stok-servisi/utils"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type CriticalStockPayload struct {
	ProductID   uint   `json:"product_id"`
	ProductName string `json:"product_name"`
	RemainStock int    `json:"remain_stock"`
}

// StartStockConsumer topic exchange'e bağlı kuyruğu ve DLQ mekanizmasını yönetir
func StartStockConsumer(amqpConn *amqp.Connection) error {
	ch, err := amqpConn.Channel()
	if err != nil {
		config.Logger.Error("RabbitMQ Consumer kanal açma hatası", zap.Error(err))
		return err
	}

	// 1. Ana Exchange Tanımı
	err = ch.ExchangeDeclare(
		"stock_events",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// 2. Dead Letter Exchange (DLX) Tanımı
	dlxExchange := "stock_events.dlx"
	err = ch.ExchangeDeclare(
		dlxExchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// 3. Dead Letter Queue (DLQ) Tanımı ve DLX'e Bağlanması
	dlqQueueName := "critical_stock_dlq"
	dlqRoutingKey := "stock.critical_alert.dlq"
	dlq, err := ch.QueueDeclare(
		dlqQueueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	err = ch.QueueBind(
		dlq.Name,
		dlqRoutingKey,
		dlxExchange,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// 4. Ana Kuyruk Tanımı (DLX argümanları ile birlikte)
	queueArgs := amqp.Table{
		"x-dead-letter-exchange":    dlxExchange,
		"x-dead-letter-routing-key": dlqRoutingKey,
	}

	mainQueue, err := ch.QueueDeclare(
		"critical_stock_queue",
		true,
		false,
		false,
		false,
		queueArgs,
	)
	if err != nil {
		return err
	}

	// Ana kuyruğu ana exchange'e bağla
	err = ch.QueueBind(
		mainQueue.Name,
		"stock.critical_alert",
		"stock_events",
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// 5. Ana Kuyruk Mesajlarını Dinleme (autoAck: false)
	msgs, err := ch.Consume(
		mainQueue.Name,
		"",
		false, // Manuel Ack/Nack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// 6. DLQ Kuyruğunu İzleme
	dlqMsgs, err := ch.Consume(
		dlq.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// Ana tüketici goroutine'i
	go func() {
		for d := range msgs {
			var payload CriticalStockPayload
			if err := json.Unmarshal(d.Body, &payload); err != nil {
				config.Logger.Error("Kritik stok mesajı çözülemedi, DLQ'ya yönlendiriliyor",
					zap.Error(err),
					zap.ByteString("raw_payload", d.Body),
				)
				_ = d.Nack(false, false)
				continue
			}

			// İş mantığı doğrulaması
			if payload.ProductID == 0 {
				config.Logger.Error("Geçersiz Product ID, mesaj DLQ'ya postalanıyor",
					zap.Uint("product_id", payload.ProductID),
				)
				_ = d.Nack(false, false)
				continue
			}

			// Başarılı loglama
			config.Logger.Warn("⚠️ [KRİTİK STOK UYARISI] Stok kritik seviyeye indi!",
				zap.Uint("product_id", payload.ProductID),
				zap.String("product_name", payload.ProductName),
				zap.Int("remain_stock", payload.RemainStock),
			)

			// Gerçek E-posta Gönderimi
			_ = utils.SendCriticalStockEmail(payload.ProductName, payload.RemainStock, payload.ProductID)

			// İşlem tamamlandı, mesajı onayla (Ack)
			_ = d.Ack(false)
		}
	}()

	// DLQ izleme goroutine'i
	go func() {
		for d := range dlqMsgs {
			config.Logger.Error("🚨 [DLQ - HATA] İşlenemeyen mesaj Dead Letter Queue'da yakalandı!",
				zap.ByteString("failed_payload", d.Body),
			)
		}
	}()

	config.Logger.Info("Stock Consumer ve DLQ başarıyla başlatıldı...")
	return nil
}