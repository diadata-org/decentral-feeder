package scrapers

import (
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
)

// Collector starts scrapers for all exchanges given by @exchangePairs.
// Outlook: Collector starts a dedicated pod for each scraper.
func Collector(
	exchangePairs []models.ExchangePair,
	pools []models.Pool,
	tradesblockChannel chan map[string]models.TradesBlock,
	triggerChannel chan time.Time,
	wg *sync.WaitGroup,
) {

	// exchangepairMap maps a centralized exchange onto the given pairs.
	exchangepairMap := models.MakeExchangepairMap(exchangePairs)
	log.Info("exchangepairMap: ", exchangepairMap)
	// poolMap maps a decentralized exchange onto the given pools.
	poolMap := models.MakePoolMap(pools)
	log.Info("poolMap: ", poolMap)

	// Start all needed scrapers.
	// @tradesChannelIn collects trades from the started scrapers.
	tradesChannelIn := make(chan models.Trade)
	for exchange := range exchangepairMap {
		wg.Add(1)
		go RunScraper(exchange, exchangepairMap[exchange], []models.Pool{}, tradesChannelIn, wg)
	}
	for exchange := range poolMap {
		wg.Add(1)
		go RunScraper(exchange, []models.ExchangePair{}, poolMap[exchange], tradesChannelIn, wg)
	}

	// tradesblockMap maps an exchangpair identifier onto a TradesBlock.
	// This also means that each value consists of trades of only one exchangepair.
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

				log.Info("triggered at : ", timestamp)
				for id := range tradesblockMap {
					tb := tradesblockMap[id]
					tb.EndTime = timestamp
					tradesblockMap[id] = tb
				}

				tradesblockChannel <- tradesblockMap
				log.Info("number of tradesblocks: ", len(tradesblockMap))

				// Make a new tradesblockMap for the next trigger period.
				tradesblockMap = make(map[string]models.TradesBlock)

			}
		}
	}()

	defer wg.Wait()
}
