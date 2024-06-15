package scrapers

import (
	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/sirupsen/logrus"
)

const (
	BINANCE_EXCHANGE      = "Binance"
	CRYPTODOTCOM_EXCHANGE = "Crypto.com"
	GATEIO_EXCHANGE       = "GateIO"
	KUCOIN_EXCHANGE       = "KuCoin"

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
	Exchanges[CRYPTODOTCOM_EXCHANGE] = models.Exchange{Name: CRYPTODOTCOM_EXCHANGE, Centralized: true}
	Exchanges[GATEIO_EXCHANGE] = models.Exchange{Name: GATEIO_EXCHANGE, Centralized: true}
	Exchanges[KUCOIN_EXCHANGE] = models.Exchange{Name: KUCOIN_EXCHANGE, Centralized: true}

	Exchanges[UNISWAPV2_EXCHANGE] = models.Exchange{Name: UNISWAPV2_EXCHANGE, Centralized: false, Blockchain: ETHEREUM}
}
