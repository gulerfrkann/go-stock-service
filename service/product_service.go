package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"stok-servisi/config"
	"stok-servisi/models"
	"stok-servisi/repository"
)

// Doğrudan stok düşürme için Lua Script
var decreaseStockLuaScript = redis.NewScript(`
    local current_stock = redis.call('GET', KEYS[1])
    if not current_stock then
        return -2
    end
    if tonumber(current_stock) >= tonumber(ARGV[1]) then
        return redis.call('DECRBY', KEYS[1], ARGV[1])
    else
        return -1
    end
`)

// Idempotent Stok Rezervasyonu için Lua Script
var reserveStockLuaScript = redis.NewScript(`
    local order_key = KEYS[1]
    local stock_key = KEYS[2]
    local amount = tonumber(ARGV[1])
    local ttl = tonumber(ARGV[2])

    -- 1. Idempotency Kontrolü: Bu sipariş daha önce rezerve edildi mi?
    if redis.call('EXISTS', order_key) == 1 then
        return -1
    end

    -- 2. Stok Key Kontrolü
    local current_stock = redis.call('GET', stock_key)
    if not current_stock then
        return -2
    end

    -- 3. Stok Yeterlilik Kontrolü
    if tonumber(current_stock) < amount then
        return -3
    end

    -- 4. Stok Düşme ve Order Rezervasyon Key'ini TTL ile Kaydetme
    local new_stock = redis.call('DECRBY', stock_key, amount)
    redis.call('SET', order_key, amount, 'EX', ttl)

    return new_stock
`)

type ProductService interface {
	CreateProduct(product *models.Product) error
	GetProducts(page, limit int, search string) ([]models.Product, int64, error)
	GetProductByID(id uint) (*models.Product, error)
	UpdateProduct(product *models.Product) error
	ReduceStock(ctx context.Context, productID uint, quantity int) (int64, error)
	ReserveStock(ctx context.Context, req models.ReserveStockRequest) (int64, error)
	UploadProductImage(productID uint, imageURL, imagePath string) error
}

type productService struct {
	repo        repository.ProductRepository
	redisClient *redis.Client
}

func NewProductService(repo repository.ProductRepository, redisClient *redis.Client) ProductService {
	return &productService{
		repo:        repo,
		redisClient: redisClient,
	}
}

func (s *productService) CreateProduct(product *models.Product) error {
	return s.repo.Create(product)
}

func (s *productService) GetProducts(page, limit int, search string) ([]models.Product, int64, error) {
	return s.repo.GetProducts(page, limit, search)
}

func (s *productService) GetProductByID(id uint) (*models.Product, error) {
	return s.repo.GetByID(id)
}

func (s *productService) UpdateProduct(product *models.Product) error {
	return s.repo.Update(product)
}

func (s *productService) ReduceStock(ctx context.Context, productID uint, quantity int) (int64, error) {
	redisKey := fmt.Sprintf("stock:product:%d", productID)

	loadCache := func() error {
		product, err := s.repo.GetByID(productID)
		if err != nil {
			return errors.New("ürün bulunamadı")
		}
		s.redisClient.SetNX(ctx, redisKey, product.Stock, 1*time.Hour)
		return nil
	}

	exists, err := s.redisClient.Exists(ctx, redisKey).Result()
	if err != nil || exists == 0 {
		if err := loadCache(); err != nil {
			return 0, err
		}
	}

	keys := []string{redisKey}
	args := []interface{}{quantity}

	result, err := decreaseStockLuaScript.Run(ctx, s.redisClient, keys, args...).Int64()
	if err != nil {
		return 0, fmt.Errorf("redis script hatası: %v", err)
	}

	if result == -2 {
		if err := loadCache(); err != nil {
			return 0, err
		}
		result, err = decreaseStockLuaScript.Run(ctx, s.redisClient, keys, args...).Int64()
		if err != nil {
			return 0, fmt.Errorf("redis script hatası: %v", err)
		}
	}

	if result == -1 {
		return 0, errors.New("yetersiz stok! işlem reddedildi")
	}

	if result == -2 {
		return 0, errors.New("stok bilgisi senkronize edilemedi, lütfen tekrar deneyin")
	}

	err = s.repo.UpdateStock(productID, int(result))
	if err != nil {
		s.redisClient.Del(ctx, redisKey)
		return 0, errors.New("veritabanı güncellenirken hata oluştu, işlem geri alındı")
	}

	return result, nil
}

// ReserveStock Stok Rezervasyon İş Mantığı (Outbox & Zap Logger Entegreli)
func (s *productService) ReserveStock(ctx context.Context, req models.ReserveStockRequest) (int64, error) {
	correlationID, _ := ctx.Value("correlation_id").(string)

	if req.ExpirationSecs == 0 {
		req.ExpirationSecs = 900 // Varsayılan 15 dakika
	}

	orderKey := fmt.Sprintf("reservation:order:%s", req.OrderID)
	stockKey := fmt.Sprintf("stock:product:%d", req.ProductID)

	loadCache := func() error {
		product, err := s.repo.GetByID(req.ProductID)
		if err != nil {
			return errors.New("ürün bulunamadı")
		}
		s.redisClient.SetNX(ctx, stockKey, product.Stock, 1*time.Hour)
		return nil
	}

	// Cache kontrolü ve yükleme
	exists, err := s.redisClient.Exists(ctx, stockKey).Result()
	if err != nil || exists == 0 {
		if err := loadCache(); err != nil {
			return 0, err
		}
	}

	keys := []string{orderKey, stockKey}
	args := []interface{}{req.Quantity, req.ExpirationSecs}

	result, err := reserveStockLuaScript.Run(ctx, s.redisClient, keys, args...).Int64()
	if err != nil {
		config.Logger.Error("Redis script çalıştırma hatası",
			zap.Error(err),
			zap.String("correlation_id", correlationID),
		)
		return 0, fmt.Errorf("redis script hatası: %v", err)
	}

	// Cache miss (-2) durumunda 1 kez tazeleyip tekrar dene
	if result == -2 {
		if err := loadCache(); err != nil {
			return 0, err
		}
		result, err = reserveStockLuaScript.Run(ctx, s.redisClient, keys, args...).Int64()
		if err != nil {
			return 0, fmt.Errorf("redis script hatası: %v", err)
		}
	}

	// Hata kodlarının kontrolü & Loglanması
	switch result {
	case -1:
		config.Logger.Warn("Mükerrer rezervasyon isteği engellendi (Idempotent)",
			zap.String("correlation_id", correlationID),
			zap.String("order_id", req.OrderID),
			zap.Uint("product_id", req.ProductID),
		)
		return 0, errors.New("bu sipariş id'si ile daha önce zaten stok rezerve edilmiş (idempotent)")
	case -2:
		return 0, errors.New("stok bilgisi senkronize edilemedi, lütfen tekrar deneyin")
	case -3:
		config.Logger.Warn("Yetersiz stok rezervasyon reddedildi",
			zap.String("correlation_id", correlationID),
			zap.String("order_id", req.OrderID),
			zap.Uint("product_id", req.ProductID),
			zap.Int("requested_quantity", req.Quantity),
		)
		return 0, errors.New("yetersiz stok! rezervasyon başarısız")
	}

	// Redis tarafı başarılı; Veritabanı rezervasyonu + Outbox kaydı ekleniyor
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"product_id":      req.ProductID,
		"order_id":        req.OrderID,
		"reserved_qty":    req.Quantity,
		"remaining_stock": result,
		"timestamp":       time.Now(),
	})

	outboxEvent := &models.OutboxEvent{
		AggregateType: "product",
		AggregateID:   req.OrderID,
		EventType:     "StockReserved",
		Payload:       string(eventPayload),
		Status:        "PENDING",
	}

	err = s.repo.ReserveStockWithOutbox(req.ProductID, req.Quantity, outboxEvent)
	if err != nil {
		// DB hatası durumunda Redis cache'i ve rezervasyon kaydı geri alınıyor
		s.redisClient.Del(ctx, stockKey)
		s.redisClient.Del(ctx, orderKey)
		config.Logger.Error("Veritabanı stok rezervasyon hatası, Redis rollback yapıldı",
			zap.Error(err),
			zap.String("correlation_id", correlationID),
		)
		return 0, errors.New("veritabanında stok rezerve edilirken hata oluştu, işlem geri alındı")
	}

	config.Logger.Info("Stok başarıyla rezerve edildi ve Outbox kaydı oluşturuldu",
		zap.String("correlation_id", correlationID),
		zap.Uint("product_id", req.ProductID),
		zap.String("order_id", req.OrderID),
		zap.Int("reserved_quantity", req.Quantity),
		zap.Int64("remaining_stock", result),
	)

	return result, nil
}

// UploadProductImage Görsel yükleme ve ProductImageUploaded Outbox olayı oluşturma
func (s *productService) UploadProductImage(productID uint, imageURL, imagePath string) error {
	eventPayload := models.ProductImageUploadedEvent{
		ProductID: productID,
		ImageURL:  imageURL,
		ImagePath: imagePath,
	}

	payloadBytes, err := json.Marshal(eventPayload)
	if err != nil {
		return fmt.Errorf("event payload serileştirme hatası: %w", err)
	}

	outboxEvent := &models.OutboxEvent{
		AggregateType: "product",
		AggregateID:   fmt.Sprintf("PROD-IMG-%d", productID),
		EventType:     "ProductImageUploaded",
		Payload:       string(payloadBytes),
		Status:        "PENDING",
	}

	err = s.repo.SaveProductImageWithOutbox(productID, imageURL, outboxEvent)
	if err != nil {
		config.Logger.Error("Görsel kaydı ve Outbox oluşturma hatası",
			zap.Error(err),
			zap.Uint("product_id", productID),
		)
		return err
	}

	config.Logger.Info("Görsel başarıyla kaydedildi ve ProductImageUploaded Outbox kaydı oluşturuldu",
		zap.Uint("product_id", productID),
		zap.String("image_url", imageURL),
	)

	return nil
}