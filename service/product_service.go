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

// Önbellek için kullanacağımız sabitleştirilmiş anahtar adı
const productsCacheKey = "products_list"

type ProductService interface {
	GetAllProducts() ([]models.Product, error)
	CreateProduct(product *models.Product) error
	ReduceStock(id uint, req models.ReduceStockRequest) (*models.Product, error)
}

type productService struct {
	repo repository.ProductRepository
	rdb  *redis.Client // Redis istemcisi bağımlılığı eklendi
}

// NewProductService artık Redis istemcisini de parametre olarak alıyor
func NewProductService(repo repository.ProductRepository, rdb *redis.Client) ProductService {
	return &productService{
		repo: repo,
		rdb:  rdb,
	}
}

func (s *productService) GetAllProducts() ([]models.Product, error) {
	// 1. ADIM: Redis'te veri var mı kontrol et
	cachedData, err := s.rdb.Get(config.Ctx, productsCacheKey).Result()
	if err == nil {
		// CACHE HIT: Veri Redis'te bulundu!
		var products []models.Product
		// JSON string formatındaki veriyi Go struct yapısına çeviriyoruz (Unmarshal)
		if err := json.Unmarshal([]byte(cachedData), &products); err == nil {
			fmt.Println("🚀 VERİ REDIS CACHE'DEN GETİRİLDİ (0ms)")
			return products, nil
		}
	}

	// 2. ADIM (CACHE MISS): Veri Redis'te yoksa veya hata alındıysa PostgreSQL'den çek
	fmt.Println("🗄️ VERİ POSTGRESQL VERİTABANINDAN ÇEKİLİYOR...")
	products, err := s.repo.GetAll()
	if err != nil {
		return nil, err
	}

	// 3. ADIM: Veritabanından çektiğimiz veriyi JSON'a dönüştür ve Redis'e kaydet (10 dakika TTL)
	jsonData, err := json.Marshal(products)
	if err == nil {
		// Set komutu: (context, key, value, ttl)
		s.rdb.Set(config.Ctx, productsCacheKey, jsonData, 10*time.Minute)
	}

	return products, nil
}

func (s *productService) CreateProduct(product *models.Product) error {
	err := s.repo.Create(product)
	if err != nil {
		return err
	}

	// CACHE INVALIDATION: Yeni ürün eklendiği için Redis'teki eski listeyi siliyoruz
	s.rdb.Del(config.Ctx, productsCacheKey)
	fmt.Println("🧹 YENİ ÜRÜN EKLENDİ - REDIS CACHE TEMİZLENDİ")

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

	// CACHE INVALIDATION: Stok değiştiği için Redis'teki eski listeyi siliyoruz
	s.rdb.Del(config.Ctx, productsCacheKey)
	fmt.Println("🧹 STOK GÜNCELLENDİ - REDIS CACHE TEMİZLENDİ")

	return product, nil
}