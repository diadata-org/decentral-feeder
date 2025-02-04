package scrapers

import (
	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
	"github.com/sirupsen/logrus"
)

const (
	BINANCE_EXCHANGE      = "Binance"
	COINBASE_EXCHANGE     = "CoinBase"
	CRYPTODOTCOM_EXCHANGE = "Crypto.com"
	GATEIO_EXCHANGE       = "GateIO"
	KRAKEN_EXCHANGE       = "Kraken"
	KUCOIN_EXCHANGE       = "KuCoin"

	UNISWAPV2_EXCHANGE = "UniswapV2"
	UNISWAP_SIMULATION = "UniswapSimulation"
)

var (
	Exchanges = make(map[string]models.Exchange)
	log       *logrus.Logger
)

func init() {

	Exchanges[BINANCE_EXCHANGE] = models.Exchange{Name: BINANCE_EXCHANGE, Centralized: true}
	Exchanges[COINBASE_EXCHANGE] = models.Exchange{Name: COINBASE_EXCHANGE, Centralized: true}
	Exchanges[CRYPTODOTCOM_EXCHANGE] = models.Exchange{Name: CRYPTODOTCOM_EXCHANGE, Centralized: true}
	Exchanges[GATEIO_EXCHANGE] = models.Exchange{Name: GATEIO_EXCHANGE, Centralized: true}
	Exchanges[KRAKEN_EXCHANGE] = models.Exchange{Name: KRAKEN_EXCHANGE, Centralized: true}
	Exchanges[KUCOIN_EXCHANGE] = models.Exchange{Name: KUCOIN_EXCHANGE, Centralized: true}

	Exchanges[UNISWAP_SIMULATION] = models.Exchange{Name: UNISWAP_SIMULATION, Centralized: false, Simulation: true, Blockchain: utils.ETHEREUM}
	Exchanges[UNISWAPV2_EXCHANGE] = models.Exchange{Name: UNISWAPV2_EXCHANGE, Centralized: false, Blockchain: utils.ETHEREUM}

	log = logrus.New()
	loglevel, err := logrus.ParseLevel(utils.Getenv("LOG_LEVEL_SCRAPERS", "info"))
	if err != nil {
		log.Errorf("Parse log level: %v.", err)
	}
	log.SetLevel(loglevel)

}
