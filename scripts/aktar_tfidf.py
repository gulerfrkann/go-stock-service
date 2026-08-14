import pandas as pd
import numpy as np
import psycopg2
from psycopg2.extras import execute_values
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics.pairwise import cosine_similarity
from datetime import datetime
import re

print("1/6 - Parquet dosyaları yükleniyor...")
df_urun = pd.read_parquet('stajyer_tavsiye_sistem.parquet')
df_siparis = pd.read_parquet('stajyer_tavsiye_sistem2.parquet')

print("2/6 - Barkodlar temizleniyor ve formatlanıyor...")
df_urun['Barkod_clean'] = df_urun['Barkod'].astype(str).str.replace(r'\.0$', '', regex=True).str.strip()
df_siparis['Barcode_clean'] = df_siparis['Barcode'].astype(str).str.replace(r'\.0$', '', regex=True).str.strip()

df_urun = df_urun[df_urun['Barkod_clean'].str.len() > 3]
df_siparis = df_siparis[df_siparis['Barcode_clean'].str.len() > 3]

print("3/6 - Sipariş verileri barkod bazında özetleniyor...")
df_siparis_ozet = df_siparis.groupby('Barcode_clean').agg({
    'Quantity': 'sum',
    'SoldCount': 'max'
}).reset_index()

print("4/6 - Barkodlar üzerinden iki veri seti birleştiriliyor (INNER JOIN)...")
df_merged = pd.merge(df_urun, df_siparis_ozet, left_on='Barkod_clean', right_on='Barcode_clean', how='inner')
df_clean = df_merged.dropna(subset=['Urun']).copy() # .head(5000)'i kaldırdık

print(f"✅ Barkod eşleşmesi sağlanan toplam ürün sayısı: {len(df_clean):,}")

print("5/6 - Ürün metinleri üzerinde TF-IDF analizi yapılıyor...")
df_clean['text_for_tfidf'] = df_clean['Urun'].fillna('') + ' ' + df_clean['UrunAciklamasi'].fillna('')
df_clean['text_for_tfidf'] = df_clean['text_for_tfidf'].apply(lambda x: re.sub(r'<[^>]+>', '', x))

tfidf = TfidfVectorizer(max_features=500, stop_words=None)
tfidf_matrix = tfidf.fit_transform(df_clean['text_for_tfidf'])
feature_names = np.array(tfidf.get_feature_names_out())

extracted_keywords = []
for row_idx in range(tfidf_matrix.shape[0]):
    row_data = tfidf_matrix.getrow(row_idx).toarray().flatten()
    top_indices = row_data.argsort()[-3:][::-1]
    top_words = [feature_names[i] for i in top_indices if row_data[i] > 0]
    extracted_keywords.append(" ".join(top_words))

df_clean['keywords'] = extracted_keywords

print("5.5/6 - Cosine Similarity (Kosinüs Benzerliği) hesaplanıyor...")
cosine_sim = cosine_similarity(tfidf_matrix, tfidf_matrix)

print("6/6 - PostgreSQL veritabanına ve öneri tablosuna aktarılıyor...")
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
    
    # Eski kayıtları temizle (çakışmayı önlemek için)
    cursor.execute("TRUNCATE TABLE product_recommendations CASCADE;")
    cursor.execute("TRUNCATE TABLE products RESTART IDENTITY CASCADE;")
    
    now = datetime.now()
    records = []
    
    for index, row in df_clean.iterrows():
        keywords_str = f" [{row['keywords']}]" if row['keywords'] else ""
        urun_adi = f"{str(row['Urun']).strip()}{keywords_str}"
        
        satilan_adet = int(row['Quantity']) if pd.notnull(row['Quantity']) and row['Quantity'] > 0 else 10
        stok = max(satilan_adet * 2, 30)
        fiyat = round(100.0 + (satilan_adet * 0.8), 2)
        
        records.append((urun_adi, stok, fiyat, now, now))

    # Ürünleri ekle ve üretilen ID'leri geri al
    insert_product_query = """
        INSERT INTO products (name, stock, price, created_at, updated_at)
        VALUES %s RETURNING id;
    """
    
    product_ids = []
    for i in range(0, len(records), 1000):
        batch = records[i:i+1000]
        execute_values(cursor, insert_product_query, batch)
        batch_ids = [row[0] for row in cursor.fetchall()]
        product_ids.extend(batch_ids)

    conn.commit()
    print(f"📦 {len(product_ids)} ürün veritabanına yazıldı.")

    print("🔗 Benzerlik skorları hesaplanıp öneri tablosuna işleniyor...")
    rec_records = []
    
    for idx, real_product_id in enumerate(product_ids):
        sim_scores = list(enumerate(cosine_sim[idx]))
        sim_scores = sorted(sim_scores, key=lambda x: x[1], reverse=True)
        
        # Kendisi hariç en yüksek skorlu 3 benzer ürünü al
        sim_scores = [s for s in sim_scores if s[0] != idx][:3]
        
        for similar_idx, score in sim_scores:
            recommended_product_id = product_ids[similar_idx]
            reason = "TF-IDF Anlamsal Metin Benzerliği & Cosine Similarity"
            rec_records.append((real_product_id, recommended_product_id, float(score), reason, now))

    insert_rec_query = """
        INSERT INTO product_recommendations (product_id, recommended_product_id, score, reason, created_at)
        VALUES %s;
    """
    
    for i in range(0, len(rec_records), 1000):
        batch = rec_records[i:i+1000]
        execute_values(cursor, insert_rec_query, batch)
        
    conn.commit()
    print("🚀 MÜKEMMEL! TF-IDF benzerlik skorları ve öneriler veritabanına başarıyla işlendi.")

except Exception as e:
    print(f" Hata oluştu: {e}")
    if 'conn' in locals():
        conn.rollback()

finally:
    if 'cursor' in locals():
        cursor.close()
    if 'conn' in locals():
        conn.close()