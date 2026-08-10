import pandas as pd
import numpy as np
import psycopg2
from psycopg2.extras import execute_values
from sklearn.feature_extraction.text import TfidfVectorizer
from datetime import datetime
import re

print("1/6 - Parquet dosyaları yükleniyor...")
df_urun = pd.read_parquet('stajyer_tavsiye_sistem.parquet')
df_siparis = pd.read_parquet('stajyer_tavsiye_sistem2.parquet')

print("2/6 - Barkodlar temizleniyor ve formatlanıyor...")
# Barkod alanlarını string'e çevirip .0 ve boşlukları temizliyoruz
df_urun['Barkod_clean'] = df_urun['Barkod'].astype(str).str.replace(r'\.0$', '', regex=True).str.strip()
df_siparis['Barcode_clean'] = df_siparis['Barcode'].astype(str).str.replace(r'\.0$', '', regex=True).str.strip()

# Geçersiz/boş barkodları filtresi
df_urun = df_urun[df_urun['Barkod_clean'].str.len() > 3]
df_siparis = df_siparis[df_siparis['Barcode_clean'].str.len() > 3]

print("3/6 - Sipariş verileri barkod bazında özetleniyor...")
df_siparis_ozet = df_siparis.groupby('Barcode_clean').agg({
    'Quantity': 'sum',
    'SoldCount': 'max'
}).reset_index()

print("4/6 - Barkodlar üzerinden iki veri seti birleştiriliyor (INNER JOIN)...")
df_merged = pd.merge(df_urun, df_siparis_ozet, left_on='Barkod_clean', right_on='Barcode_clean', how='inner')
df_clean = df_merged.dropna(subset=['Urun']).head(5000).copy()

print(f" Barkod eşleşmesi sağlanan toplam ürün sayısı: {len(df_clean):,}")

print("5/6 - Ürün metinleri üzerinde TF-IDF analizi yapılıyor...")
# Ürün adı ve açıklamasını birleştirip metin analizi yapıyoruz
df_clean['text_for_tfidf'] = df_clean['Urun'].fillna('') + ' ' + df_clean['UrunAciklamasi'].fillna('')
# HTML etiketlerini temizle
df_clean['text_for_tfidf'] = df_clean['text_for_tfidf'].apply(lambda x: re.sub(r'<[^>]+>', '', x))

# TF-IDF Vectorizer
tfidf = TfidfVectorizer(max_features=500, stop_words=None) # Türkçe stop words gerekirse eklenebilir
tfidf_matrix = tfidf.fit_transform(df_clean['text_for_tfidf'])
feature_names = np.array(tfidf.get_feature_names_out())

# Her ürün için yüksek skorlu top 3 TF-IDF kelimesini bulup ürün adına etiket olarak ekliyoruz
extracted_keywords = []
for row_idx in range(tfidf_matrix.shape[0]):
    row_data = tfidf_matrix.getrow(row_idx).toarray().flatten()
    top_indices = row_data.argsort()[-3:][::-1]
    top_words = [feature_names[i] for i in top_indices if row_data[i] > 0]
    extracted_keywords.append(" ".join(top_words))

df_clean['keywords'] = extracted_keywords

print("6/6 - PostgreSQL veritabanına aktarılıyor...")
conn_params = {
    "host": "localhost",
    "port": "5432",
    "database": "stok_db",
    "user": "postgres",
    "password": "postgres"
}

try:
    conn = psycopg2.connect(**conn_params)
    cursor = conn.cursor()
    
    now = datetime.now()
    records = []
    
    for index, row in df_clean.iterrows():
        # Ürün adının yanına TF-IDF ile çıkardığımız anahtar etiketleri parantez içinde ekliyoruz
        keywords_str = f" [{row['keywords']}]" if row['keywords'] else ""
        urun_adi = f"{str(row['Urun']).strip()}{keywords_str}"
        
        satilan_adet = int(row['Quantity']) if pd.notnull(row['Quantity']) and row['Quantity'] > 0 else 10
        stok = max(satilan_adet * 2, 30)
        fiyat = round(100.0 + (satilan_adet * 0.8), 2)
        
        records.append((urun_adi, stok, fiyat, now, now))

    insert_query = """
        INSERT INTO products (name, stock, price, created_at, updated_at)
        VALUES %s;
    """
    
    execute_values(cursor, insert_query, records)
    conn.commit()
    
    print(" MÜKEMMEL! Barkod eşleşmeli ve TF-IDF etiketli tüm ürünler veritabanına eklendi.")

except Exception as e:
    print(f" Hata oluştu: {e}")

finally:
    if 'cursor' in locals():
        cursor.close()
    if 'conn' in locals():
        conn.close()