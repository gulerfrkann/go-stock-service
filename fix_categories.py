import pandas as pd
import psycopg2
from psycopg2.extras import execute_batch

print("1/3 - Parquet dosyası okunuyor...")
df_urun = pd.read_parquet('stajyer_tavsiye_sistem.parquet')
df_clean = df_urun.dropna(subset=['Urun']).reset_index(drop=True)

print("2/3 - PostgreSQL bağlantısı kuruluyor...")
conn = psycopg2.connect(host="localhost", port="5432", database="stok_db", user="postgres", password="postgres")
cursor = conn.cursor()

print("3/3 - Kategoriler güncelleniyor...")
updates = []
for idx, row in df_clean.iterrows():
    prod_id = idx + 1
    
    # KtgAdi varsa onu al, yoksa Ktg2Adi'ye bak
    ktg = str(row.get('KtgAdi', '')).strip()
    if not ktg or ktg.lower() == 'nan' or ktg.lower() == 'none':
        ktg = str(row.get('Ktg2Adi', 'Genel')).strip()
    
    if not ktg or ktg.lower() == 'nan':
        ktg = 'Genel'
        
    updates.append((ktg, prod_id))

query = "UPDATE products SET category = %s WHERE id = %s;"
execute_batch(cursor, query, updates, page_size=5000)

conn.commit()
cursor.close()
conn.close()

print("🚀 Kategoriler (KtgAdi) başarıyla PostgreSQL'e aktarıldı!")