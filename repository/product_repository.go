package repository

import (
	"errors"

	"stok-servisi/models"

	"gorm.io/gorm"
)

type ProductRepository interface {
	Create(product *models.Product) error
	GetProducts(page, limit int, search string) ([]models.Product, int64, error)
	GetByID(id uint) (*models.Product, error)
	Update(product *models.Product) error
	UpdateStock(productID uint, newStock int) error
	ReserveStock(productID uint, quantity int) error
	ReserveStockWithOutbox(productID uint, quantity int, event *models.OutboxEvent) error
	SaveProductImageWithOutbox(productID uint, imageURL string, event *models.OutboxEvent) error
	UpdateAICatalogData(productID uint, category, seoTitle, description, careInstructions string) error
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

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err = query.Offset(offset).Limit(limit).Find(&products).Error
	return products, total, err
}

func (r *productRepository) GetByID(id uint) (*models.Product, error) {
	var product models.Product
	err := r.db.First(&product, id).Error
	return &product, err
}

func (r *productRepository) Update(product *models.Product) error {
	return r.db.Save(product).Error
}

func (r *productRepository) UpdateStock(productID uint, newStock int) error {
	return r.db.Model(&models.Product{}).Where("id = ?", productID).Update("stock", newStock).Error
}

func (r *productRepository) ReserveStock(productID uint, quantity int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var product models.Product
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&product, productID).Error; err != nil {
			return err
		}

		if product.Stock < quantity {
			return errors.New("yetersiz stok")
		}

		product.Stock -= quantity
		return tx.Save(&product).Error
	})
}

// Transactional Outbox Pattern ile stok düşümü ve Outbox olayı kaydı
func (r *productRepository) ReserveStockWithOutbox(productID uint, quantity int, event *models.OutboxEvent) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var product models.Product
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&product, productID).Error; err != nil {
			return err
		}

		if product.Stock < quantity {
			return errors.New("yetersiz stok")
		}

		product.Stock -= quantity
		if err := tx.Save(&product).Error; err != nil {
			return err
		}

		if err := tx.Create(event).Error; err != nil {
			return err
		}

		return nil
	})
}

// Transactional Outbox Pattern ile Görsel URL güncelleme ve ProductImageUploaded olay kaydı
func (r *productRepository) SaveProductImageWithOutbox(productID uint, imageURL string, event *models.OutboxEvent) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Product{}).Where("id = ?", productID).Update("image_url", imageURL).Error; err != nil {
			return err
		}

		if err := tx.Create(event).Error; err != nil {
			return err
		}

		return nil
	})
}

// AI Worker tarafından üretilen katalog verilerini veritabanında güncelleme
func (r *productRepository) UpdateAICatalogData(productID uint, category, seoTitle, description, careInstructions string) error {
	return r.db.Model(&models.Product{}).Where("id = ?", productID).Updates(map[string]interface{}{
		"category":          category,
		"seo_title":         seoTitle,
		"description":       description,
		"care_instructions": careInstructions,
		"ai_cataloged":      true,
	}).Error
}