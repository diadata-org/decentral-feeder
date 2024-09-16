package processor

import (
	"strings"
	"sync"
	"time"

	"github.com/diadata-org/decentral-feeder/pkg/filters"
	"github.com/diadata-org/decentral-feeder/pkg/metafilters"
	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/scrapers"
)

// Processor handles blocks from @tradesblockChannel.
// More precisley, it does so in a 2 step procedure:
// 1. Aggregate trades for each (atomic) block.
// 2. Aggregate filter values obtained in step 1.
func Processor(
	exchangePairs []models.ExchangePair,
	pools []models.Pool,
	tradesblockChannel chan map[string]models.TradesBlock,
	filtersChannel chan []models.FilterPointExtended,
	triggerChannel chan time.Time,
	failoverChannel chan string,
	wg *sync.WaitGroup,
) {

	log.Info("Processor - Start......")
	// Collector starts collecting trades in the background and sends atomic tradesblocks to @tradesblockChannel.
	go scrapers.Collector(exchangePairs, pools, tradesblockChannel, triggerChannel, failoverChannel, wg)

	// As soon as the trigger channel receives input a processing step is initiated.
	for tradesblocks := range tradesblockChannel {

		var filterPoints []models.FilterPointExtended

		// --------------------------------------------------------------------------------------------
		// 1. Compute an aggregated value for each pair on a given exchange using all collected trades.
		// --------------------------------------------------------------------------------------------
		for exchangepairIdentifier, tb := range tradesblocks {

			// TO DO: Set flag for trades' filter switch. For instance Median, Average, Minimum, etc.
			atomicFilterValue, _, err := filters.LastPrice(tb.Trades, true)
			if err != nil {
				log.Errorf("Processor - GetLastPrice: %v.", err)
				continue
			}
			log.Infof(
				"Processor - Atomic filter value for market %s with %v trades: %v.",
				tb.Trades[0].Exchange.Name+":"+tb.Trades[0].QuoteToken.Symbol+"-"+tb.Trades[0].BaseToken.Symbol,
				len(tb.Trades),
				atomicFilterValue,
			)

			// Identify Pair from tradesblock
			filterPoint := models.FilterPointExtended{
				Pair:   tb.Pair,
				Value:  atomicFilterValue,
				Time:   tb.EndTime,
				Source: strings.Split(exchangepairIdentifier, "-")[0],
			}
			filterPoints = append(filterPoints, filterPoint)

		}

		var removedFilterPoints int
		filterPoints, removedFilterPoints = models.RemoveOldFilters(filterPoints, toleranceSeconds, time.Now())
		if removedFilterPoints > 0 {
			log.Warnf("Processor - Removed %v old filter points.", removedFilterPoints)
		}

		// --------------------------------------------------------------------------------------------
		// 2. Compute an aggregated value across exchanges for each asset obtained from the aggregated
		// filter values in Step 1.
		// --------------------------------------------------------------------------------------------
		// TO DO: Set flag for metafilter switch. For instance Median, Average, Minimum, etc.
		filterPointsMedianized := metafilters.Median(filterPoints)
		for _, fpm := range filterPointsMedianized {
			log.Infof("Processor - filter %s for %s: %v.", fpm.Name, fpm.Pair.QuoteToken.Symbol, fpm.Value)
		}

		filtersChannel <- filterPointsMedianized
	}

}
