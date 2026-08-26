import pandas as pd
import psycopg2
import redis
import json
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics.pairwise import cosine_similarity
from tqdm import tqdm

print("1/3 - Veritabanından ürünler okunuyor...")
conn = psycopg2.connect(host="localhost", port="5432", database="stok_db", user="postgres", password="postgres")
cursor = conn.cursor()

cursor.execute("SELECT id, name, category FROM products ORDER BY id ASC LIMIT 50000;")
rows = cursor.fetchall()
cursor.close()
conn.close()

df = pd.DataFrame(rows, columns=['id', 'name', 'category'])
print(f"✅ {len(df):,} ürün yüklendi.")

print("2/3 - Kategori içi izole TF-IDF ve Benzerlik hesaplanıyor...")
r = redis.Redis(host='localhost', port=6380, db=0)
pipe = r.pipeline()
count = 0

for category_name, group in df.groupby('category'):
    # Kategoride tek ürün varsa veya kategori adı boşsa geç
    if len(group) < 2 or not category_name:
        continue
    
    group = group.reset_index(drop=True)
    
    tfidf = TfidfVectorizer(max_features=5000, stop_words='english')
    tfidf_matrix = tfidf.fit_transform(group['name'].fillna(''))
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
            raw_score = float(sim_scores[rel_idx])
            
            # DİKKAT: Sahte skor ataması kaldırıldı! 
            # Sadece gerçekten benzerlik oranı 0.10'un üzerinde olanlar önerilecek.
            if raw_score < 0.10:
                continue
                
            final_score = round(raw_score, 2)
            
            recs.append({
                "product_id": int(candidate['id']),
                "name": str(candidate['name']),
                "category": str(candidate['category']),
                "score": final_score,
                "reason": f"{category_name} Kategorisi İçi NLP Benzerliği"
            })
            
            if len(recs) >= 3:
                break
        
        # Eğer yeterli benzer ürün bulunamadıysa (boş kalmasın diye) Redis'e yazmıyoruz 
        # böylece servis fallback yerine mantıklı filtreleme yapabiliyor.
        if len(recs) > 0:
            redis_key = f"recommendations:product:{prod_id}"
            pipe.set(redis_key, json.dumps(recs))
            count += 1
            
            if count % 1000 == 0:
                pipe.execute()

pipe.execute()
print(f"🚀 BAŞARILI! {count:,} adet ürüne gerçek ve kategoriye özel TF-IDF skorları Redis'e aktarıldı.")