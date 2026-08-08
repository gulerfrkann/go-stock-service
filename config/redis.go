package config

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

// Go'da Redis işlemleri eşzamanlılık (concurrency) ve zamanaşımı (timeout) 
// yönetimi için bir Context nesnesine ihtiyaç duyar.
var Ctx = context.Background()

func ConnectRedis() *redis.Client {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost" // Docker dışında çalıştırılırsa varsayılan adres
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379" // Standart Redis portu
	}

	// Redis İstemcisi Oluşturma
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: "", // Parola belirlemediğimiz için boş bırakıyoruz
		DB:       0,  // Varsayılan veritabanı indeksini (0) kullanıyoruz
	})

	// PING Komutuyla Bağlantı Testi
	// Redis sunucusuna "Orada mısın?" sorusu atarız, "PONG" cevabı bekleriz.
	_, err := rdb.Ping(Ctx).Result()
	if err != nil {
		log.Fatalf("Redis sunucusuna bağlanılamadı: %v", err)
	}

	log.Println("Redis bağlantısı başarıyla sağlandı!")
	return rdb
}