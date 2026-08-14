package models

// RecommendationResponse, bir ürün için kullanıcıya dönülecek tavsiye formatıdır.
type RecommendationResponse struct {
	ProductID uint    `json:"product_id"`
	Name      string  `json:"name"`
	Category  string  `json:"category"` // Kullanıcı arayüzünde göstermek için faydalıdır
	Score     float64 `json:"score"`    // Örn: 0.92 (%92 benzerlik)
	Reason    string  `json:"reason"`   // Örn: "İçerik Benzerliği (NLP)" veya "Birlikte Alınanlar (Apriori)"
}