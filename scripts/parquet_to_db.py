import pandas as pd
import psycopg2
from psycopg2.extras import execute_values

file1_path = r"C:\Users\guler\OneDrive\Masaüstü\YAZILIM MÜHENDİSLİ\stajyer_tavsiye_sistem.parquet"
file2_path = r"C:\Users\guler\OneDrive\Masaüstü\YAZILIM MÜHENDİSLİ\stajyer_tavsiye_sistem2.parquet"

print("Parquet dosyaları okunuyor...")
df1 = pd.read_parquet(file1_path) # Katalog (UrunId, Urun, KtgAdi, Barkod)
df2 = pd.read_parquet(file2_path) # Sipariş/Metrik (ProductId, Barcode, SoldCount vb.)

print(f"Katalog Ürün Sayısı: {len(df1)}, Etkileşim Satır Sayısı: {len(df2)}")

# 1. Adım: İkinci dosyayı hem Barcode hem de ProductId üzerinden ayrı ayrı özetleyelim (Aggregation)
print("İkinci dosya Barkod bazında özetleniyor...")
df2_by_barcode = df2.dropna(subset=['Barcode']).groupby('Barcode').agg({
    'SoldCount': 'sum',
    'ReviewCount': 'sum',
    'TotalRating': 'mean',
    'FavCount': 'sum',
    'Quantity': 'sum'
}).reset_index()

print("İkinci dosya ProductId bazında özetleniyor...")
df2_by_product = df2.dropna(subset=['ProductId']).groupby('ProductId').agg({
    'SoldCount': 'sum',
    'ReviewCount': 'sum',
    'TotalRating': 'mean',
    'FavCount': 'sum',
    'Quantity': 'sum'
}).reset_index()

# 2. Adım: Önce Barkod ile Left Join yapalım
print("1. Aşama: Barkod üzerinden eşleştirme yapılıyor...")
merged_on_barcode = pd.merge(df1, df2_by_barcode, left_on='Barkod', right_on='Barcode', how='left')

# Barkod üzerinden eşleşenler (SoldCount değeri dolu olanlar)
matched_barcode = merged_on_barcode[merged_on_barcode['SoldCount'].notna()].copy()

# Barkod üzerinden eşleşmeyenler (SoldCount boş olanlar)
unmatched = merged_on_barcode[merged_on_barcode['SoldCount'].isna()].drop(
    columns=['SoldCount', 'ReviewCount', 'TotalRating', 'FavCount', 'Quantity', 'Barcode'], errors='ignore'
).copy()

print(f"Barkod ile eşleşen ürün sayısı: {len(matched_barcode)}")
print(f"Eşleşmeyen ve ProductId ile aranacak ürün sayısı: {len(unmatched)}")

# 3. Adım: Eşleşmeyenler için UrunId -> ProductId üzerinden 2. Aşama (Fallback) eşleşme yapalım
print("2. Aşama: Barkod bulunamayanlar için ProductId üzerinden eşleştirme yapılıyor...")
matched_by_product = pd.merge(unmatched, df2_by_product, left_on='UrunId', right_on='ProductId', how='left')

# 4. Adım: Her iki grubu tekrar alt alta birleştirelim (Böylece hiçbir ürün kaybolmaz!)
final_df = pd.concat([matched_barcode, matched_by_product], ignore_index=True)
total_rows = len(final_df)
print(f"\nToplam {total_rows} ürün (tüm katalog) eksiksiz olarak hazırlandı. Veritabanına aktarılıyor...\n")

try:
    connection = psycopg2.connect(
        host="localhost",
        database="stok_db",
        user="postgres",
        password="postgres",
        port="5432"
    )
    cursor = connection.cursor()

    # Eski verileri temizle
    print("Veritabanındaki eski veriler temizleniyor...")
    cursor.execute("TRUNCATE TABLE products RESTART IDENTITY CASCADE;")
    connection.commit()

    insert_query = """
        INSERT INTO products (name, category, stock, price)
        VALUES %s
    """

    chunk_size = 50000
    for i in range(0, total_rows, chunk_size):
        chunk = final_df.iloc[i:i + chunk_size]
        data_tuples = []
        
        for row in chunk.itertuples(index=False):
            row_dict = row._asdict()
            
            name = str(row_dict.get('Urun', 'Staj Ürünü'))
            if not name or name == 'nan':
                name = 'Staj Ürünü'
                
            category = str(row_dict.get('KtgAdi', 'Genel'))
            if not category or category == 'nan':
                category = 'Genel'

            # Eğer metrik dosyasından (barkod veya productid ile) veri geldiyse onları kullanalım, yoksa varsayılan verelim
            sold = row_dict.get('SoldCount', 50)
            stock = int(sold) if pd.notna(sold) and sold > 0 else 100

            rating = row_dict.get('TotalRating', 4.0)
            price = float(rating) * 25.0 if pd.notna(rating) and rating > 0 else 99.90

            data_tuples.append((name, category, stock, price))

        execute_values(cursor, insert_query, data_tuples, page_size=10000)
        connection.commit()
        print(f"İlerleme: {min(i + chunk_size, total_rows)} / {total_rows} ürün aktarıldı...")

    print("\n🎉 Akıllı Fallback birleştirmesiyle tüm ürünler başarıyla PostgreSQL veritabanına aktarıldı!")

except Exception as error:
    print("❌ Hata oluştu:", error)

finally:
    if 'connection' in locals() and connection:
        cursor.close()
        connection.close()