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

		watchdogDelay, err := strconv.Atoi(utils.Getenv("BINANCE_WATCHDOG_DELAY", "300"))
		if err != nil {
			log.Error("parse BINANCE_WATCHDOG_DELAY: ", err)
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
						log.Error("Binance - Close(): ", err)
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

		watchdogDelay, err := strconv.Atoi(utils.Getenv("COINBASE_WATCHDOG_DELAY", "300"))
		if err != nil {
			log.Error("parse COINBASE_WATCHDOG_DELAY: ", err)
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
						log.Error("CoinBase - Close(): ", err)
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

		watchdogDelay, err := strconv.Atoi(utils.Getenv("CRYPTODOTCOM_WATCHDOG_DELAY", "300"))
		if err != nil {
			log.Error("parse CRYPTODOTCOM_WATCHDOG_DELAY: ", err)
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
						log.Error("Crypto.com - Close(): ", err)
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

		watchdogDelay, err := strconv.Atoi(utils.Getenv("GATEIO_WATCHDOG_DELAY", "300"))
		if err != nil {
			log.Error("parse GATEIO_WATCHDOG_DELAY: ", err)
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
						log.Error("GateIO - Close(): ", err)
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

		watchdogDelay, err := strconv.Atoi(utils.Getenv("KRAKEN_WATCHDOG_DELAY", "300"))
		if err != nil {
			log.Error("parse KRAKEN_WATCHDOG_DELAY: ", err)
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
						log.Error("Kraken - Close(): ", err)
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

		watchdogDelay, err := strconv.Atoi(utils.Getenv("KUCOIN_WATCHDOG_DELAY", "300"))
		if err != nil {
			log.Error("parse KUCOIN_WATCHDOG_DELAY: ", err)
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
						log.Error("KuCoin - Close(): ", err)
					}
					log.Warnf("Close KuCoin scraper as duration since last trade is %v.", duration)
					failoverChannel <- KUCOIN_EXCHANGE
					return
				}
			}
		}

	case UNISWAPV2_EXCHANGE:
		NewUniswapV2Scraper(pools, tradesChannel, wg)
	case Simulation:
		NewSimulationScraper(pools, tradesChannel, wg)

	}
}

// If @handleErrorReadJSON returns true, the calling function should return. Otherwise continue.
func handleErrorReadJSON(err error, errCount *int, maxErrCount int, restartWaitTime int) bool {
	log.Errorf("%s - ReadMessage: %v", COINBASE_EXCHANGE, err)
	*errCount++

	if strings.Contains(err.Error(), "closed network connection") {
		return true
	}

	if *errCount > maxErrCount {
		log.Warnf("too many errors. wait for %v seconds and restart scraper.", restartWaitTime)
		time.Sleep(time.Duration(restartWaitTime) * time.Second)
		return true
	}

	return false
}
