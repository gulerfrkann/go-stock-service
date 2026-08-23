package marketplace

import (
	"context"
	"log"
	"time"
)

// MarketplaceAdapter tüm dış pazaryerlerinin uygulaması gereken standart arayüzdür.
type MarketplaceAdapter interface {
	GetPlatformName() string
	SyncStock(ctx context.Context, productID uint, remainingStock int) error
}

// HepsiburadaAdapter Hepsiburada API mock entegrasyonu
type HepsiburadaAdapter struct {
	APIKey string
}

func NewHepsiburadaAdapter(apiKey string) *HepsiburadaAdapter {
	return &HepsiburadaAdapter{APIKey: apiKey}
}

func (h *HepsiburadaAdapter) GetPlatformName() string {
	return "Hepsiburada"
}

func (h *HepsiburadaAdapter) SyncStock(ctx context.Context, productID uint, remainingStock int) error {
	// Gerçek ortamda burası Hepsiburada Merchant API'ye HTTP POST/PUT isteği atar
	time.Sleep(50 * time.Millisecond) // Simüle edilmiş ağ gecikmesi
	log.Printf("[Hepsiburada API] Ürün ID: %d için stok %d olarak güncellendi (Key: %s)", productID, remainingStock, h.APIKey)
	return nil
}

// TrendyolAdapter Trendyol API mock entegrasyonu
type TrendyolAdapter struct {
	SupplierID string
}

func NewTrendyolAdapter(supplierID string) *TrendyolAdapter {
	return &TrendyolAdapter{SupplierID: supplierID}
}

func (t *TrendyolAdapter) GetPlatformName() string {
	return "Trendyol"
}

func (t *TrendyolAdapter) SyncStock(ctx context.Context, productID uint, remainingStock int) error {
	// Gerçek ortamda burası Trendyol Supplier API'ye HTTP POST isteği atar
	time.Sleep(50 * time.Millisecond)
	log.Printf("[Trendyol API] Ürün ID: %d için stok %d olarak güncellendi (Tedarikçi: %s)", productID, remainingStock, t.SupplierID)
	return nil
}

// SyncManager tüm aktif platformlara paralel senkronizasyon dağıtır
type SyncManager struct {
	adapters []MarketplaceAdapter
}

func NewSyncManager(adapters ...MarketplaceAdapter) *SyncManager {
	return &SyncManager{adapters: adapters}
}

func (m *SyncManager) SyncAll(ctx context.Context, productID uint, remainingStock int, sourcePlatform string) error {
	for _, adapter := range m.adapters {
		// Siparişin geldiği kaynak platform dışındaki diğer platformları güncelle
		if adapter.GetPlatformName() == sourcePlatform {
			continue
		}

		go func(ad MarketplaceAdapter) {
			if err := ad.SyncStock(ctx, productID, remainingStock); err != nil {
				log.Printf("❌ [%s] Stok senkronizasyon hatası: %v", ad.GetPlatformName(), err)
			}
		}(adapter)
	}
	return nil
}