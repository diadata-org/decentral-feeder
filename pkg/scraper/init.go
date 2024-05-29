package scrapers

import (
	models "github.com/diadata-org/diaprotocol/pkg/models"
	"github.com/sirupsen/logrus"
)

const (
	BINANCE_EXCHANGE   = "Binance"
	UNISWAPV2_EXCHANGE = "UniswapV2"

	ETHEREUM = "Ethereum"
)

var (
	Exchanges = make(map[string]models.Exchange)
	log       *logrus.Logger
)

func init() {
	log = logrus.New()
	Exchanges[BINANCE_EXCHANGE] = models.Exchange{Name: BINANCE_EXCHANGE, Centralized: true}
	Exchanges[UNISWAPV2_EXCHANGE] = models.Exchange{Name: UNISWAPV2_EXCHANGE, Centralized: false, Blockchain: ETHEREUM}
}
