package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
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
	ReserveStock(ctx context.Context, req models.ReserveStockRequest) (int64, error) // Yeni eklendi
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

// ReserveStock Stok Rezervasyon İş Mantığı
func (s *productService) ReserveStock(ctx context.Context, req models.ReserveStockRequest) (int64, error) {
	if req.ExpirationSecs == 0 {
		req.ExpirationSecs = 900 // Süre verilmediyse varsayılan 15 dakika (900 sn)
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

	// Hata kodlarının kontrolü
	switch result {
	case -1:
		return 0, errors.New("bu sipariş id'si ile daha önce zaten stok rezerve edilmiş (idempotent)")
	case -2:
		return 0, errors.New("stok bilgisi senkronize edilemedi, lütfen tekrar deneyin")
	case -3:
		return 0, errors.New("yetersiz stok! rezervasyon başarısız")
	}

	// Redis tarafı başarılı; veritabanında da atomik düşüm gerçekleştiriliyor
	err = s.repo.ReserveStock(req.ProductID, req.Quantity)
	if err != nil {
		// DB hatası durumunda Redis cache'i temizleniyor ve rezervasyon kaydı siliniyor
		s.redisClient.Del(ctx, stockKey)
		s.redisClient.Del(ctx, orderKey)
		return 0, errors.New("veritabanında stok rezerve edilirken hata oluştu, işlem geri alındı")
	}

	return result, nil
}