from qdrant_client import QdrantClient
from qdrant_client.http import models
from tqdm import tqdm

print("Bağlantılar kuruluyor...")
local_client = QdrantClient(path="./qdrant_local_db")
docker_client = QdrantClient(url="http://localhost:6333")
collection_name = "products_embeddings"

if docker_client.collection_exists(collection_name):
    docker_client.delete_collection(collection_name)

docker_client.create_collection(
    collection_name=collection_name,
    vectors_config=models.VectorParams(size=384, distance=models.Distance.COSINE)
)

total_count = local_client.count(collection_name=collection_name).count
print(f"Toplam {total_count} vektör Docker'a aktarılıyor. Bu işlem 1-2 dakika sürer...")

offset = None
with tqdm(total=total_count, desc="Qdrant Aktarımı") as pbar:
    while True:
        records, next_page = local_client.scroll(
            collection_name=collection_name, limit=1000, offset=offset, with_vectors=True, with_payload=True
        )
        if not records: break
        
        # HATA DÜZELTİLDİ: Record formatını PointStruct formatına dönüştürüyoruz
        points = [
            models.PointStruct(id=r.id, vector=r.vector, payload=r.payload)
            for r in records
        ]
        
        docker_client.upsert(collection_name=collection_name, points=points)
        pbar.update(len(records))
        offset = next_page
        if offset is None: break

print("🚀 MÜKEMMEL! 3.5 saatlik veri, başarıyla Docker sunucusuna aktarıldı!")