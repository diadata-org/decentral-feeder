package models

import "time"

// AssetQuotation is the most recent price point information on an asset.
type AssetQuotation struct {
	Asset  Asset     `json:"Asset"`
	Price  float64   `json:"Price"`
	Source string    `json:"Source"`
	Time   time.Time `json:"Time"`
}
