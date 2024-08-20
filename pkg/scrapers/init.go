package scrapers

import (
	"strconv"

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
	Simulation         = "Simulation"
)

var (
	Exchanges = make(map[string]models.Exchange)
	log       *logrus.Logger
)

func init() {
	var err error
	log = logrus.New()
	Exchanges[BINANCE_EXCHANGE] = models.Exchange{Name: BINANCE_EXCHANGE, Centralized: true}
	Exchanges[COINBASE_EXCHANGE] = models.Exchange{Name: COINBASE_EXCHANGE, Centralized: true}
	Exchanges[CRYPTODOTCOM_EXCHANGE] = models.Exchange{Name: CRYPTODOTCOM_EXCHANGE, Centralized: true}
	Exchanges[GATEIO_EXCHANGE] = models.Exchange{Name: GATEIO_EXCHANGE, Centralized: true}
	Exchanges[KRAKEN_EXCHANGE] = models.Exchange{Name: KRAKEN_EXCHANGE, Centralized: true}
	Exchanges[KUCOIN_EXCHANGE] = models.Exchange{Name: KUCOIN_EXCHANGE, Centralized: true}

	Exchanges[UNISWAPV2_EXCHANGE] = models.Exchange{Name: UNISWAPV2_EXCHANGE, Centralized: false, Blockchain: utils.ETHEREUM}

	binanceWatchdogDelay, err = strconv.ParseInt(utils.Getenv("BINANCE_WATCHDOGDELAY", "60"), 10, 64)
	if err != nil {
		log.Error("Parse BINANCE_WATCHDOGDELAY: ", err)
	}
	coinbaseWatchdogDelay, err = strconv.ParseInt(utils.Getenv("COINBASE_WATCHDOGDELAY", "60"), 10, 64)
	if err != nil {
		log.Error("Parse COINBASE_WATCHDOGDELAY: ", err)
	}
	cryptoDotComWatchdogDelay, err = strconv.ParseInt(utils.Getenv("CRYPTODOTOM_WATCHDOGDELAY", "60"), 10, 64)
	if err != nil {
		log.Error("Parse CRYPTODOTOM_WATCHDOGDELAY: ", err)
	}
	gateIOWatchdogDelay, err = strconv.ParseInt(utils.Getenv("GATEIO_WATCHDOGDELAY", "60"), 10, 64)
	if err != nil {
		log.Error("Parse GATEIO_WATCHDOGDELAY: ", err)
	}
	krakenWatchdogDelay, err = strconv.ParseInt(utils.Getenv("KRAKEN_WATCHDOGDELAY", "60"), 10, 64)
	if err != nil {
		log.Error("Parse KRAKEN_WATCHDOGDELAY: ", err)
	}
	kucoinWatchdogDelay, err = strconv.ParseInt(utils.Getenv("KUCOIN_WATCHDOGDELAY", "60"), 10, 64)
	if err != nil {
		log.Error("Parse KUCOIN_WATCHDOGDELAY: ", err)
	}

}
