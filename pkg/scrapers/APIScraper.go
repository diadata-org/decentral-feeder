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

	}
}

// globalWatchdog checks for liveliness of a scraper.
// More precisely, if there is no trades for a period longer than @watchdogDelay the scraper is stopped
// by setting run=false.
func globalWatchdog(ticker *time.Ticker, lastTradeTime *time.Time, watchdogDelay int64, run *bool) {
	for range ticker.C {
		duration := time.Since(*lastTradeTime)
		if duration > time.Duration(watchdogDelay)*time.Second {
			log.Error("CoinBase - watchdogTicker failover")
			*run = false
			break
		}
	}
}

func watchdog(pair models.ExchangePair, ticker *time.Ticker, lastTradeTime *time.Time, watchdogDelay int64, run *bool) {
	// TO DO: watchdog per pair.
}

func readJSONError(exchange string, err error, errCount *int, run *bool, restartWaitTime int, maxErrCount int) {
	log.Errorf("%s - ReadMessage: %v", exchange, err)
	*errCount++
	if *errCount > maxErrCount {
		log.Warnf("too many errors. wait for %v seconds and restart scraper.", restartWaitTime)
		time.Sleep(time.Duration(restartWaitTime) * time.Second)
		*run = false
	}
	return
}
