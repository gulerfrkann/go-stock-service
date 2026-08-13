package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"stok-servisi/repository"

	"github.com/google/generative-ai-go/genai"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/api/option"
)

type apiKeyTransport struct {
	apiKey string
	wrapped http.RoundTripper
}

func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("x-goog-api-key", t.apiKey)
	q := req.URL.Query()
	q.Set("key", t.apiKey)
	req.URL.RawQuery = q.Encode()
	return t.wrapped.RoundTrip(req)
}

type AIWorker struct {
	amqpConn *amqp.Connection
	repo     repository.ProductRepository
}

type AIAnalysisResponse struct {
	Category         string `json:"category"`
	Description      string `json:"description"`
	CareInstructions string `json:"care_instructions"`
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
		log.Printf("AIWorker: RabbitMQ kanalı açılamadı: %v", err)
		return
	}

	err = ch.ExchangeDeclare("stock_events", "topic", true, false, false, false, nil)
	if err != nil { return }

	q, err := ch.QueueDeclare("stok_urun_kuyrugu", true, false, false, false, nil)
	if err != nil { return }

	err = ch.QueueBind(q.Name, "#", "stock_events", false, nil)
	if err != nil { return }

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil { return }

	log.Println("🤖 AIWorker başlatıldı, 'stock_events' Exchange'i üzerinden mesajlar bekleniyor...")

	go func() {
		for d := range msgs {
			var rawMap map[string]interface{}
			if err := json.Unmarshal(d.Body, &rawMap); err != nil { continue }

			var pID uint
			var imgURL string

			if v, ok := rawMap["product_id"]; ok {
				if floatVal, ok := v.(float64); ok { pID = uint(floatVal) }
			} else if v, ok := rawMap["ProductID"]; ok {
				if floatVal, ok := v.(float64); ok { pID = uint(floatVal) }
			}

			if v, ok := rawMap["image_url"].(string); ok {
				imgURL = v
			} else if v, ok := rawMap["ImageURL"].(string); ok {
				imgURL = v
			}

			if pID == 0 || imgURL == "" { continue }

			log.Printf("🎯 AIWorker: İşlenecek resim yakalandı! (ProductID: %d, ImageURL: %s)", pID, imgURL)
			w.processImageWithAI(pID, imgURL)
		}
	}()
}

func (w *AIWorker) processImageWithAI(productID uint, imageURL string) {
	ctx := context.Background()
	apiKey := os.Getenv("GEMINI_API_KEY")
	apiKey = strings.TrimSpace(strings.Trim(apiKey, `"`))

	customClient := &http.Client{
		Transport: &apiKeyTransport{
			apiKey:  apiKey,
			wrapped: http.DefaultTransport,
		},
	}

	client, err := genai.NewClient(ctx, option.WithHTTPClient(customClient), option.WithAPIKey(apiKey))
	if err != nil {
		log.Printf("AIWorker HATA: Gemini client oluşturulamadı: %v", err)
		return
	}
	defer client.Close()

	// 1. Google'ın sana izin verdiği TÜM modelleri bir listeye topla
	var supportedModels []string
	iter := client.ListModels(ctx)
	for {
		m, err := iter.Next()
		if err != nil { break }
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				supportedModels = append(supportedModels, strings.TrimPrefix(m.Name, "models/"))
				break
			}
		}
	}

	// Eğer API boş liste dönerse, garanti modelleri elle ekle
	if len(supportedModels) == 0 {
		supportedModels = []string{"gemini-1.5-flash", "gemini-1.5-pro", "gemini-1.0-pro"}
	}

	cleanPath := strings.TrimPrefix(imageURL, "/")
	filePath := filepath.Join("public", cleanPath)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		filePath = cleanPath
	}

	imgData, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("AIWorker HATA: Resim dosyası okunamadı: %v", err)
		return
	}

	mimeType := "image/jpeg"
	lowerPath := strings.ToLower(filePath)
	if strings.HasSuffix(lowerPath, ".png") { mimeType = "image/png" } else if strings.HasSuffix(lowerPath, ".webp") { mimeType = "image/webp" }

	prompt := genai.Text(`Bu e-ticaret ürün görselini dikkatlice analiz et.
Aşağıdaki JSON formatında yanıt ver:
{
  "category": "Ürünün ana kategorisi (Örn: Temizlik & Hijyen, Elektronik, Gıda vb.)",
  "description": "Ürün görselini özetleyen, müşteri odaklı 1-2 cümlelik Türkçe e-ticaret ürün açıklaması.",
  "care_instructions": "Ürünün kullanım, muhafaza veya güvenlik uyarısı (Örn: Kuru yerde saklayınız vb.)"
}`)
	imgPart := genai.ImageData(mimeType, imgData)

	var resp *genai.GenerateContentResponse
	var lastErr error
	var usedModel string

	// 2. Modelleri tek tek dene. Biri reddedilirse ANINDA diğerine geç!
	for _, modelName := range supportedModels {
		log.Printf("🚀 Deneniyor: %s...", modelName)
		model := client.GenerativeModel(modelName)
		model.SetTemperature(0.2)
		model.ResponseMIMEType = "application/json"

		resp, lastErr = model.GenerateContent(ctx, prompt, imgPart)
		if lastErr == nil && resp != nil && len(resp.Candidates) > 0 {
			usedModel = modelName
			break // Başarılı! Döngüden çık.
		}
		log.Printf("⚠️ %s reddedildi veya hata verdi. Listeden sıradakine geçiliyor...", modelName)
	}

	if lastErr != nil || usedModel == "" {
		log.Printf("❌ AIWorker HATA: Hiçbir model isteği kabul etmedi. Son hata: %v", lastErr)
		return
	}

	// Yanıtı işle
	var rawText string
	if txt, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
		rawText = string(txt)
	} else {
		rawText = fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	}

	rawText = strings.TrimPrefix(rawText, "```json")
	rawText = strings.TrimPrefix(rawText, "```")
	rawText = strings.TrimSuffix(rawText, "```")
	rawText = strings.TrimSpace(rawText)

	var aiRes AIAnalysisResponse
	if err := json.Unmarshal([]byte(rawText), &aiRes); err != nil {
		log.Printf("AIWorker HATA: JSON Parse hatası: %v", err)
		return
	}

	product, err := w.repo.GetByID(productID)
	if err != nil { return }

	product.Category = aiRes.Category
	product.Description = aiRes.Description
	product.CareInstructions = aiRes.CareInstructions
	product.AICataloged = true

	if err := w.repo.Update(product); err != nil { return }

	log.Printf("🎉 AI BAŞARIYLA KATALOGLADI! (Model: %s) Kategori: %s", usedModel, aiRes.Category)
}