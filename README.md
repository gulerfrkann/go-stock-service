# Distributed Stock Management & Vector Recommendation Engine

Yüksek eşzamanlılık (high concurrency), veri tutarlılığı ve asenkron olay yönetimi odaklı geliştirilmiş; Go, Redis, RabbitMQ, PostgreSQL ve Qdrant tabanlı dağıtık stok yönetimi ve vektör arama mikroservisidir.

---

## Mimari Bileşenler ve Teknoloji Yığını

| Katman | Teknoloji | Kullanım Amacı |
|---|---|---|
| **Backend Core** | Go (Golang) 1.22+ / Fiber v2 | Yüksek verimli HTTP API ve mikroservis çekirdeği |
| **Veritabanı (RDBMS)** | PostgreSQL 15 / GORM | Ürün ve Outbox olay kayıtları için ACID veri kalıcılığı |
| **Önbellek & Eşzamanlılık** | Redis 7 | Cache-Aside stratejisi ve Lua script tabanlı atomik stok işlemleri |
| **Mesaj Kuyruğu (Broker)** | RabbitMQ 3 (Topic Exchange) | Asenkron olay dağıtımı ve worker iletişimi |
| **Hata İzolasyonu** | DLX / Dead Letter Queue (DLQ) | Poison message yönetimi ve tüketici hata toleransı (resilience) |
| **Vektörel Veritabanı** | Qdrant | Dense vector indeksleme ve semantik ürün tavsiye motoru |
| **Yapay Zeka Entegrasyonu** | Google Gemini API / Python (Scikit-Learn) | Metin embedding çıkarma ve hibrit NLP modelleme |
| **Bildirim Servisi** | Go `net/smtp` / Mailtrap | Asenkron kritik stok e-posta bildirim hattı |
| **Konteynerizasyon** | Docker & Docker Compose | İzole çoklu servis orkestrasyonu |
| **API Dokümantasyonu** | Swagger / OpenAPI 3.0 | İnteraktif API sözleşmesi ve test arayüzü |

---

## Temel Mimari Desenler ve İş Mantığı

### 1. Transactional Outbox Pattern
Veritabanı güncellemesi ile mesaj kuyruğuna yazma arasındaki **dual-write** tutarsızlığını ortadan kaldırmak için kullanılır.
- Stok rezervasyon işlemi esnasında ürün durumu ve fırlatılacak olay kaydı (`outbox_events`) PostgreSQL üzerinde tek bir **ACID Transaction** içinde commit edilir.
- Arka planda çalışan bağımsız `Outbox Worker`, işlenmemiş olayları periyodik olarak okur, RabbitMQ `stock_events` topic exchange'ine güvenli şekilde iletir ve durumu `PROCESSED` olarak işaretler.

### 2. Redis Lua Scripting ile Atomik Stok Yönetimi
Eşzamanlı (concurrent) gelen yoğun isteklerde **race condition** ve **over-selling** (stoktan fazla satma) açıklarını engellemek için stok kontrol ve düşüm adımları Redis üzerinde atomik Lua scriptleri aracılığıyla işletilir.

### 3. Dead Letter Queue (DLQ) ve Hata Toleransı
- RabbitMQ tüketim hattında `autoAck: false` politikası uygulanır.
- Formatı bozuk veya iş mantığı kurallarını ihlal eden (poison) mesajlar ana kuyruğu kilitlememesi için `Nack(false, false)` ile doğrudan `stock_events.dlx` exchange'i üzerinden `critical_stock_dlq` kuyruğuna yönlendirilir ve izole edilir.

### 4. Asenkron Kritik Stok E-Posta Bildirimi
Ürün stoğu belirlenen kritik eşiğin altına düştüğünde ($\le 3$), `stock.critical_alert` yönlendirme anahtarıyla fırlatılan olay, `Stock Consumer` tarafından tüketilir ve depo sorumlularına Mailtrap/SMTP üzerinden HTML formatlı acil tedarik e-postası iletilir.

### 5. Semantik Arama ve Vektör Tabanlı Tavsiye (Qdrant & NLP)
- **Dense Vector Search:** Ürün başlık ve açıklamaları embedding vektörlerine dönüştürülerek Qdrant üzerinde kosinüs benzerliği (Cosine Similarity) ile taranır.
- **İki Kademeli Fallback:** İstek geldiğinde sistem sırasıyla Redis NLP matrisine bakar; bulunamadığı durumlarda Qdrant vektör aramasını devreye sokarak yüksek doğruluklu alternatif ürün listesi üretir.

---

## Mimari Akış Şeması

```mermaid
flowchart TD
    Client([HTTP İstemcisi]) -->|POST /reserve-stock| API[Go / Fiber API]
    
    subgraph ACID Transaction
        API -->|1. Atomik Rezervasyon| DB[(PostgreSQL)]
        API -->|2. Event Kaydı| Outbox[(outbox_events Tablosu)]
    end

    Outbox -->|Periyodik Tarama| Worker[Outbox Worker]
    Worker -->|Publish: stock.critical_alert| RMQ{RabbitMQ Topic Exchange}

    RMQ -->|Mesaj Dağıtımı| MainQueue[critical_stock_queue]
    MainQueue -->|Consume| Consumer[Stock Consumer]

    Consumer -->|Geçerli Mesaj: Ack| Mailer[SMTP / Mailtrap Bildirimi]
    Consumer -->|Hatalı / Zehirli Mesaj: Nack| DLX{Dead Letter Exchange}
    DLX --> DLQ[critical_stock_dlq]

    API -->|GET /recommendations| Redis[(Redis Cache)]
    Redis -.->|Cache Miss| Qdrant[(Qdrant Vector DB)]