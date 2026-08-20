package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"stok-servisi/models"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- MOCK REPOSITORY ---
type MockProductRepository struct {
	mock.Mock
}

func (m *MockProductRepository) Create(product *models.Product) error {
	args := m.Called(product)
	return args.Error(0)
}

func (m *MockProductRepository) GetProducts(page, limit int, search string) ([]models.Product, int64, error) {
	args := m.Called(page, limit, search)
	return args.Get(0).([]models.Product), args.Get(1).(int64), args.Error(2)
}

func (m *MockProductRepository) GetByID(id uint) (*models.Product, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Product), args.Error(1)
}

func (m *MockProductRepository) Update(product *models.Product) error {
	args := m.Called(product)
	return args.Error(0)
}

func (m *MockProductRepository) UpdateStock(productID uint, newStock int) error {
	args := m.Called(productID, newStock)
	return args.Error(0)
}

func (m *MockProductRepository) ReserveStock(productID uint, quantity int) error {
	args := m.Called(productID, quantity)
	return args.Error(0)
}

func (m *MockProductRepository) ReserveStockWithOutbox(productID uint, quantity int, event *models.OutboxEvent) error {
	args := m.Called(productID, quantity, event)
	return args.Error(0)
}

func (m *MockProductRepository) SaveProductImageWithOutbox(productID uint, imageURL string, event *models.OutboxEvent) error {
	args := m.Called(productID, imageURL, event)
	return args.Error(0)
}

func (m *MockProductRepository) UpdateAICatalogData(productID uint, category, seoTitle, description, careInstructions string) error {
	args := m.Called(productID, category, seoTitle, description, careInstructions)
	return args.Error(0)
}

// --- TEST 1: Yetersiz Stok Durumu Hata Dönecek Mi? ---
func TestReduceStock_InsufficientStock(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mockRepo := new(MockProductRepository)
	productService := NewProductService(mockRepo, rdb)

	mockProduct := &models.Product{Stock: 5}
	mockProduct.ID = 1
	mockRepo.On("GetByID", uint(1)).Return(mockProduct, nil)

	newStock, err := productService.ReduceStock(context.Background(), 1, 10)

	assert.Error(t, err)
	assert.Equal(t, int64(0), newStock)
	assert.Equal(t, "yetersiz stok! işlem reddedildi", err.Error())
}

// --- TEST 2: Başarılı Stok Düşme ve Cache Temizliği ---
func TestReduceStock_Success(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mockRepo := new(MockProductRepository)
	productService := NewProductService(mockRepo, rdb)

	mockProduct := &models.Product{Stock: 10}
	mockProduct.ID = 1

	mockRepo.On("GetByID", uint(1)).Return(mockProduct, nil)
	mockRepo.On("UpdateStock", uint(1), 7).Return(nil)

	newStock, err := productService.ReduceStock(context.Background(), 1, 3)

	assert.NoError(t, err)
	assert.Equal(t, int64(7), newStock)
}

// --- TEST 3: Tavsiye Motoru - Redis Cache Hit Senaryosu ---
func TestGetRecommendations_CacheHit(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mockRepo := new(MockProductRepository)
	productService := NewProductService(mockRepo, rdb)

	targetID := uint(10)
	expectedRecs := []models.RecommendationResponse{
		{
			ProductID: 20,
			Name:      "Dell 30 Led Monitör",
			Category:  "Elektronik",
			Score:     0.95,
			Reason:    "Yapay Zeka (Kategori İçi NLP) Benzerliği",
		},
	}

	// Redis'e veri yazılır
	dataBytes, _ := json.Marshal(expectedRecs)
	redisKey := fmt.Sprintf("recommendations:product:%d", targetID)
	err = mr.Set(redisKey, string(dataBytes))
	assert.NoError(t, err)

	recs, err := productService.GetRecommendations(context.Background(), targetID)

	assert.NoError(t, err)
	assert.Len(t, recs, 1)
	assert.Equal(t, "Dell 30 Led Monitör", recs[0].Name)
	assert.Equal(t, 0.95, recs[0].Score)

	// Redis'te veri olduğu için DB sorgusu yapılmamalıdır
	mockRepo.AssertNotCalled(t, "GetByID", mock.Anything)
	mockRepo.AssertNotCalled(t, "GetProducts", mock.Anything, mock.Anything, mock.Anything)
}

// --- TEST 4: Tavsiye Motoru - Cache Miss & Go Fallback Senaryosu ---
func TestGetRecommendations_CacheMiss_Fallback(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mockRepo := new(MockProductRepository)
	productService := NewProductService(mockRepo, rdb)

	targetProduct := &models.Product{
		Name:     "Asus Zenbook 14X",
		Category: "Elektronik",
	}
	targetProduct.ID = 1

	categoryProducts := []models.Product{
		{Name: "Asus Zenbook 14X", Category: "Elektronik"},
		{Name: "Dell XPS 13", Category: "Elektronik"},
		{Name: "Lenovo ThinkPad", Category: "Elektronik"},
		{Name: "Roman Kitabı", Category: "Kitap"}, // Farklı kategori elenmeli
	}
	categoryProducts[0].ID = 1
	categoryProducts[1].ID = 2
	categoryProducts[2].ID = 3
	categoryProducts[3].ID = 4

	mockRepo.On("GetByID", uint(1)).Return(targetProduct, nil)
	mockRepo.On("GetProducts", 1, 50, "").Return(categoryProducts, int64(4), nil)

	recs, err := productService.GetRecommendations(context.Background(), 1)

	assert.NoError(t, err)
	assert.Len(t, recs, 2) // Kendisi ve kitap filtrelenir, sadece 2 elektronik ürün döner
	assert.Equal(t, uint(2), recs[0].ProductID)
	assert.Equal(t, uint(3), recs[1].ProductID)
	assert.Equal(t, "Aynı Kategori İçi Benzer Ürün", recs[0].Reason)
	mockRepo.AssertExpectations(t)
}

// --- TEST 5: Tavsiye Motoru - Ürün Bulunamadı Senaryosu ---
func TestGetRecommendations_ProductNotFound(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mockRepo := new(MockProductRepository)
	productService := NewProductService(mockRepo, rdb)

	mockRepo.On("GetByID", uint(999)).Return(nil, errors.New("product not found"))

	recs, err := productService.GetRecommendations(context.Background(), 999)

	assert.Error(t, err)
	assert.Nil(t, recs)
	assert.Equal(t, "product not found", err.Error())
	mockRepo.AssertExpectations(t)
}