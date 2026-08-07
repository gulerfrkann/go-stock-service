package service

import (
	"errors"

	"stok-servisi/models"
	"stok-servisi/repository"
)

type ProductService interface {
	GetAllProducts() ([]models.Product, error)
	CreateProduct(product *models.Product) error
	ReduceStock(id string, adet int) (*models.Product, error)
}

type productService struct {
	repo repository.ProductRepository
}

func NewProductService(repo repository.ProductRepository) ProductService {
	return &productService{repo: repo}
}

func (s *productService) GetAllProducts() ([]models.Product, error) {
	return s.repo.GetAll()
}

func (s *productService) CreateProduct(product *models.Product) error {
	return s.repo.Create(product)
}

func (s *productService) ReduceStock(id string, adet int) (*models.Product, error) {
	product, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("ürün bulunamadı")
	}

	if product.Stok < adet {
		return nil, errors.New("yetersiz stok")
	}

	product.Stok -= adet
	err = s.repo.Update(product)
	if err != nil {
		return nil, err
	}

	return product, nil
}