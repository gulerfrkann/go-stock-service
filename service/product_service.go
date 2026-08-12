package service

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
    "stok-servisi/models"
    "stok-servisi/repository"
)

var decreaseStockLuaScript = redis.NewScript(`
    local current_stock = redis.call('GET', KEYS[1])
    if not current_stock then
        return -2
    end
    if tonumber(current_stock) >= tonumber(ARGV[1]) then
        return redis.call('DECRBY', KEYS[1], ARGV[1])
    else
        return -1
    end
`)

type ProductService interface {
    CreateProduct(product *models.Product) error
    GetProducts(page, limit int, search string) ([]models.Product, int64, error)
    GetProductByID(id uint) (*models.Product, error)
    UpdateProduct(product *models.Product) error
    ReduceStock(ctx context.Context, productID uint, quantity int) (int64, error)
}

type productService struct {
    repo        repository.ProductRepository
    redisClient *redis.Client
}

func NewProductService(repo repository.ProductRepository, redisClient *redis.Client) ProductService {
    return &productService{
        repo:        repo,
        redisClient: redisClient,
    }
}

func (s *productService) CreateProduct(product *models.Product) error {
    return s.repo.Create(product)
}

func (s *productService) GetProducts(page, limit int, search string) ([]models.Product, int64, error) {
    return s.repo.GetProducts(page, limit, search)
}

func (s *productService) GetProductByID(id uint) (*models.Product, error) {
    return s.repo.GetByID(id)
}

func (s *productService) UpdateProduct(product *models.Product) error {
    return s.repo.Update(product)
}

func (s *productService) ReduceStock(ctx context.Context, productID uint, quantity int) (int64, error) {
    redisKey := fmt.Sprintf("stock:product:%d", productID)

    loadCache := func() error {
        product, err := s.repo.GetByID(productID)
        if err != nil {
            return errors.New("ürün bulunamadı")
        }
        s.redisClient.SetNX(ctx, redisKey, product.Stock, 1*time.Hour)
        return nil
    }

    exists, err := s.redisClient.Exists(ctx, redisKey).Result()
    if err != nil || exists == 0 {
        if err := loadCache(); err != nil {
            return 0, err
        }
    }

    keys := []string{redisKey}
    args := []interface{}{quantity}

    result, err := decreaseStockLuaScript.Run(ctx, s.redisClient, keys, args...).Int64()
    if err != nil {
        return 0, fmt.Errorf("redis script hatası: %v", err)
    }

    if result == -2 {
        if err := loadCache(); err != nil {
            return 0, err
        }
        result, err = decreaseStockLuaScript.Run(ctx, s.redisClient, keys, args...).Int64()
        if err != nil {
            return 0, fmt.Errorf("redis script hatası: %v", err)
        }
    }

    if result == -1 {
        return 0, errors.New("yetersiz stok! işlem reddedildi")
    }

    if result == -2 {
        return 0, errors.New("stok bilgisi senkronize edilemedi, lütfen tekrar deneyin")
    }

    err = s.repo.UpdateStock(productID, int(result))
    if err != nil {
        s.redisClient.Del(ctx, redisKey)
        return 0, errors.New("veritabanı güncellenirken hata oluştu, işlem geri alındı")
    }

    return result, nil
}
