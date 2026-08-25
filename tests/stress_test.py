import concurrent.futures
import requests
import time
import psycopg2
import redis
import json

URL = "http://localhost:8080/api/v1/products/reserve-stock"
PRODUCT_ID = 1
INITIAL_STOCK = 10
CONCURRENT_USERS = 50

print("--- TEST BAŞLATILIYOR ---")
conn = psycopg2.connect(host="localhost", port=5432, database="stok_db", user="postgres", password="postgres")
cur = conn.cursor()
cur.execute("UPDATE products SET stock = %s WHERE id = %s;", (INITIAL_STOCK, PRODUCT_ID))
conn.commit()
cur.close()
conn.close()

# Redis anahtarını doğrudan başlangıç stoğuyla eşitleyelim
r = redis.Redis(host="localhost", port=6380, db=0)
r.set(f"stock:product:{PRODUCT_ID}", INITIAL_STOCK)

print(f"📦 Başlangıç Stoğu: {INITIAL_STOCK}")
print(f"🚀 Aynı anda saldıran kullanıcı sayısı: {CONCURRENT_USERS}")
print("⚡ İstekler gönderiliyor...\n")

success_count = 0
failed_count = 0
sample_errors = []

def send_reserve_request(user_idx):
    payload = {
        "product_id": PRODUCT_ID,
        "order_id": f"ORD-STRESS-{user_idx}-{int(time.time() * 1000)}",
        "quantity": 1,
        "expiration_secs": 300
    }
    headers = {"Content-Type": "application/json"}
    try:
        res = requests.post(URL, data=json.dumps(payload), headers=headers, timeout=5)
        return res.status_code, res.text
    except Exception as e:
        return 500, str(e)

start_time = time.time()
with concurrent.futures.ThreadPoolExecutor(max_workers=CONCURRENT_USERS) as executor:
    futures = [executor.submit(send_reserve_request, i) for i in range(CONCURRENT_USERS)]
    for future in concurrent.futures.as_completed(futures):
        status, text = future.result()
        if status in [200, 201]:
            success_count += 1
        else:
            failed_count += 1
            if len(sample_errors) < 3:
                sample_errors.append(f"HTTP {status} -> {text}")

elapsed = time.time() - start_time

conn = psycopg2.connect(host="localhost", port=5432, database="stok_db", user="postgres", password="postgres")
cur = conn.cursor()
cur.execute("SELECT stock FROM products WHERE id = %s;", (PRODUCT_ID,))
final_db_stock = cur.fetchone()[0]
cur.close()
conn.close()

final_redis_stock = r.get(f"stock:product:{PRODUCT_ID}")
final_redis_stock = int(final_redis_stock) if final_redis_stock else 0

print("=" * 45)
print("🎯 STRES TESTİ SONUÇ RAPORU")
print("=" * 45)
print(f"⏱️ Toplam Süre           : {elapsed:.2f} saniye")
print(f"✅ Başarılı Sipariş      : {success_count} (Beklenen: {INITIAL_STOCK})")
print(f"❌ Reddedilen İstek      : {failed_count} (Beklenen: {CONCURRENT_USERS - INITIAL_STOCK})")
print(f"📊 PostgreSQL Kalan Stok : {final_db_stock} (Beklenen: 0)")
print(f"⚡ Redis Kalan Stok      : {final_redis_stock} (Beklenen: 0)")
print("-" * 45)

if sample_errors:
    print("🔍 Örnek Hata Yanıtları:")
    for err in sample_errors:
        print(f"   {err}")
    print("-" * 45)

if success_count == INITIAL_STOCK and final_db_stock == 0 and final_redis_stock == 0:
    print("🏆 KUSURSUZ! Sıfır Over-Selling, Atomik Kilit ve Eşzamanlılık Doğrulandı!")
else:
    print("⚠️ UYARI: Kontrol edilmesi gereken durumlar var.")
print("=" * 45)