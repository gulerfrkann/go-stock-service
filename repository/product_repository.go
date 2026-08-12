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
	UpdateStock(id uint, newStock int) error
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

func (r *productRepository) GetProducts(page, limit int, search string) ([]models.Product, int64, error) {
	var products []models.Product
	var total int64

	query := r.db.Model(&models.Product{})

	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

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

func (r *productRepository) UpdateStock(id uint, newStock int) error {
	return r.db.Model(&models.Product{}).Where("id = ?", id).Update("stock", newStock).Error
}