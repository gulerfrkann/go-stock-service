package service

import (
	"testing"

	"stok-servisi/models"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- MOCK REPOSITORY ---
// Veritabanına gitmek yerine veritabanı gibi davranan sahte yapı.
type MockProductRepository struct {
	mock.Mock
}

func (m *MockProductRepository) GetAll() ([]models.Product, error) {
	args := m.Called()
	return args.Get(0).([]models.Product), args.Error(1)
}

func (m *MockProductRepository) Create(product *models.Product) error {
	args := m.Called(product)
	return args.Error(0)
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

// --- TEST 1: Yetersiz Stok Durumu Hata Dönecek Mi? ---
func TestReduceStock_InsufficientStock(t *testing.T) {
	// 1. Sahte Redis Başlat
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mockRepo := new(MockProductRepository)
	productService := NewProductService(mockRepo, rdb)

	// 2. Senaryo: Veritabanında stoğu 5 olan bir ürün bulunsun
	mockProduct := &models.Product{Stock: 5}
	mockProduct.ID = 1
	mockRepo.On("GetByID", uint(1)).Return(mockProduct, nil)

	// 3. Eylem: Kullanıcı 10 adet stok düşmek istesin (Stok yetersiz!)
	req := models.ReduceStockRequest{Quantity: 10}
	updatedProduct, err := productService.ReduceStock(1, req)

	// 4. Doğrulama (Assertions)
	assert.Error(t, err)                                     // Hata dönmeli
	assert.Nil(t, updatedProduct)                            // Ürün güncellenmemeli
	assert.Equal(t, "yetersiz stok! mevcut stok: 5", err.Error()) // Hata mesajı eşleşmeli
}

// --- TEST 2: Başarılı Stok Düşme ve Cache Temizliği ---
func TestReduceStock_Success(t *testing.T) {
	// 1. Sahte Redis Başlat
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mockRepo := new(MockProductRepository)
	productService := NewProductService(mockRepo, rdb)

	// 2. Senaryo: Stoğu 10 olan ürün bulunsun ve güncellemeye izin verilsin
	mockProduct := &models.Product{Stock: 10}
	mockProduct.ID = 1

	mockRepo.On("GetByID", uint(1)).Return(mockProduct, nil)
	mockRepo.On("Update", mock.Anything).Return(nil)

	// 3. Eylem: 3 adet stok düş
	req := models.ReduceStockRequest{Quantity: 3}
	updatedProduct, err := productService.ReduceStock(1, req)

	// 4. Doğrulama (Assertions)
	assert.NoError(t, err)                     // Hata olmamalı
	assert.NotNil(t, updatedProduct)           // Ürün dönmeli
	assert.Equal(t, 7, updatedProduct.Stock)
}