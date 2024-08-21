package scrapers

import (
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
)

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

// watchdog checks for liveliness of a pair subscription.
// More precisely, if there is no trades for a period longer than @watchdogDelayMap[pair.ForeignName],
// the @runChannel receives the corresponding pair. The calling function can decide what to do, for
// instance resubscribe to the pair.
func watchdog(
	pair models.ExchangePair,
	ticker *time.Ticker,
	lastTradeTimeMap map[string]time.Time,
	watchdogDelayMap map[string]int64,
	runChannel chan models.ExchangePair,
	lock *sync.RWMutex,
) {
	for range ticker.C {
		lock.RLock()
		duration := time.Since(lastTradeTimeMap[pair.ForeignName])
		if duration > time.Duration(watchdogDelayMap[pair.ForeignName])*time.Second {
			log.Errorf("CoinBase - watchdogTicker failover for %s", pair.ForeignName)
			runChannel <- pair
		}
		lock.RUnlock()
	}
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
