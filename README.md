#  Distributed Stock Management & Vector Recommendation Engine

Yüksek eşzamanlılık (high concurrency), veri tutarlılığı ve asenkron olay yönetimi odaklı geliştirilmiş; Go, Redis, RabbitMQ, PostgreSQL ve Qdrant tabanlı dağıtık çok kanallı stok yönetimi ve vektör arama mikroservisidir.


##  Hızlı Başlangıç ve Kurulum

Projede yer alan tüm servisleri (PostgreSQL, Redis, RabbitMQ, Qdrant ve Go Backend) Docker ile tek komutla ayağa kaldırabilirsiniz:

1. Depoyu klonlayın:
   ```bash
   git clone [https://github.com/gulerfrkann/go-stock-service.git](https://github.com/gulerfrkann/go-stock-service.git)
   cd go-stock-service

##  Mimari Bileşenler ve Teknoloji Yığını

| Katman | Teknoloji | Kullanım Amacı |
|---|---|---|
| **Backend Core** | Go (Golang) 1.22+ / Fiber v2 | Yüksek verimli HTTP API, idempotency kontrolü ve mikroservis çekirdeği |
| **Veritabanı (RDBMS)** | PostgreSQL 15 / GORM | ~1M ürün kataloğu, GIN indeksli FTS ve Outbox olay kayıtları için ACID kalıcılık |
| **Önbellek & Eşzamanlılık** | Redis 7 (Port 6380) | Cache-Aside stratejisi, TTL tabanlı rezervasyon kilidi ve Lua atomik stok scriptleri |
| **Mesaj Kuyruğu (Broker)** | RabbitMQ 3 (Topic Exchange) | Asenkron olay dağıtımı, pazaryeri senkronizasyonu ve worker iletişimi |
| **Hata İzolasyonu** | DLX / Dead Letter Queue (DLQ) | Poison message yönetimi ve tüketici hata toleransı (resilience) |
| **Vektörel Veritabanı** | Qdrant | Dense vector indeksleme (HNSW) ve semantik ürün tavsiye motoru |
| **Pazaryeri Adaptörleri** | Go Interfaces (Trendyol & HB) | Asenkron çok kanallı stok eşitleme ve simülasyon katmanı |
| **Doğal Dil İşleme (NLP)** | Python (Scikit-Learn / TF-IDF) | Kategori bazlı izole metin benzerliği ve semantik eşleme matrisi |
| **Bildirim Servisi** | Go `net/smtp` / Mailtrap | Asenkron kritik stok e-posta bildirim hattı ($\le 3$ eşik alarmı) |
| **Kontrol Paneli** | Tailwind CSS Dashboard | Merkezi stok, API dokümantasyonu, broker ve vector DB yönetim arayüzü |
| **Konteynerizasyon** | Docker & Docker Compose | İzole orkestrasyon ve kalıcı disk volume yönetimi |

---

## Temel Mimari Desenler ve İş Mantığı

### 1. Transactional Outbox Pattern & Çok Kanallı Eşitleme
Veritabanı güncellemesi ile mesaj kuyruğuna yazma arasındaki **dual-write** tutarsızlığını ortadan kaldırmak için kullanılır.
- Stok rezervasyon işlemi esnasında ürün durumu ve fırlatılacak olay kaydı (`outbox_events`) PostgreSQL üzerinde tek bir **ACID Transaction** içinde commit edilir.
- Arka plandaki `Outbox Worker`, işlenmemiş olayları periyodik olarak okur, RabbitMQ topic exchange'ine iletir ve durumu `PROCESSED` yapar.
- Tüketici (`Stock Consumer`), gelen mesajı işleyerek **Trendyol** ve **Hepsiburada** adaptörlerine anlık stok eşitleme çağrılarını asenkron dağıtır.

### 2. Redis Lua Scripting ile Idempotent Atomik Stok Yönetimi
Eşzamanlı gelen yoğun isteklerde **race condition**, **over-selling** ve mükerrer sipariş açıklarını engellemek için:
- İşlem öncesinde Redis üzerinde sipariş anahtarı (`order:reserved:{id}`) TTL ile kontrol edilir.
- Stok düşümü atomik Lua script ile işletilir; bellek kontrolü başarılı olursa PostgreSQL transaction'ı başlatılır.

### 3. Saga Pattern & Dağıtık Telafi (Rollback) Mekanizması
Çoklu mikroservis ortamlarında veri tutarlılığını ve hata yönetimini sağlamak amacıyla entegre edilmiştir:
- **Başarılı Akış:** Ödeme ve stok rezervasyon adımları başarılı olduğunda işlem normal seyrinde tamamlanır.
- **Hata ve Rollback Senaryosu:** Ödeme adımında (`should_fail: true`) bir hata/red simüle edildiğinde, sistem otomatik olarak RabbitMQ `payment.failed` kuyruğuna olay fırlatır.
- **Telafi Worker'ı (`PaymentFailureConsumer`):** Bu olayı anında yakalayarak veritabanında rezerve edilen stokları ilgili ürünün stoğuna güvenli bir şekilde geri iade eder (rollback).

### 4. Dead Letter Queue (DLQ) ve Hata Toleransı
- RabbitMQ tüketim hattında `autoAck: false` politikası uygulanır.
- Formatı bozuk veya iş mantığı kurallarını ihlal eden mesajlar ana kuyruğu kilitlememesi için `Nack(false, false)` ile doğrudan `stock_events.dlx` üzerinden `critical_stock_dlq` kuyruğuna yönlendirilir.

### 5. Asenkron Kritik Stok E-Posta Bildirimi
Ürün stoğu kritik eşiğin altına düştüğünde ($\le 3$), `stock.critical_alert` yönlendirme anahtarıyla fırlatılan olay tüketilir ve depo sorumlularına Mailtrap/SMTP üzerinden HTML acil tedarik e-postası iletilir.

### 6. Semantik Arama ve Kategori Bazlı NLP Tavsiye Motoru
- **Kategori İçi İzolasyon:** Monitör arayan kullanıcıya monitör donanımları, kitap arayana kitap önerilmesi için ürünler kategori bazında izole TF-IDF ve kosinüs benzerliği matrisiyle eşleştirilir.
- **İki Kademeli Fallback:** İstek geldiğinde sistem sırasıyla Redis NLP önbelleğine bakar; bulunamadığı durumlarda Qdrant vektör aramasını devreye sokarak alternatif ürün listesi üretir.

---

## 📄 API Dokümantasyonu & Test
Proje çalıştırıldıktan sonra uçtan uca etkileşimli testler ve şema incelemesi için Swagger UI arayüzü kullanılabilir:
- **Swagger UI:** `http://localhost:8081/swagger/index.html`
- **Ödeme & Saga Testi:** `POST /api/v1/payment/process` (İstek gövdesinde `should_fail: true/false` parametresi ile başarı ve rollback senaryoları simüle edilir.)

---

## Mimari Akış Şeması

```mermaid
flowchart TD
    Client([HTTP İstemcisi]) -->|POST /reserve-stock| API[Go / Fiber API]
    
    subgraph Core_Engine [Çekirdek İşlem & Outbox]
        API -->|1. Atomik Lua Script & Idempotency| RedisCache[(Redis Cache)]
        API -->|2. Pessimistic Lock Stok Düşümü| PG[(PostgreSQL stok_db)]
        API -->|3. Transactional Event Kaydı| OutboxTable[(outbox_events Tablosu)]
    end

    subgraph Async_Pipeline [Asenkron İletim & RabbitMQ]
        OutboxTable -->|Periyodik Tarama| Worker[Outbox Worker]
        Worker -->|Publish: stock.reserved / critical_alert| Exchange{RabbitMQ Topic Exchange}
        Exchange -->|Mesaj Dağıtımı| Queue[stock_sync_queue]
        Queue -->|Consume| Consumer[Stock Consumer]
    end

    subgraph Marketplace_Notification [Pazaryeri, Ödeme Saga & Alarm Entegrasyonu]
        Consumer -->|Geçerli Mesaj: Ack| SyncLogic[Sync Dağıtıcı]
        SyncLogic -->|Stok Eşitleme| Adapters[Trendyol & Hepsiburada Adaptörleri]
        SyncLogic -->|Kritik Eşik <= 3| Mail[SMTP / Mailtrap Bildirimi]
        Consumer -->|Hatalı Mesaj: Nack| DLX{Dead Letter Exchange}
        DLX --> DLQ[stock_sync_dlq]
    end

    subgraph Recommendation_Engine [Tavsiye Motoru]
        Client -->|GET /recommendations| RecAPI[Recommendation API]
        RecAPI -->|Cache Hit| RedisCache
        RecAPI -->|Cache Miss / Fallback| NLP[Kategori İçi TF-IDF / Qdrant Vektör DB]
    end