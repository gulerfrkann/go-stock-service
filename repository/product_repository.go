package repository

import (
	"stok-servisi/models"

	"gorm.io/gorm"
)

type ProductRepository interface {
	Create(product *models.Product) error
	GetProducts(page, limit int, search string) ([]models.Product, int64, error)
	GetByID(id uint) (*models.Product, error)
	Update(product *models.Product) error
}

type productRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) Create(product *models.Product) error {
	return r.db.Create(product).Error
}

// GetProducts - Sayfalama (Pagination) ve GIN Trigram destekli ultra hızlı metin araması yapar
func (r *productRepository) GetProducts(page, limit int, search string) ([]models.Product, int64, error) {
	var products []models.Product
	var total int64

	query := r.db.Model(&models.Product{})

	// Arama filtresi varsa pg_trgm GIN indeksini tetikleyecek ILIKE sorgusu eklenir
	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	// Filtrelenmiş toplam kayıt sayısını hesapla
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sayfalama (Offset/Limit) ile sadece istenen sayfadaki kayıtları getir
	offset := (page - 1) * limit
	err := query.Offset(offset).Limit(limit).Order("id ASC").Find(&products).Error

	return products, total, err
}

func (r *productRepository) GetByID(id uint) (*models.Product, error) {
	var product models.Product
	err := r.db.First(&product, id).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *productRepository) Update(product *models.Product) error {
	return r.db.Save(product).Error
}