package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"stok-servisi/models"

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
	return "tx_resp_99887766", nil
}

type PaymentService struct {
	db        *gorm.DB
	amqpConn  *amqp.Connection
	gateway   PaymentGateway
}

func NewPaymentService(db *gorm.DB, amqpConn *amqp.Connection, gateway PaymentGateway) *PaymentService {
	return &PaymentService{
		db:        db,
		amqpConn:  amqpConn,
		gateway:   gateway,
	}
}

func (s *PaymentService) ProcessPayment(req models.PaymentRequest) (float64, error) {
	// 1. Önce veritabanından ürünü, stoğunu ve güncel BİRİM FİYATINI bulalım
	var product models.Product
	if err := s.db.First(&product, req.ProductID).Error; err != nil {
		return 0, errors.New("ödeme reddedildi: ürün bulunamadı")
	}

	// 2. Stok kontrolü
	if product.Stock < req.Quantity {
		return 0, errors.New("ödeme reddedildi: yetersiz stok (Mevcut: " + fmt.Sprintf("%d", product.Stock) + ")")
	}

	// 3. SUNUCU TARAFINDA GÜVENLİ TUTAR HESAPLAMA (Price Tampering Önlemi)
	secureAmount := product.Price * float64(req.Quantity)

	// 4. Ödeme Sağlayıcıya Güvenli Tutar ile İstek Atma
	_, err := s.gateway.ProcessCharge(secureAmount, "TRY", req.ShouldFail)
	if err != nil {
		// Ödeme başarısız oldu -> Saga Pattern tetikleniyor
		s.publishPaymentFailedEvent(req, err.Error())
		return 0, err
	}

	// 5. Başarılı Ödemeden Sonra Stoğu Düş ve Kaydet
	product.Stock -= req.Quantity
	if err := s.db.Save(&product).Error; err != nil {
		return 0, errors.New("stok güncellenirken veritabanı hatası oluştu")
	}

	log.Printf("Ödeme başarıyla onaylandı (Güvenli Tutar: %.2f TL). Sipariş ID: %s, Kalan Stok: %d", secureAmount, req.OrderID, product.Stock)
	return secureAmount, nil
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