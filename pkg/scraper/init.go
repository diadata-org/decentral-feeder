package scrapers

import (
	models "github.com/diadata-org/diaprotocol/pkg/models"
	"github.com/sirupsen/logrus"
)

const (
	BINANCE_EXCHANGE = "Binance"
)

var (
	Exchanges = make(map[string]models.Exchange)
	log       *logrus.Logger
)

func init() {
	log = logrus.New()
	Exchanges[BINANCE_EXCHANGE] = models.Exchange{Name: BINANCE_EXCHANGE, Centralized: true}
}
