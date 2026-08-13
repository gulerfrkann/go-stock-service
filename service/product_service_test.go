package service

import (
	"context"
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