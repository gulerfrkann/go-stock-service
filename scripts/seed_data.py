import duckdb
import psycopg2
from psycopg2.extras import execute_values
from datetime import datetime

print("1/4 - DuckDB motoru başlatılıyor...")
con = duckdb.connect()

print("2/4 - Parquet dosyaları disk üzerinde SQL ile işleniyor (RAM harcanmaz)...")

# DuckDB ile doğrudan parquet dosyaları üzerinde SQL çalıştırıyoruz
query = """
WITH siparis_ozet AS (
    SELECT 
        regexp_replace(CAST(Barcode AS VARCHAR), '\\.0$', '') AS barcode_clean,
        SUM(Quantity) AS total_quantity
    FROM 'stajyer_tavsiye_sistem2.parquet'
    WHERE Barcode IS NOT NULL AND length(CAST(Barcode AS VARCHAR)) > 3
    GROUP BY 1
)
SELECT 
    TRIM(u.Urun) AS name,
    GREATEST(CAST(COALESCE(s.total_quantity * 2, 50) AS INTEGER), 10) AS stock,
    ROUND(100.0 + COALESCE(s.total_quantity * 0.5, 10.0), 2) AS price
FROM 'stajyer_tavsiye_sistem.parquet' u
LEFT JOIN siparis_ozet s 
    ON regexp_replace(CAST(u.Barkod AS VARCHAR), '\\.0$', '') = s.barcode_clean
WHERE u.Urun IS NOT NULL AND TRIM(u.Urun) != '';
"""

# Sorgu sonucunu bir result-set olarak alıyoruz
result = con.execute(query)

print("3/4 - PostgreSQL bağlantısı kuruluyor...")
conn_params = {
    "host": "localhost",
    "port": "5432",
    "database": "stok_db",
    "user": "postgres",
    "password": "postgres"
}

conn = psycopg2.connect(**conn_params)
cursor = conn.cursor()

print("4/4 - Toplu aktarım başlıyor (25.000'erli paketler halinde)...")

BATCH_SIZE = 25000
total_inserted = 0
now = datetime.now()

insert_query = """
    INSERT INTO products (name, stock, price, created_at, updated_at)
    VALUES %s;
"""

while True:
    # DuckDB'den sıradaki 25.000 satırı çek
    rows = result.fetchmany(BATCH_SIZE)
    if not rows:
        break
    
    # Veritabanı formatına dönüştür
    records = [(r[0], r[1], float(r[2]), now, now) for r in rows]
    
    # PostgreSQL'e batch halinde bas
    execute_values(cursor, insert_query, records)
    conn.commit()
    
    total_inserted += len(records)
    print(f"📦 Aktarılan Ürün Sayısı: {total_inserted:,}")

cursor.close()
conn.close()
con.close()

print(f"\n İŞLEM TAMAMLANDI! Toplam {total_inserted:,} adet ürün kilitlenme olmadan veritabanına eklendi.")