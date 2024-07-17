package scrapers

import (
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
)

// TO DO: We can think about making a scraper interface with tradesChannel, failoverChannel and normalizePairTicker methods.
type Scraper interface {
	TradesChannel() chan models.Trade
	FailoverChannel() chan string
	// normalizePairTicker()
	Close() error
}

// RunScraper starts a scraper for @exchange.
func RunScraper(exchange string, pairs []models.ExchangePair, pools []models.Pool, tradesChannel chan models.Trade, failoverChannel chan string, wg *sync.WaitGroup) {
	switch exchange {
	case BINANCE_EXCHANGE:
		NewBinanceScraper(pairs, tradesChannel, failoverChannel, wg)
	case COINBASE_EXCHANGE:
		NewCoinBaseScraper(pairs, tradesChannel, wg)
	case CRYPTODOTCOM_EXCHANGE:
		NewCryptoDotComScraper(pairs, tradesChannel, wg)
	case GATEIO_EXCHANGE:
		NewGateIOScraper(pairs, tradesChannel, wg)
	case KRAKEN_EXCHANGE:
		NewKrakenScraper(pairs, tradesChannel, wg)
	case KUCOIN_EXCHANGE:
		NewKuCoinScraper(pairs, tradesChannel, wg)

	case UNISWAPV2_EXCHANGE:
		NewUniswapV2Scraper(pools, tradesChannel, wg)

	}
}

func watchdog(
	failoverChannel chan string,
	lastTradeTime time.Time,
	watchdogDelay int,
	watchdogTicker *time.Ticker,
	exchange string,
) {

	for range watchdogTicker.C {
		log.Info("lastTradeTime: ", lastTradeTime)
		log.Info("timeNow: ", time.Now())
		duration := time.Since(lastTradeTime)
		if duration > time.Duration(watchdogDelayBinance)*time.Second {
			log.Error("failover")
			failoverChannel <- exchange
		}
	}

}
