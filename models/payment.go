package models

// PaymentRequest Ödeme simülasyonu için gelen istek yapısı
type PaymentRequest struct {
	OrderID     string  `json:"order_id"`
	ProductID   uint    `json:"product_id"`
	Quantity    int     `json:"quantity"`
	
	ShouldFail  bool    `json:"should_fail"` // true yapılırsa Saga telafi mekanizması tetiklenir
}