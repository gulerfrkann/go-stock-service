package main

import (
	"context"
	"log"
	"os"

	"stok-servisi/adapter/marketplace"
	"stok-servisi/config"
	"stok-servisi/consumer"
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

	// 6. AI Catalog Worker
	aiWorker := worker.NewAIWorker(amqpConn, productRepo)
	aiWorker.Start()

	// 6.1 Stock Consumer (Kritik stok bildirimleri)
	if err := worker.StartStockConsumer(amqpConn); err != nil {
		logger.Error("Stock Consumer başlatılamadı", zap.Error(err))
	}

	// 6.2 Marketplace Sync Consumer (Çoklu Pazaryeri Entegrasyonu)
	rabbitCh, err := amqpConn.Channel()
	if err != nil {
		logger.Fatal("RabbitMQ Channel açılamadı", zap.Error(err))
	}
	defer rabbitCh.Close()

	hbAdapter := marketplace.NewHepsiburadaAdapter("hb_live_mock_key_991")
	tyAdapter := marketplace.NewTrendyolAdapter("ty_supplier_mock_774")
	syncManager := marketplace.NewSyncManager(hbAdapter, tyAdapter)

	mpConsumer := consumer.NewMarketplaceConsumer(rabbitCh, syncManager)
	if err := mpConsumer.Start(ctx); err != nil {
		logger.Error("Marketplace Consumer başlatılamadı", zap.Error(err))
	}

// 6.3 Saga Pattern - Ödeme Hatası Telafi Consumer (YENİ)
	paymentFailureConsumer := consumer.NewPaymentFailureConsumer(amqpConn, db)
	if err := paymentFailureConsumer.Start(ctx); err != nil {
		logger.Error("Saga Payment Failure Consumer başlatılamadı", zap.Error(err))
	}
// ... (Eski kodların devamı)

	app := fiber.New()

	// 7. CORS İzinleri
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// Static Dosya Sunumu
	app.Static("/", "./public")

	// 8. Observability & Prometheus Metrikleri
	prometheus := fiberprometheus.New("stok_servisi")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
	app.Use(middleware.CorrelationID())

	// 9. Health Check Rotaları
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