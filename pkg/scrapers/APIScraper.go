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
func RunScraper(
	exchange string,
	pairs []models.ExchangePair,
	pools []models.Pool,
	tradesChannel chan models.Trade,
	failoverChannel chan string,
	wg *sync.WaitGroup,
) {
	switch exchange {
	case BINANCE_EXCHANGE:
		status := NewBinanceScraper(pairs, tradesChannel, failoverChannel, wg)
		if status == "closed" {
			return
		}
	case COINBASE_EXCHANGE:
		NewCoinBaseScraper(pairs, tradesChannel, failoverChannel, wg)
		status := NewCoinBaseScraper(pairs, tradesChannel, failoverChannel, wg)
		if status == "closed" {
			return
		}
	case CRYPTODOTCOM_EXCHANGE:
		status := NewCryptoDotComScraper(pairs, tradesChannel, failoverChannel, wg)
		if status == "closed" {
			return
		}
	case GATEIO_EXCHANGE:
		status := NewGateIOScraper(pairs, tradesChannel, failoverChannel, wg)
		if status == "closed" {
			return
		}
	case KRAKEN_EXCHANGE:
		status := NewKrakenScraper(pairs, tradesChannel, failoverChannel, wg)
		if status == "closed" {
			return
		}
	case KUCOIN_EXCHANGE:
		status := NewKuCoinScraper(pairs, tradesChannel, failoverChannel, wg)
		if status == "closed" {
			return
		}

	case UNISWAPV2_EXCHANGE:
		NewUniswapV2Scraper(pools, tradesChannel, wg)
	case Simulation:
		NewSimulationScraper(pools, tradesChannel, wg)

	}
}

func watchdog(
	lastTradeTime time.Time,
	watchdogDelay int,
	watchdogTicker *time.Ticker,
	run *bool,
) {

	for range watchdogTicker.C {
		log.Info("lastTradeTime: ", lastTradeTime)
		log.Info("timeNow: ", time.Now())
		duration := time.Since(lastTradeTime)
		if duration > time.Duration(binanceWatchdogDelay)*time.Second {
			log.Error("failover")
			*run = false
			break
		}
	}

}
