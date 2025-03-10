package scrapers

import (
	"context"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
)

// Collector starts scrapers for all exchanges given by @exchangePairs.
func Collector(
	exchangePairs []models.ExchangePair,
	pools []models.Pool,
	tradesblockChannel chan map[string]models.TradesBlock,
	triggerChannel chan time.Time,
	failoverChannel chan string,
	wg *sync.WaitGroup,
) {

	// exchangepairMap maps a centralized exchange onto the given pairs.
	exchangepairMap := models.MakeExchangepairMap(exchangePairs)
	log.Debugf("Collector - exchangepairMap: %v.", exchangepairMap)
	// poolMap maps a decentralized exchange onto the given pools.
	poolMap := models.MakePoolMap(pools)
	log.Debugf("Collector - poolMap: %v.", poolMap)

	// Start all needed scrapers.
	// @tradesChannelIn collects trades from the started scrapers.
	tradesChannelIn := make(chan models.Trade)
	for exchange := range exchangepairMap {
		wg.Add(1)
		go RunScraper(context.Background(), exchange, exchangepairMap[exchange], []models.Pool{}, tradesChannelIn, failoverChannel, wg)
	}
	for exchange := range poolMap {
		wg.Add(1)
		go RunScraper(context.Background(), exchange, []models.ExchangePair{}, poolMap[exchange], tradesChannelIn, failoverChannel, wg)
	}

	// tradesblockMap maps an exchangpair identifier onto a TradesBlock.
	// This also means that each value in the map consists of trades of only one exchangepair.
	// We call these blocks "atomic" tradesblocks.
	// TO DO: Make a dedicated type for atomic tradesblocks?
	tradesblockMap := make(map[string]models.TradesBlock)

	go func() {
		for {
			select {
			case trade := <-tradesChannelIn:

				// Determine exchangepair and the corresponding identifier in order to assign the tradesBlockMap.
				exchangepair := models.Pair{QuoteToken: trade.QuoteToken, BaseToken: trade.BaseToken}
				exchangepairIdentifier := exchangepair.ExchangePairIdentifier(trade.Exchange.Name)

				if _, ok := tradesblockMap[exchangepairIdentifier]; !ok {
					tradesblockMap[exchangepairIdentifier] = models.TradesBlock{
						Trades: []models.Trade{trade},
						Pair:   exchangepair,
					}
				} else {
					tradesblock := tradesblockMap[exchangepairIdentifier]
					tradesblock.Trades = append(tradesblock.Trades, trade)
					tradesblockMap[exchangepairIdentifier] = tradesblock
				}

			case timestamp := <-triggerChannel:

				log.Debugf("Collector - triggered at %v.", timestamp)
				for id := range tradesblockMap {
					tb := tradesblockMap[id]
					tb.Atomic = true
					tb.EndTime = timestamp
					tradesblockMap[id] = tb
				}

				tradesblockChannel <- tradesblockMap
				log.Infof("Collector - number of tradesblocks: %v.", len(tradesblockMap))

				// Make a new tradesblockMap for the next trigger period.
				tradesblockMap = make(map[string]models.TradesBlock)

			case exchange := <-failoverChannel:
				log.Debugf("Collector - Restart scraper for %s.", exchange)
				wg.Add(1)
				go RunScraper(context.Background(), exchange, exchangepairMap[exchange], []models.Pool{}, tradesChannelIn, failoverChannel, wg)
			}
		}
	}()

	defer wg.Wait()
}
