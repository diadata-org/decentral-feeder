package scrapers

import (
	"context"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
)

// watchdog checks for liveliness of a pair subscription.
// More precisely, if there is no trades for a period longer than @watchdogDelayMap[pair.ForeignName],
// the @runChannel receives the corresponding pair. The calling function can decide what to do, for
// instance resubscribe to the pair.
func watchdog(
	ctx context.Context,
	pair models.ExchangePair,
	ticker *time.Ticker,
	lastTradeTimeMap map[string]time.Time,
	watchdogDelay int64,
	subscribeChannel chan models.ExchangePair,
	lock *sync.RWMutex,
) {
	log.Infof("%s - start watching %s.", pair.Exchange, pair.ForeignName)
	for {
		select {
		case <-ticker.C:
			log.Debugf("%s - check liveliness of %s.", pair.Exchange, pair.ForeignName)

			// Make read lock for lastTradeTimeMap.
			lock.RLock()
			duration := time.Since(lastTradeTimeMap[pair.ForeignName])
			log.Debugf("%s - duration for %s: %v. Threshold: %v.", pair.Exchange, pair.ForeignName, duration, watchdogDelay)
			lock.Unlock()
			if duration > time.Duration(watchdogDelay)*time.Second {
				log.Errorf("%s - watchdogTicker failover for %s.", pair.Exchange, pair.ForeignName)
				subscribeChannel <- pair
			}
		case <-ctx.Done():
			log.Debugf("%s - close watchdog for pair %s.", pair.Exchange, pair.ForeignName)
			return
		}
	}
}

// TO DO: rewrite scrapers such that they return a static object.
// It can be run using a Run() method that should be added to the Scraper interface.
// This way, we can instantiate a scraper by name below. Then we can call the resubscribe method which should also
// be added to the Scraper interface.
// func watchPair(
// 	ctx context.Context,
// 	lastTradeTimeMap map[string]time.Time,
// 	subscribeChannel chan models.ExchangePair,
// 	pair models.ExchangePair,
// 	exchange string,
// 	lock *sync.RWMutex,
// ) {
// 	envVar := strings.ToUpper(exchange) + "_WATCHDOG_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[0] + "_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[1]
// 	watchdogDelay, err := strconv.ParseInt(utils.Getenv(envVar, "60"), 10, 64)
// 	if err != nil {
// 		log.Error("Parse cryptodotcomWatchdogDelay: ", err)
// 	}
// 	watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
// 	go watchdog(ctx, pair, watchdogTicker, lastTradeTimeMap, watchdogDelay, subscribeChannel, lock)
// 	go scraper.resubscribe(ctx, lock)
// }
