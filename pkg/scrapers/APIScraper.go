package scrapers

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
)

// TO DO: We can think about making a scraper interface with tradesChannel, failoverChannel and normalizePairTicker methods.
type Scraper interface {
	TradesChannel() chan models.Trade
	Close(cancel context.CancelFunc) error
	// Subscribe(pair models.ExchangePair, subscribe bool, lock *sync.RWMutex) error
}

// RunScraper starts a scraper for @exchange.
func RunScraper(
	ctx context.Context,
	exchange string,
	pairs []models.ExchangePair,
	pools []models.Pool,
	tradesChannel chan models.Trade,
	failoverChannel chan string,
	wg *sync.WaitGroup,
) {
	switch exchange {
	case BINANCE_EXCHANGE:

		ctx, cancel := context.WithCancel(context.Background())
		scraper := NewBinanceScraper(ctx, pairs, failoverChannel, wg)

		watchdogDelay, err := strconv.Atoi(utils.Getenv("BINANCE_WATCHDOG", "300"))
		if err != nil {
			log.Errorf("parse BINANCE_WATCHDOG: %v.", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
		lastTradeTime := time.Now()

		for {
			select {
			case trade := <-scraper.TradesChannel():
				lastTradeTime = time.Now()
				tradesChannel <- trade

			case <-watchdogTicker.C:
				duration := time.Since(lastTradeTime)
				if duration > time.Duration(watchdogDelay)*time.Second {
					err := scraper.Close(cancel)
					if err != nil {
						log.Errorf("Binance - Close(): %v.", err)
					}
					log.Warnf("Closed Binance scraper as duration since last trade is %v.", duration)
					failoverChannel <- BINANCE_EXCHANGE
					return
				}
			}
		}

	case COINBASE_EXCHANGE:
		ctx, cancel := context.WithCancel(context.Background())
		scraper := NewCoinBaseScraper(ctx, pairs, failoverChannel, wg)

		watchdogDelay, err := strconv.Atoi(utils.Getenv("COINBASE_WATCHDOG", "300"))
		if err != nil {
			log.Errorf("parse COINBASE_WATCHDOG: %v.", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
		lastTradeTime := time.Now()

		for {
			select {
			case trade := <-scraper.TradesChannel():
				lastTradeTime = time.Now()
				tradesChannel <- trade

			case <-watchdogTicker.C:
				duration := time.Since(lastTradeTime)
				if duration > time.Duration(watchdogDelay)*time.Second {
					err := scraper.Close(cancel)
					if err != nil {
						log.Errorf("CoinBase - Close(): %v.", err)
					}
					log.Warnf("Closed CoinBase scraper as duration since last trade is %v.", duration)
					failoverChannel <- COINBASE_EXCHANGE
					return
				}
			}
		}
	case CRYPTODOTCOM_EXCHANGE:
		ctx, cancel := context.WithCancel(context.Background())
		scraper := NewCryptodotcomScraper(ctx, pairs, failoverChannel, wg)

		watchdogDelay, err := strconv.Atoi(utils.Getenv("CRYPTODOTCOM_WATCHDOG", "300"))
		if err != nil {
			log.Errorf("parse CRYPTODOTCOM_WATCHDOG: %v.", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
		lastTradeTime := time.Now()

		for {
			select {
			case trade := <-scraper.TradesChannel():
				lastTradeTime = time.Now()
				tradesChannel <- trade

			case <-watchdogTicker.C:
				duration := time.Since(lastTradeTime)
				if duration > time.Duration(watchdogDelay)*time.Second {
					err := scraper.Close(cancel)
					if err != nil {
						log.Errorf("Crypto.com - Close(): %v.", err)
					}
					log.Warnf("Closed Crypto.com scraper as duration since last trade is %v.", duration)
					failoverChannel <- CRYPTODOTCOM_EXCHANGE
					return
				}
			}
		}
	case GATEIO_EXCHANGE:
		ctx, cancel := context.WithCancel(context.Background())
		scraper := NewGateIOScraper(ctx, pairs, failoverChannel, wg)

		watchdogDelay, err := strconv.Atoi(utils.Getenv("GATEIO_WATCHDOG", "300"))
		if err != nil {
			log.Errorf("parse GATEIO_WATCHDOG: %v.", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
		lastTradeTime := time.Now()

		for {
			select {
			case trade := <-scraper.TradesChannel():
				lastTradeTime = time.Now()
				tradesChannel <- trade

			case <-watchdogTicker.C:
				duration := time.Since(lastTradeTime)
				if duration > time.Duration(watchdogDelay)*time.Second {
					err := scraper.Close(cancel)
					if err != nil {
						log.Errorf("GateIO - Close(): %v.", err)
					}
					log.Warnf("Closed GateIO scraper as duration since last trade is %v.", duration)
					failoverChannel <- GATEIO_EXCHANGE
					return
				}
			}
		}
	case KRAKEN_EXCHANGE:
		ctx, cancel := context.WithCancel(context.Background())
		scraper := NewKrakenScraper(ctx, pairs, failoverChannel, wg)

		watchdogDelay, err := strconv.Atoi(utils.Getenv("KRAKEN_WATCHDOG", "300"))
		if err != nil {
			log.Errorf("parse KRAKEN_WATCHDOG: %v.", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
		lastTradeTime := time.Now()

		for {
			select {
			case trade := <-scraper.TradesChannel():
				lastTradeTime = time.Now()
				tradesChannel <- trade

			case <-watchdogTicker.C:
				duration := time.Since(lastTradeTime)
				if duration > time.Duration(watchdogDelay)*time.Second {
					err := scraper.Close(cancel)
					if err != nil {
						log.Errorf("Kraken - Close(): %v.", err)
					}
					log.Warnf("Close Kraken scraper as duration since last trade is %v.", duration)
					failoverChannel <- KRAKEN_EXCHANGE
					return
				}
			}
		}
	case KUCOIN_EXCHANGE:
		ctx, cancel := context.WithCancel(context.Background())
		scraper := NewKuCoinScraper(ctx, pairs, failoverChannel, wg)

		watchdogDelay, err := strconv.Atoi(utils.Getenv("KUCOIN_WATCHDOG", "300"))
		if err != nil {
			log.Errorf("parse KUCOIN_WATCHDOG: %v.", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
		lastTradeTime := time.Now()

		for {
			select {
			case trade := <-scraper.TradesChannel():
				lastTradeTime = time.Now()
				tradesChannel <- trade

			case <-watchdogTicker.C:
				duration := time.Since(lastTradeTime)
				if duration > time.Duration(watchdogDelay)*time.Second {
					err := scraper.Close(cancel)
					if err != nil {
						log.Errorf("KuCoin - Close(): %v.", err)
					}
					log.Warnf("Close KuCoin scraper as duration since last trade is %v.", duration)
					failoverChannel <- KUCOIN_EXCHANGE
					return
				}
			}
		}

	case UNISWAPV2_EXCHANGE:
		NewUniswapV2Scraper(pools, tradesChannel, wg)

	}
}

// If @handleErrorReadJSON returns true, the calling function should return. Otherwise continue.
func handleErrorReadJSON(err error, errCount *int, maxErrCount int, exchange string, restartWaitTime int) bool {
	log.Errorf("%s - ReadMessage: %v", exchange, err)
	*errCount++

	if strings.Contains(err.Error(), "closed network connection") {
		return true
	}

	if *errCount > maxErrCount {
		log.Warnf("%s - too many errors. wait for %v seconds and restart scraper.", exchange, restartWaitTime)
		time.Sleep(time.Duration(restartWaitTime) * time.Second)
		return true
	}

	return false
}
