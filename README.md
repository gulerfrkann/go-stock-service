# Go Stock Service & Recommendation Engine

Go (Golang) ve Fiber framework kullanılarak geliştirilmiş, PostgreSQL ile veri kalıcılığı sağlayan, Redis ile Cache-Aside Pattern ve NLP bazlı tavsiye önbelleklemesi sunan, Docker Compose mimarisi üzerinde izole çalışan yüksek performanslı stok yönetimi ve ürün tavsiye mikroservisidir.

---

## Mimari ve Teknolojiler

- **Backend:** Go (Golang) 1.22+
- **Web Framework:** Fiber v2
- **ORM:** GORM (PostgreSQL Driver)
- **Veritabanı:** PostgreSQL 15
- **Önbellek & Vektör Önbellekleme:** Redis 7
- **Makine Öğrenmesi & NLP:** Python 3.10+, Scikit-learn (TF-IDF Vectorization, Cosine Similarity), Pandas
- **Konteynerizasyon:** Docker & Docker Compose
- **Dokümantasyon:** Swagger UI (OpenAPI 3.0)
- **Test & Sürekli Entegrasyon:** Testify, Miniredis, GitHub Actions (CI/CD)

---

## Sistem Özellikleri ve Tasarım Desenleri

### 1. Katmanlı Mimari (Clean Architecture)
Proje; Handler, Service, Repository, Model ve Config katmanlarına ayrılarak SOLID prensiplerine uygun, genişletilebilir ve test edilebilir bir yapıda tasarlanmıştır.

### 2. Önbellek Stratejisi (Cache-Aside Pattern)
- **Cache Hit:** Ürün listeleme istekleri (`GET /api/v1/products`) doğrudan Redis önbelleğinden milisaniyeler seviyesinde sunulur.
- **Cache Miss:** Önbellekte veri bulunamadığında sorgu PostgreSQL'e iletilir ve sonuç belirlenen TTL (10 dakika) süresiyle Redis'e yazılır.
- **Cache Invalidation:** Yeni ürün eklendiğinde (`POST /api/v1/products`) veya stok düşürüldüğünde (`POST /api/v1/products/reduce-stock`) Redis önbelleği otomatik olarak temizlenir.

### 3. Kategori Bazlı NLP Tavsiye Motoru
- **ETL ve Veri Hijyeni:** Ham ERP veri setlerindeki muhasebe, iskonto, kampanya farkı ve kargo gibi ticari olmayan fatura kalemleri filtreleme katmanında ayıklanır.
- **İzole TF-IDF Hesaplaması:** Ürünler kategorilerine göre gruplandırılarak metin benzerlikleri (TF-IDF & Cosine Similarity) kategori içinde izole hesaplanır. Bu sayede bellek tüketimi optimize edilir ve farklı kategoriler arasındaki anlamsal sapmalar engellenir.
- **Hibrit Fallback Mekanizması:** Tavsiye isteklerinde (`GET /api/v1/products/:id/recommendations`) sistem sırasıyla:
  1. Redis üzerindeki önceden hesaplanmış NLP matrisine bakar (O(1) erişim hızı).
  2. Önbellekte bulunmayan yeni ürünler için Go servis katmanında kategori bazlı güvenli arama fallback'ini devreye sokarak cevapsız istek kalmamasını sağlar.

---

## API Uç Noktaları

| Metot | Uç Nokta | Açıklama |
|---|---|---|
| GET | `/api/v1/products` | Sayfalanmış ürün listesini döner (Redis Cache) |
| GET | `/api/v1/products/:id` | Belirtilen ürünün detayını döner |
| POST | `/api/v1/products` | Yeni ürün kaydı oluşturur (Cache Invalidation) |
| POST | `/api/v1/products/reduce-stock` | Belirtilen ürünün stoğunu düşürür |
| GET | `/api/v1/products/:id/recommendations` | Ürüne ait yapay zeka/kategori bazlı alternatifleri listeler |
| GET | `/swagger/*` | Canlı Swagger API dokümantasyonu |

---

## Kurulum ve Çalıştırma

### Gereksinimler
- Docker & Docker Compose
- Python 3.10+ (Veri aktarımı ve ML pipeline çalıştırmak için)

### 1. Servisleri Başlatma

Repoyu klonlayıp Docker Compose ile PostgreSQL, Redis ve Go mikroservisini ayağa kaldırın:

```bash
git clone [https://github.com/gulerfrkann/go-stock-service.git](https://github.com/gulerfrkann/go-stock-service.git)
cd go-stock-service
docker-compose up -d