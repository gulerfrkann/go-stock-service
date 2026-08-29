package service

import (
	"encoding/json"
	"errors"
	"log"
	"stok-servisi/models"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

// PaymentGateway - Gerçek bir sanal pos veya 3. parti servis için soyutlama (Adapter Pattern)
type PaymentGateway interface {
	ProcessCharge(amount float64, currency string, shouldFail bool) (string, error)
}

// DefaultPaymentGateway - Simüle edilmiş gerçekçi ödeme sağlayıcısı
type DefaultPaymentGateway struct{}

func (g *DefaultPaymentGateway) ProcessCharge(amount float64, currency string, shouldFail bool) (string, error) {
	if shouldFail {
		return "", errors.New("ödeme geçidi: yetersiz bakiye veya kart reddedildi")
	}
	// Başarılı işlemde benzersiz bir işlem (transaction) ID dönüyoruz
	return "tx_resp_99887766", nil
}

type PaymentService struct {
	db      *gorm.DB
	amqpConn *amqp.Connection
	gateway  PaymentGateway
}

func NewPaymentService(db *gorm.DB, amqpConn *amqp.Connection, gateway PaymentGateway) *PaymentService {
	return &PaymentService{
		db:      db,
		amqpConn: amqpConn,
		gateway:  gateway,
	}
}

func (s *PaymentService) ProcessPayment(req models.PaymentRequest) error {
	// 1. Önce veritabanından ürünü ve güncel stoğunu bulalım
	var product models.Product
	if err := s.db.First(&product, req.ProductID).Error; err != nil {
		return errors.New("ödeme reddedildi: ürün bulunamadı")
	}

	// 2. Stok kontrolü: Eğer talep edilen miktar stoktan fazlaysa ödemeyi reddet
	if product.Stock < req.Quantity {
		// Yetersiz stok durumunda da Saga telafisi tetiklenebilir veya doğrudan hata dönülebilir
		return errors.New("ödeme reddedildi: yetersiz stok (Mevcut: " + fmt.Sprintf("%d", product.Stock) + ")")
	}

	// 3. Ödeme Sağlayıcıya İstek Atma (Gateway Entegrasyonu)
	_, err := s.gateway.ProcessCharge(req.Amount, "TRY", req.ShouldFail)
	if err != nil {
		// Ödeme başarısız oldu -> Saga Pattern tetikleniyor
		s.publishPaymentFailedEvent(req, err.Error())
		return err
	}

	// 4. Başarılı Ödemeden Sonra Stoğu Düş ve Kaydet
	product.Stock -= req.Quantity
	if err := s.db.Save(&product).Error; err != nil {
		return errors.New("stok güncellenirken veritabanı hatası oluştu")
	}

	log.Printf("Ödeme başarıyla onaylandı ve stok güncellendi. Sipariş ID: %s, Kalan Stok: %d", req.OrderID, product.Stock)
	return nil
}

func (s *PaymentService) publishPaymentFailedEvent(req models.PaymentRequest, reason string) {
	ch, err := s.amqpConn.Channel()
	if err != nil {
		log.Printf("RabbitMQ kanal hatası: %v", err)
		return
	}
	defer ch.Close()

	eventData, _ := json.Marshal(map[string]interface{}{
		"order_id":   req.OrderID,
		"product_id": req.ProductID,
		"quantity":   req.Quantity,
		"reason":     reason,
	})

	ch.Publish(
		"stock_exchange",
		"payment.failed",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        eventData,
		},
	)
	log.Printf("Saga Telafi Olayı Yayınlandı (payment.failed) -> Sipariş: %s", req.OrderID)
}