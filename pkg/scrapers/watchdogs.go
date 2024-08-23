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
	watchdogDelay int64,
	subscribeChannel chan models.ExchangePair,
	lock *sync.RWMutex,
) {
	log.Infof("start watching %s.", pair.ForeignName)
	for range ticker.C {
		log.Infof("%s - check liveliness of %s.", pair.Exchange, pair.ForeignName)

		// Make read lock for lastTradeTimeMap.
		lock.RLock()
		duration := time.Since(lastTradeTimeMap[pair.ForeignName])
		log.Infof("%s - duration for %s: %v. Threshold: %v", pair.Exchange, pair.ForeignName, duration, watchdogDelay)
		lock.RUnlock()
		if duration > time.Duration(watchdogDelay)*time.Second {
			log.Errorf("CoinBase - watchdogTicker failover for %s", pair.ForeignName)
			subscribeChannel <- pair
		}
	}
}

// // TO DO: Add watchdog per pair that includes watchdog per pair and a subsquent resubscribe.
// func watchdogPair(
// 	pair models.ExchangePair,
// 	lastTradeTimeMap map[string]time.Time,
// 	watchdogDelay int64,
// 	subscribeChannel chan models.ExchangePair,
// 	lock *sync.RWMutex,
// 	wsClient *ws.Conn,
// ) {

// 	envVar := strings.ToUpper(pair.Exchange) + "_WATCHDOG_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[0] + "_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[1]
// 	watchdogDelay, err := strconv.ParseInt(utils.Getenv(envVar, "60"), 10, 64)
// 	if err != nil {
// 		log.Error("Parse coinbaseWatchdogDelayMap: ", err)
// 	}
// 	watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
// 	go watchdog(pair, watchdogTicker, coinbaseLastTradeTimeMap, coinbaseWatchdogDelay, coinbaseSubscribeChannel, lock)
// 	go coinbaseResubscribe(coinbaseSubscribeChannel, lock, wsClient)
// }

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
