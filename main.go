package main

import (
	"log"
	"os"

	"stok-servisi/config"
	"stok-servisi/handlers"
	"stok-servisi/middleware"
	"stok-servisi/models"
	"stok-servisi/repository"
	"stok-servisi/routes"
	"stok-servisi/service"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func main() {
	// 1. Uber Zap Logger Başlatılması
	logger := config.InitLogger()
	defer logger.Sync()

	// 2. Veritabanı ve Redis Bağlantıları
	db := config.ConnectDB()
	rdb := config.ConnectRedis()

	// Veritabanı Tablo Migrasyonu
	if err := db.AutoMigrate(&models.Product{}); err != nil {
		logger.Fatal("Veritabanı migrasyon hatası", zap.Error(err))
	}

	// 3. Bağımlılıkların Oluşturulması (Dependency Injection)
	productRepo := repository.NewProductRepository(db)
	productService := service.NewProductService(productRepo, rdb)
	productHandler := handlers.NewProductHandler(productService)
	healthHandler := handlers.NewHealthHandler(db, rdb) // Health Handler eklendi

	app := fiber.New()
	app.Static("/", "./public")

	// 4. Observability Middleware'leri & Prometheus Metrikleri
	prometheus := fiberprometheus.New("stok_servisi")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
	app.Use(middleware.CorrelationID()) // Her isteğe benzersiz X-Correlation-ID atar

	// 5. Health Check Rotaları (/healthz/live & /healthz/ready)
	routes.SetupHealthRoutes(app, healthHandler)

	// 6. API Versiyon Grubu ve Ürün Rotaları
	api := app.Group("/api/v1")
	routes.SetupProductRoutes(api, productHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Sunucu başlatılıyor", zap.String("port", port))
	log.Fatal(app.Listen(":" + port))
}