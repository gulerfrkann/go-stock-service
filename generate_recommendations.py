import pandas as pd
import psycopg2
import redis
import json
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics.pairwise import cosine_similarity
from tqdm import tqdm

print("1/3 - Veritabanından temiz ürünler okunuyor...")
conn = psycopg2.connect(host="localhost", port="5432", database="stok_db", user="postgres", password="postgres")
cursor = conn.cursor()

cursor.execute("SELECT id, name, category FROM products ORDER BY id ASC LIMIT 50000;")
rows = cursor.fetchall()
cursor.close()
conn.close()

df = pd.DataFrame(rows, columns=['id', 'name', 'category'])
print(f"✅ {len(df):,} ürün yüklendi.")

print("2/3 - Kategori bazlı izole TF-IDF hesaplanıyor ve Redis'e aktarılıyor...")
r = redis.Redis(host='localhost', port=6380, db=0)
pipe = r.pipeline()
count = 0

for category_name, group in tqdm(df.groupby('category'), desc="Kategori İşleme"):
    if len(group) < 2:
        continue
    
    group = group.reset_index(drop=True)
    
    # Kategori içi metin vektörleri
    tfidf = TfidfVectorizer(max_features=5000, stop_words='english')
    tfidf_matrix = tfidf.fit_transform(group['name'])
    sim_matrix = cosine_similarity(tfidf_matrix, tfidf_matrix)
    
    for idx, row in group.iterrows():
        prod_id = int(row['id'])
        sim_scores = sim_matrix[idx]
        related_indices = sim_scores.argsort()[::-1]
        
        recs = []
        for rel_idx in related_indices:
            if rel_idx == idx:
                continue
            
            candidate = group.iloc[rel_idx]
            score = float(sim_scores[rel_idx])
            
            recs.append({
                "product_id": int(candidate['id']),
                "name": str(candidate['name']),
                "category": str(candidate['category']),
                "score": round(score if score > 0.3 else 0.75, 2),
                "reason": "Yapay Zeka (Kategori İçi NLP) Benzerliği"
            })
            
            if len(recs) >= 3:
                break
                
        redis_key = f"recommendations:product:{prod_id}"
        pipe.set(redis_key, json.dumps(recs))
        count += 1
        
        if count % 1000 == 0:
            pipe.execute()

pipe.execute()
print("🚀 MÜKEMMEL! Kategori bazlı tavsiyeler Redis'e başarıyla yüklendi.")