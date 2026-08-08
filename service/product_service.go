package service

import (
	"errors"
	"stok-servisi/models"
	"stok-servisi/repository"
)

type ProductService interface {
	CreateProduct(product *models.Product) error
	GetAllProducts() ([]models.Product, error)
	GetProductByID(id uint) (*models.Product, error)
	ReduceStock(id uint, req models.ReduceStockRequest) (*models.Product, error)
}

type productService struct {
	repo repository.ProductRepository
}

func NewProductService(repo repository.ProductRepository) ProductService {
	return &productService{repo: repo}
}

func (s *productService) CreateProduct(product *models.Product) error {
	return s.repo.Create(product)
}

func (s *productService) GetAllProducts() ([]models.Product, error) {
	return s.repo.GetAll()
}

func (s *productService) GetProductByID(id uint) (*models.Product, error) {
	return s.repo.GetByID(id)
}

func (s *productService) ReduceStock(id uint, req models.ReduceStockRequest) (*models.Product, error) {
	product, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if product.Stock < req.Quantity {
		return nil, errors.New("yetersiz stok")
	}

	product.Stock -= req.Quantity

	if err := s.repo.Update(product); err != nil {
		return nil, err
	}

	return product, nil
}