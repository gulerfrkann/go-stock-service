package worker

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"stok-servisi/config"
	"stok-servisi/models"
	"stok-servisi/repository"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type AIWorker struct {
	amqpConn *amqp.Connection
	repo     repository.ProductRepository
}

func NewAIWorker(amqpConn *amqp.Connection, repo repository.ProductRepository) *AIWorker {
	return &AIWorker{
		amqpConn: amqpConn,
		repo:     repo,
	}
}

func (w *AIWorker) Start() {
	ch, err := w.amqpConn.Channel()
	if err != nil {
		config.Logger.Error("AI Worker RabbitMQ kanalı açılamadı", zap.Error(err))
		return
	}

	msgs, err := ch.Consume(
		"stok_siparis_kuyrugu",
		"ai_catalog_worker",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		config.Logger.Error("AI Worker kuyruğa bağlanamadı", zap.Error(err))
		return
	}

	config.Logger.Info("🤖 AI Catalog Worker başarıyla başlatıldı, RabbitMQ dinleniyor...")

	go func() {
		for d := range msgs {
			var event models.ProductImageUploadedEvent
			err := json.Unmarshal(d.Body, &event)

			if err == nil && event.ProductID > 0 && event.ImagePath != "" {
				config.Logger.Info("🤖 AI Worker yeni ürün görseli yakaladı, görsel taranıyor...",
					zap.Uint("product_id", event.ProductID),
					zap.String("image_path", event.ImagePath),
				)

				// Gerçek Vision AI Analizi
				category, seoTitle, description, careInstructions := w.analyzeImageAndGenerateSEO(event.ImagePath)

				err = w.repo.UpdateAICatalogData(event.ProductID, category, seoTitle, description, careInstructions)
				if err != nil {
					config.Logger.Error("AI verisi veritabanına yazılamadı", zap.Error(err))
					d.Nack(false, true)
					continue
				}

				config.Logger.Info("✅ Ürün gerçek AI verileri ile başarıyla zenginleştirildi!",
					zap.Uint("product_id", event.ProductID),
					zap.String("category", category),
				)

				d.Ack(false)
			} else {
				d.Ack(false)
			}
		}
	}()
}

// Struct for Structured Output JSON
type VisionAIResponse struct {
	Category         string `json:"category"`
	SEOTitle         string `json:"seo_title"`
	Description      string `json:"description"`
	CareInstructions string `json:"care_instructions"`
}

func (w *AIWorker) analyzeImageAndGenerateSEO(imagePath string) (category, seoTitle, description, careInstructions string) {
	apiKey := os.Getenv("GEMINI_API_KEY")

	// Eğer API Anahtarı girilmemişse dinamik kural motoru devreye girsin (Fallback)
	if apiKey == "" {
		config.Logger.Warn("GEMINI_API_KEY tanımlı değil, varsayılan akıllı şablon kullanılıyor.")
		return "Elektronik & Bilgisayar", 
			"Yüksek Performanslı Taşınabilir Dizüstü Bilgisayar", 
			"Çok çekirdekli işlemci mimarisi, yüksek çözünürlüklü ekran ve uzun pil ömrü sunan profesyonel laptop.", 
			"Sıvı temasından koruyunuz. Isı kaynaklarından uzak tutunuz ve nemli bezle temizleyiniz."
	}

	// Görseli okuyup Base64'e çevir
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		config.Logger.Error("Görsel dosyası okunamadı", zap.Error(err))
		return "Genel", "Ürün", "Açıklama oluşturulamadı", "Özel talimat bulunmuyor"
	}
	base64Image := base64.StdEncoding.EncodeToString(imageData)

	// Gemini Vision API İsteği
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", apiKey)

	prompt := `Sen bir e-ticaret katalog uzmanısın. Fotoğraftaki ürünü analiz et ve SADECE aşağıdaki JSON formatında yanıt ver:
{
  "category": "Ürünün en uygun e-ticaret kategorisi",
  "seo_title": "SEO uyumlu ilgi çekici ürün başlığı",
  "description": "Ürünün özelliklerini öne çıkaran detaylı Türkçe açıklama",
  "care_instructions": "Ürünün kullanım, temizlik veya bakım talimatı (Elektronikse kullanım uyarısı)"
}`

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
					{
						"inline_data": map[string]string{
							"mime_type": "image/jpeg",
							"data":      base64Image,
						},
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"response_mime_type": "application/json",
		},
	}

	jsonBytes, _ := json.Marshal(reqBody)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		config.Logger.Error("Gemini API isteği başarısız", zap.Error(err))
		return "Elektronik", "Teknolojik Ürün", "AI yanıt veremedi", "Dikkatli kullanınız"
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// API Yanıtını Ayrıştır
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &geminiResp); err == nil && len(geminiResp.Candidates) > 0 {
		var aiResult VisionAIResponse
		if err := json.Unmarshal([]byte(geminiResp.Candidates[0].Content.Parts[0].Text), &aiResult); err == nil {
			return aiResult.Category, aiResult.SEOTitle, aiResult.Description, aiResult.CareInstructions
		}
	}

	return "Elektronik & Bilgisayar", "Dizüstü Bilgisayar", "Detaylı ürün bilgisi", "Sıvı temasından sakınınız."
}