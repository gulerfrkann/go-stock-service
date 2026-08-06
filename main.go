package main

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// Urun: Veritabanımızdaki ürün şablonunu temsil eden Struct
type Urun struct {
	ID    int     `json:"id"`
	Ad    string  `json:"ad"`
	Fiyat float64 `json:"fiyat"`
	Stok  int     `json:"stok"`
}

// envanter: Veritabanı öncesi RAM'de tuttuğumuz geçici ürün listesi
var envanter = []Urun{
	{ID: 1, Ad: "Laptop", Fiyat: 25000.0, Stok: 10},
	{ID: 2, Ad: "Kablosuz Fare", Fiyat: 450.0, Stok: 5},
}

func main() {
	// 1. Fiber web sunucusu uygulamasını başlatıyoruz
	app := fiber.New()

	// ---------------------------------------------------------
	// ENDPOINT 1: Tüm Ürünleri Listele (GET)
	// ---------------------------------------------------------
	app.Get("/api/v1/products", func(c *fiber.Ctx) error {
		// HTTP 200 (OK) statü kodu ve envanter dizisini JSON olarak döner
		return c.Status(200).JSON(envanter)
	})

	// ---------------------------------------------------------
	// ENDPOINT 2: Yeni Ürün Ekle (POST)
	// ---------------------------------------------------------
	app.Post("/api/v1/products", func(c *fiber.Ctx) error {
		var yeniUrun Urun

		// İstemciden (Postman/Frontend) gelen JSON verisini okuyup 'yeniUrun' değişkenine yazar
		if err := c.BodyParser(&yeniUrun); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "Geçersiz JSON formatı gönderildi",
			})
		}

		// Otomatik ID atayıp envanter dizisine ekler
		yeniUrun.ID = len(envanter) + 1
		envanter = append(envanter, yeniUrun)

		// HTTP 201 (Created) statüsü ile eklenen ürünü geri döner
		return c.Status(201).JSON(yeniUrun)
	})

	// ---------------------------------------------------------
	// ENDPOINT 3: Stok Düş (POST)
	// ---------------------------------------------------------
	app.Post("/api/v1/products/:id/reduce-stock", func(c *fiber.Ctx) error {
		// URL'den gelen ':id' parametresini alır ve metinden (string) tam sayıya (int) çevirir
		idParam := c.Params("id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "Geçersiz ID formatı",
			})
		}

		// İstek gövdesinden (Body) kaç adet stok düşüleceğini okumak için geçici struct
		type StokDusIstegi struct {
			Adet int `json:"adet"`
		}
		var istek StokDusIstegi

		if err := c.BodyParser(&istek); err != nil || istek.Adet <= 0 {
			return c.Status(400).JSON(fiber.Map{
				"error": "Lütfen düşülecek adet miktarını 0'dan büyük girin",
			})
		}

		// Envanterde ürünü arama ve stok düşme mantığı
		for i := range envanter {
			if envanter[i].ID == id {
				// Stok yetersizliği kontrolü
				if envanter[i].Stok < istek.Adet {
					return c.Status(400).JSON(fiber.Map{
						"error":       "Yetersiz stok!",
						"mevcut_stok": envanter[i].Stok,
					})
				}

				// Stok yeterliyse RAM'deki orijinal veriyi günceller
				envanter[i].Stok -= istek.Adet

				return c.Status(200).JSON(fiber.Map{
					"message": "Stok başarıyla güncellendi",
					"urun":    envanter[i],
				})
			}
		}

		// Ürün listede bulunamadıysa
		return c.Status(404).JSON(fiber.Map{
			"error": "Ürün bulunamadı",
		})
	})

	// 2. Web sunucusunu 8080 portunda dinlemeye alır
	app.Listen(":8080")
}