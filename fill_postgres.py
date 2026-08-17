import pandas as pd
import psycopg2
from psycopg2.extras import execute_values
from datetime import datetime
from tqdm import tqdm

print("1/2 - Parquet dosyaları okunuyor ve temizleniyor...")
df_urun = pd.read_parquet('stajyer_tavsiye_sistem.parquet')
df_siparis = pd.read_parquet('stajyer_tavsiye_sistem2.parquet')

# Barkod temizliği
df_urun['Barkod_clean'] = df_urun['Barkod'].astype(str).str.replace(r'\.0$', '', regex=True).str.strip()
df_siparis['Barcode_clean'] = df_siparis['Barcode'].astype(str).str.replace(r'\.0$', '', regex=True).str.strip()

df_urun = df_urun[df_urun['Barkod_clean'].str.len() > 3]
df_siparis = df_siparis[df_siparis['Barcode_clean'].str.len() > 3]

df_siparis_ozet = df_siparis.groupby('Barcode_clean').agg({'Quantity': 'sum', 'SoldCount': 'max'}).reset_index()

df_merged = pd.merge(df_urun, df_siparis_ozet, left_on='Barkod_clean', right_on='Barcode_clean', how='left')
df_merged['Quantity'] = df_merged['Quantity'].fillna(0)
df_merged['SoldCount'] = df_merged['SoldCount'].fillna(0)
df_clean = df_merged.dropna(subset=['Urun']).reset_index(drop=True).copy()

# ERP / Muhasebe / Fatura Çöplerini Ayıklama
junk_patterns = [
    'İSKONTO', 'ISKONTO', 'FARK', 'BEDEL', 'KOMİSYON', 'KOMISYON', 
    'HİZMET', 'HIZMET', 'KAMPANYA', 'NAKLİYE', 'NAKLIYE', 'İADE', 'IADE',
    'FATURA', 'GİDER', 'GIDER', 'MASRAF', 'YANSITMA', 'DÜZELTME', 'DUZELTME'
]
regex_junk = '|'.join(junk_patterns)
df_clean = df_clean[~df_clean['Urun'].str.upper().str.contains(regex_junk, na=False)].reset_index(drop=True)

print(f"✅ Temizlenmiş toplam satılabilir ürün: {len(df_clean):,}")
print("2/2 - PostgreSQL'e veriler aktarılıyor...")

conn = psycopg2.connect(host="localhost", port="5432", database="stok_db", user="postgres", password="postgres")
cursor = conn.cursor()

cursor.execute("TRUNCATE TABLE products RESTART IDENTITY CASCADE;")
conn.commit()

batch_size = 2000
now = datetime.now()

for i in tqdm(range(0, len(df_clean), batch_size), desc="DB Aktarım"):
    batch_df = df_clean.iloc[i:i+batch_size]
    records = []
    for _, row in batch_df.iterrows():
        urun_adi = str(row['Urun']).strip()
        kategori_val = str(row['KtgAdi']).strip() if pd.notna(row['KtgAdi']) and str(row['KtgAdi']).strip() != '' else "Genel"
        satilan_adet = int(row['Quantity']) if row['Quantity'] > 0 else 10
        stok = max(satilan_adet * 2, 30)
        fiyat = round(100.0 + (satilan_adet * 0.8), 2)
        records.append((urun_adi, kategori_val, stok, fiyat, now, now))
        
    cursor.execute("BEGIN;")
    insert_product_query = "INSERT INTO products (name, category, stock, price, created_at, updated_at) VALUES %s;"
    execute_values(cursor, insert_product_query, records)
    conn.commit()

print("✅ PostgreSQL temiz veri aktarımı tamamlandı!")
cursor.close()
conn.close()