package processing

import (
	"encoding/json"
	"time"

	utils "github.com/diadata-org/decentral-feeder/pkg/utils"
	models "github.com/diadata-org/diaprotocol/pkg/models"
)

// LastPrice returns the price of the latest trade with quotetoken @asset.
func LastPrice(trades []models.Trade, asset models.Asset, USDPrice bool) (lastPrice float64, timestamp time.Time, err error) {

	var basetoken models.Asset

	for _, trade := range trades {
		if trade.QuoteToken.Address == asset.Address && trade.QuoteToken.Blockchain == asset.Blockchain {
			if trade.Time.After(timestamp) {
				timestamp = trade.Time
				lastPrice = trade.Price
				basetoken = trade.BaseToken
			}
		}
	}

	// Fetch USD price of basetoken from DIA API.
	if USDPrice {
		type assetQuotation struct {
			Price  float64 `json:"Price"`
			Volume float64 `json:"VolumeYesterdayUSD"`
		}
		var (
			response []byte
			aq       assetQuotation
		)

		baseString := "https://api.diadata.org/v1/assetQuotation/" + basetoken.Blockchain + "/" + basetoken.Address
		response, _, err = utils.GetRequest(baseString)
		if err != nil {
			return
		}
		err = json.Unmarshal(response, &aq)
		if err != nil {
			return
		}
		lastPrice *= aq.Price
	}
	return
}
