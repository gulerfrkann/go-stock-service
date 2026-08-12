package main

import (
	"context"
	"log"
	"os"

	"stok-servisi/config"
	"stok-servisi/handlers"
	"stok-servisi/middleware"
	"stok-servisi/models"
	"stok-servisi/repository"
	"stok-servisi/routes"
	"stok-servisi/service"
	"stok-servisi/worker"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"go.uber.org/zap"
)

func main() {
	// 1. Uber Zap Logger Başlatılması
	logger := config.InitLogger()
	defer logger.Sync()

	// 2. Veritabanı, Redis ve RabbitMQ Bağlantıları
	db := config.ConnectDB()
	rdb := config.ConnectRedis()
	amqpConn := config.ConnectRabbitMQ()
	defer amqpConn.Close()

	// 3. Veritabanı Tablo Migrasyonları (Product ve OutboxEvent)
	if err := db.AutoMigrate(&models.Product{}, &models.OutboxEvent{}); err != nil {
		logger.Fatal("Veritabanı migrasyon hatası", zap.Error(err))
	}

	// 4. Outbox Relay Worker (Goroutine olarak arka planda baslatilir)
	outboxWorker := worker.NewOutboxWorker(db, amqpConn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go outboxWorker.Start(ctx)

	// 5. Bağımlılıkların Oluşturulması (Dependency Injection)
	productRepo := repository.NewProductRepository(db)
	productService := service.NewProductService(productRepo, rdb)
	productHandler := handlers.NewProductHandler(productService)
	healthHandler := handlers.NewHealthHandler(db, rdb)

	// 6. AI Catalog Worker (RabbitMQ'dan resim yükleme olaylarını dinler ve ürünü zenginleştirir)
	aiWorker := worker.NewAIWorker(amqpConn, productRepo)
	aiWorker.Start()

	app := fiber.New()

	// 7. CORS İzinleri (Tarayıcı / HTML panellerinin API'ye erişebilmesi için)
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// Static Dosya Sunumu (HTML panellerini doğrudan sunucudan yayınlamak için)
	app.Static("/", "./public")

	// 8. Observability Middleware'leri & Prometheus Metrikleri
	prometheus := fiberprometheus.New("stok_servisi")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
	app.Use(middleware.CorrelationID())

	// 9. Health Check Rotaları (/healthz/live & /healthz/ready)
	routes.SetupHealthRoutes(app, healthHandler)

	// 10. API Versiyon Grubu ve Ürün Rotaları
	api := app.Group("/api/v1")
	routes.SetupProductRoutes(api, productHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Sunucu başlatılıyor", zap.String("port", port))
	log.Fatal(app.Listen(":" + port))
}