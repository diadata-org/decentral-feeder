package filters

import (
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	utils "github.com/diadata-org/decentral-feeder/pkg/utils"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func init() {
	log = logrus.New()
	loglevel, err := logrus.ParseLevel(utils.Getenv("LOG_LEVEL_FILTERS", "info"))
	if err != nil {
		log.Errorf("Parse log level: %v.", err)
	}
	log.SetLevel(loglevel)
}

// LastPrice returns the price of the latest trade.
// If price should be returned in terms of native currency @basePrice should be set to 1.
func LastPrice(trades []models.Trade, basePrice float64) (float64, time.Time) {
	lastTrade := models.GetLastTrade(trades)
	return basePrice * lastTrade.Price, lastTrade.Time
}
