package filters

import (
	"encoding/json"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	utils "github.com/diadata-org/decentral-feeder/pkg/utils"
	log "github.com/sirupsen/logrus"
)

// LastPrice returns the price of the latest trade.
func LastPrice(trades []models.Trade, USDPrice bool) (lastPrice float64, timestamp time.Time, err error) {

	lastTrade := models.GetLastTrade(trades)
	timestamp = lastTrade.Time
	if lastTrade.BaseToken.Blockchain == "Fiat" && lastTrade.BaseToken.Address == "840" {
		lastPrice = lastTrade.Price
		return
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

		baseString := "https://api.diadata.org/v1/assetQuotation/" + lastTrade.BaseToken.Blockchain + "/" + lastTrade.BaseToken.Address
		response, _, err = utils.GetRequest(baseString)
		if err != nil {
			log.Debugf("GetRequest for %s on %s", lastTrade.BaseToken.Address, lastTrade.BaseToken.Blockchain)
			return
		}
		err = json.Unmarshal(response, &aq)
		if err != nil {
			return
		}
		lastPrice = aq.Price * lastTrade.Price

	} else {
		lastPrice = lastTrade.Price
	}

	return
}
