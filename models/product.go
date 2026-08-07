package models

type Product struct {
	ID    uint    `json:"id" gorm:"primaryKey"`
	Ad    string  `json:"ad"`
	Fiyat float64 `json:"fiyat"`
	Stok  int     `json:"stok"`
}

type ReduceStockRequest struct {
	Adet int `json:"adet"`
}