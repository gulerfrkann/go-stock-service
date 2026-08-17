import pandas as pd
import numpy as np
import psycopg2
from psycopg2.extras import execute_values
from sentence_transformers import SentenceTransformer
from qdrant_client import QdrantClient
from qdrant_client.http import models
from datetime import datetime
import re
from tqdm import tqdm

print("1/6 - Parquet dosyaları yükleniyor...")
df_urun = pd.read_parquet('stajyer_tavsiye_sistem.parquet')
df_siparis = pd.read_parquet('stajyer_tavsiye_sistem2.parquet')

print("2/6 - Barkodlar temizleniyor...")
df_urun['Barkod_clean'] = df_urun['Barkod'].astype(str).str.replace(r'\.0$', '', regex=True).str.strip()
df_siparis['Barcode_clean'] = df_siparis['Barcode'].astype(str).str.replace(r'\.0$', '', regex=True).str.strip()

df_urun = df_urun[df_urun['Barkod_clean'].str.len() > 3]
df_siparis = df_siparis[df_siparis['Barcode_clean'].str.len() > 3]

print("3/6 - Sipariş verileri özetleniyor...")
df_siparis_ozet = df_siparis.groupby('Barcode_clean').agg({
    'Quantity': 'sum',
    'SoldCount': 'max'
}).reset_index()

print("4/6 - Veriler birleştiriliyor (TÜM KATALOG - LEFT JOIN)...")
df_merged = pd.merge(df_urun, df_siparis_ozet, left_on='Barkod_clean', right_on='Barcode_clean', how='left')
df_merged['Quantity'] = df_merged['Quantity'].fillna(0)
df_merged['SoldCount'] = df_merged['SoldCount'].fillna(0)
df_clean = df_merged.dropna(subset=['Urun']).reset_index(drop=True).copy()

print(f"✅ Toplam ürün sayısı: {len(df_clean):,}")

print("5/6 - all-MiniLM-L6-v2 modeli ile embedding üretiliyor ve Qdrant'a yükleniyor...")
model = SentenceTransformer('all-MiniLM-L6-v2')

client = QdrantClient(path="./qdrant_local_db")
collection_name = "products_embeddings"

if client.collection_exists(collection_name):
    client.delete_collection(collection_name)

client.create_collection(
    collection_name=collection_name,
    vectors_config=models.VectorParams(
        size=384, 
        distance=models.Distance.COSINE
    )
)

print("Veriler parçalar halinde (batch) Qdrant ve PostgreSQL'e aktarılıyor...")
conn_params = {"host": "localhost", "port": "5432", "database": "stok_db", "user": "postgres", "password": "postgres"}
conn = psycopg2.connect(**conn_params)
cursor = conn.cursor()

cursor.execute("TRUNCATE TABLE product_recommendations CASCADE;")
cursor.execute("TRUNCATE TABLE products RESTART IDENTITY CASCADE;")
conn.commit()

batch_size = 2000
now = datetime.now()

for i in tqdm(range(0, len(df_clean), batch_size), desc="Embedding & DB Aktarım"):
    batch_df = df_clean.iloc[i:i+batch_size]
    
    texts = (batch_df['Urun'].fillna('') + ' ' + batch_df['UrunAciklamasi'].fillna('')).apply(lambda x: re.sub(r'<[^>]+>', '', x)).tolist()
    
    embeddings = model.encode(texts, show_progress_bar=False, batch_size=64)
    
    records = []
    for _, row in batch_df.iterrows():
        urun_adi = str(row['Urun']).strip()
        satilan_adet = int(row['Quantity']) if row['Quantity'] > 0 else 10
        stok = max(satilan_adet * 2, 30)
        fiyat = round(100.0 + (satilan_adet * 0.8), 2)
        records.append((urun_adi, stok, fiyat, now, now))
        
    cursor.execute("BEGIN;")
    insert_product_query = "INSERT INTO products (name, stock, price, created_at, updated_at) VALUES %s RETURNING id;"
    execute_values(cursor, insert_product_query, records)
    product_ids = [row[0] for row in cursor.fetchall()]
    
    points = []
    for idx, prod_id in enumerate(product_ids):
        global_idx = i + idx
        points.append(
            models.PointStruct(
                id=global_idx,
                vector=embeddings[idx].tolist(),
                payload={"product_id": prod_id}
            )
        )
    
    client.upsert(collection_name=collection_name, points=points)
    conn.commit()

print("🚀 MÜKEMMEL! Tüm katalog all-MiniLM-L6-v2 ve Qdrant altyapısıyla başarıyla işlendi.")
cursor.close()
conn.close()