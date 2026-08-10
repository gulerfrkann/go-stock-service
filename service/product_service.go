package service

import (
	"encoding/json"
	"fmt"
	"time"

	"stok-servisi/config"
	"stok-servisi/models"
	"stok-servisi/repository"

	"github.com/redis/go-redis/v9"
)

type ProductService interface {
	GetProducts(page, limit int, search string) ([]models.Product, int64, error)
	CreateProduct(product *models.Product) error
	ReduceStock(id uint, req models.ReduceStockRequest) (*models.Product, error)
}

type productService struct {
	repo repository.ProductRepository
	rdb  *redis.Client
}

// Önbellekte saklayacağımız verinin yapısı (Ürün listesi + Toplam satır sayısı)
type PaginatedProductsResult struct {
	Products []models.Product `json:"products"`
	Total    int64            `json:"total"`
}

func NewProductService(repo repository.ProductRepository, rdb *redis.Client) ProductService {
	return &productService{
		repo: repo,
		rdb:  rdb,
	}
}

// GetProducts - Sayfalama ve arama sorgusuna özel dinamik Redis Cache kontrolü yapar
func (s *productService) GetProducts(page, limit int, search string) ([]models.Product, int64, error) {
	// Her sayfa, limit ve arama terimi için benzersiz dinamik Cache Key
	cacheKey := fmt.Sprintf("products:page:%d:limit:%d:search:%s", page, limit, search)

	// 1. ADIM: Redis'te bu spesifik arama/sayfa kombinasyonu var mı kontrol et
	cachedData, err := s.rdb.Get(config.Ctx, cacheKey).Result()
	if err == nil {
		var result PaginatedProductsResult
		if err := json.Unmarshal([]byte(cachedData), &result); err == nil {
			fmt.Printf("🚀 VERİ REDIS CACHE'DEN GETİRİLDİ (0ms) -> Key: %s\n", cacheKey)
			return result.Products, result.Total, nil
		}
	}

	// 2. ADIM (CACHE MISS): Veri Redis'te yoksa PostgreSQL (GIN Index) üzerinden çek
	fmt.Println("🗄️ VERİ POSTGRESQL VERİTABANINDAN ÇEKİLİYOR...")
	products, total, err := s.repo.GetProducts(page, limit, search)
	if err != nil {
		return nil, 0, err
	}

	// 3. ADIM: Çekilen veriyi JSON yapısına çevir ve Redis'e 10 dakika TTL ile kaydet
	toCache := PaginatedProductsResult{
		Products: products,
		Total:    total,
	}
	if jsonData, err := json.Marshal(toCache); err == nil {
		s.rdb.Set(config.Ctx, cacheKey, jsonData, 10*time.Minute)
	}

	return products, total, nil
}

func (s *productService) CreateProduct(product *models.Product) error {
	err := s.repo.Create(product)
	if err != nil {
		return err
	}

	// CACHE INVALIDATION: Yeni ürün eklendiği için oluşturulmuş tüm ürün önbelleklerini temizle
	s.clearProductsCache()
	fmt.Println("🧹 YENİ ÜRÜN EKLENDİ - TÜM REDIS ÜRÜN CACHE'LERİ TEMİZLENDİ")

	return nil
}

func (s *productService) ReduceStock(id uint, req models.ReduceStockRequest) (*models.Product, error) {
	product, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("ürün bulunamadı")
	}

	if product.Stock < req.Quantity {
		return nil, fmt.Errorf("yetersiz stok! mevcut stok: %d", product.Stock)
	}

	product.Stock -= req.Quantity
	if err := s.repo.Update(product); err != nil {
		return nil, err
	}

	// CACHE INVALIDATION: Stok değiştiği için oluşturulmuş tüm ürün önbelleklerini temizle
	s.clearProductsCache()
	fmt.Println("🧹 STOK GÜNCELLENDİ - TÜM REDIS ÜRÜN CACHE'LERİ TEMİZLENDİ")

	return product, nil
}

// clearProductsCache - "products:*" kalıbına uyan tüm önbellek anahtarlarını siler
func (s *productService) clearProductsCache() {
	keys, err := s.rdb.Keys(config.Ctx, "products:*").Result()
	if err == nil && len(keys) > 0 {
		s.rdb.Del(config.Ctx, keys...)
	}
}