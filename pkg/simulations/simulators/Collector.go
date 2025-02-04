package simulators

import (
	"context"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
)

// Collector starts scrapers for all exchanges given by @exchangePairs.
func Collector(
	exchangePairs []models.ExchangePair,
	tradesblockChannel chan map[string]models.SimulatedTradesBlock,
	triggerChannel chan time.Time,
	wg *sync.WaitGroup,
) {

	exchangepairMap := models.MakeExchangepairMap(exchangePairs)
	// Start all needed scrapers.
	// @tradesChannelIn collects trades from the started scrapers.
	tradesChannelIn := make(chan models.SimulatedTrade)

	for exchange := range exchangepairMap {
		wg.Add(1)
		go RunSimulator(context.Background(), exchange, exchangepairMap[exchange], tradesChannelIn, wg)
	}

	// tradesblockMap maps a pool identifier onto a SimulatedTradesBlock.
	// This also means that each value in the map consists of trades of only one pool.
	// We call these blocks "atomic" tradesblocks.
	tradesblockMap := make(map[string]models.SimulatedTradesBlock)

	go func() {
		for {
			select {
			case trade := <-tradesChannelIn:

				// Determine pool and the corresponding identifier in order to assign the tradesBlockMap.
				poolIdentifier := trade.Exchange.Blockchain + trade.PoolAddress

				if _, ok := tradesblockMap[poolIdentifier]; !ok {
					tradesblockMap[poolIdentifier] = models.SimulatedTradesBlock{
						Trades: []models.SimulatedTrade{trade},
						Pool:   models.Pool{Blockchain: models.Blockchain{Name: trade.Exchange.Blockchain}, Address: trade.PoolAddress},
					}
				} else {
					tradesblock := tradesblockMap[poolIdentifier]
					tradesblock.Trades = append(tradesblock.Trades, trade)
					tradesblockMap[poolIdentifier] = tradesblock
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
				tradesblockMap = make(map[string]models.SimulatedTradesBlock)

			}
		}
	}()

	defer wg.Wait()
}
