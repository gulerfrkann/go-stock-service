#  Go Stock Service (Stok Yönetimi Mikroservisi)

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![Fiber Framework](https://img.shields.io/badge/Fiber-v2-000000?style=flat&logo=gofiber)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15-4169E1?style=flat&logo=postgresql)
![Redis](https://img.shields.io/badge/Redis-7-DC382D?style=flat&logo=redis)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat&logo=docker)
![CI/CD](https://github.com/gulerfrkann/go-stock-service/actions/workflows/ci.yml/badge.svg)

Go (Golang) ve Fiber framework kullanılarak geliştirilmiş; Docker mimarisi üzerinde PostgreSQL ile veri kalıcılığı sağlayan, Redis ile **Cache-Aside Pattern** performans önbelleklemesi sunan ve GitHub Actions ile sürekli entegrasyonu (CI/CD) yapılan yüksek performanslı stok yönetimi RESTful API mikroservisi.

---

##  Mimari ve Teknolojiler

- **Programlama Dili:** Go (Golang)
- **Web Framework:** [Fiber v2](https://gofiber.io/) (Yüksek performanslı Go web framework'ü)
- **ORM:** [GORM](https://gorm.io/) (Object Relational Mapping)
- **Veritabanı:** PostgreSQL
- **Önbellekleme (Caching):** Redis (Cache-Aside Pattern)
- **Konteynerizasyon:** Docker & Docker Compose
- **Dokümantasyon:** Swagger UI (OpenAPI 3.0)
- **Test & CI/CD:** Testify, Miniredis & GitHub Actions

---

##  Öne Çıkan Özellikler & Önbellek Stratejisi

- **Clean Architecture:** Katmanlı mimari (Config, Handler, Service, Repository, Model) prensiplerine uygun yapı.
- **Cache-Aside Pattern (Önbellek Yönetimi):**
  - **Cache Hit:** Ürün listeleme istekleri (`GET /api/v1/products`) öncelikle Redis RAM önbelleğinden milisaniyeler seviyesinde yanıtlanır.
  - **Cache Miss:** Önbellekte veri olmaması durumunda veri PostgreSQL'den okunur ve 10 dakikalık yaşam süresi (TTL) ile Redis'e kaydedilir.
  - **Cache Invalidation:** Yeni ürün eklendiğinde (`POST`) veya stok düşürüldüğünde (`POST /reduce-stock`) Redis'teki eski veri otomatik temizlenir.
- **İnteraktif API Dokümantasyonu:** Swagger UI entegrasyonu ile canlı test imkanı.
- **Birim Testler (Unit Testing):** Service katmanı için `testify/mock` ve `miniredis` kullanılarak yazılmış bağımsız birim testleri.
- **Otomatik CI/CD:** Yapılan her commit ve Pull Request'te GitHub Actions üzerinde otomatik koşulan test senaryoları.

---

##  Projenin Çalıştırılması (Docker Compose)

Projede PostgreSQL, Redis ve Go API servisleri Docker ağı (`stok-network`) üzerinde izole bir şekilde çalışır.

### Gereksinimler
- Docker & Docker Compose

### Adımlar

1. Repoyu klonla:
   git clone [https://github.com/gulerfrkann/go-stock-service.git](https://github.com/gulerfrkann/go-stock-service.git)
   cd go-stock-service
