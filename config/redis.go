package config

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

var Ctx = context.Background()

func ConnectRedis() *redis.Client {
	// docker-compose.yml içinde tanımlanan REDIS_ADDR değişkenini oku
	addr := os.Getenv("REDIS_ADDR")

	// Eğer REDIS_ADDR yoksa REDIS_HOST ve REDIS_PORT değerlerini dene
	if addr == "" {
		redisHost := os.Getenv("REDIS_HOST")
		if redisHost == "" {
			redisHost = "redis-cache" // Docker ağındaki servis adı
		}

		redisPort := os.Getenv("REDIS_PORT")
		if redisPort == "" {
			redisPort = "6379"
		}
		addr = fmt.Sprintf("%s:%s", redisHost, redisPort)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

	_, err := rdb.Ping(Ctx).Result()
	if err != nil {
		log.Printf("⚠️ Redis bağlantı uyarısı (%s): %v. İşleme devam ediliyor...", addr, err)
		return rdb
	}

	log.Println("✅ Redis bağlantısı başarıyla sağlandı!")
	return rdb
}