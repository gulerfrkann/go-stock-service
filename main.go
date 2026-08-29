package main

import (
	"context"
	"log"

	"stok-servisi/adapter/marketplace"
	"stok-servisi/config"
	"stok-servisi/consumer"
	_ "stok-servisi/docs" // Swagger dokümantasyon paketi
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
	"github.com/gofiber/swagger" // Güncel Fiber Swagger paketi
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
    
	// 5.1 Ödeme Servis ve Gateway Bağımlılıkları
	paymentGateway := &service.DefaultPaymentGateway{}
	paymentService := service.NewPaymentService(db, amqpConn, paymentGateway)
	paymentHandler := handlers.NewPaymentHandler(paymentService)

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

	// 6.3 Saga Pattern - Ödeme Hatası Telafi Consumer
	paymentFailureConsumer := consumer.NewPaymentFailureConsumer(amqpConn, db)
	if err := paymentFailureConsumer.Start(ctx); err != nil {
		logger.Error("Saga Payment Failure Consumer başlatılamadı", zap.Error(err))
	}

	app := fiber.New()

	// 7. CORS İzinleri
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// 8. Observability & Prometheus Metrikleri
	prometheus := fiberprometheus.New("stok_servisi")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
	app.Use(middleware.CorrelationID())

	// --- KESİN ÇÖZÜM BAŞLANGICI ---
	
// --- KESİN ÇÖZÜM BAŞLANGICI ---
    
    // 9. Önce swagger.json dosyasını doğrudan diskten sunuyoruz (Çakışmayı %100 engeller)
    app.Get("/swagger.json", func(c *fiber.Ctx) error {
        return c.SendFile("./docs/swagger.json")
    })

    // 10. Swagger arayüzünü başlatıp doğrudan üstte sunduğumuz URL'ye bağlıyoruz
    app.Get("/swagger/*", swagger.New(swagger.Config{
        URL: "/swagger.json", // UI artık hiçbir yeri aramayacak, direkt bu dosyayı okuyacak
    }))

    // --- KESİN ÇÖZÜM BİTİŞİ ---

    // 11. Static Dosya Sunumu (Artık swagger'ı ezemez)
    app.Static("/", "./public")

    // 12. API Versiyon Grubu (Önce grubu tanımlıyoruz ki rotalarda kullanabilelim)
    api := app.Group("/api/v1")

    // 13. Rotaların Bağlanması
    routes.SetupHealthRoutes(app, healthHandler)
    routes.SetupProductRoutes(api, productHandler)
    
    // Ödeme Rotaları (Saga Testleri İçin - Artık 'api' değişkeni tanımlı olduğu için hata vermeyecek)
    // Ödeme Rotaları (Güncellenmiş Katmanlı Mimari)
	routes.SetupPaymentRoutes(api, paymentHandler)

    // 14. Sunucunun Başlatılması
    logger.Info("Sunucu başlatılıyor", zap.String("port", "8081"))
    log.Fatal(app.Listen(":8081"))
}