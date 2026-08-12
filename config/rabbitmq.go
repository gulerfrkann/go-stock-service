package config

import (
	"fmt"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

func ConnectRabbitMQ() *amqp.Connection {
	host := os.Getenv("RABBITMQ_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("RABBITMQ_PORT")
	if port == "" {
		port = "5672"
	}
	user := os.Getenv("RABBITMQ_USER")
	if user == "" {
		user = "guest"
	}
	pass := os.Getenv("RABBITMQ_PASS")
	if pass == "" {
		pass = "guest"
	}

	dsn := fmt.Sprintf("amqp://%s:%s@%s:%s/", user, pass, host, port)

	var conn *amqp.Connection
	var err error

	// Docker başlarken RabbitMQ'nun hazır olmasını beklemek için retry döngüsü
	for i := 0; i < 10; i++ {
		conn, err = amqp.Dial(dsn)
		if err == nil {
			Logger.Info("RabbitMQ bağlantısı başarıyla kuruldu")
			return conn
		}
		Logger.Warn("RabbitMQ bekleniyor, 3 saniye sonra tekrar denenecek...", zap.Error(err))
		time.Sleep(3 * time.Second)
	}

	Logger.Fatal("RabbitMQ bağlantısı kurulamadı", zap.Error(err))
	return nil
}