package simulationprocessor

import (
	"sync"
	"time"

	"github.com/diadata-org/decentral-feeder/pkg/filters"
	"github.com/diadata-org/decentral-feeder/pkg/metafilters"
	models "github.com/diadata-org/decentral-feeder/pkg/models"
	simulationfilters "github.com/diadata-org/decentral-feeder/pkg/simulations/simulationFilters"
	"github.com/diadata-org/decentral-feeder/pkg/simulations/simulators"
)

// Processor handles blocks from @tradesblockChannel.
// More precisley, it does so in a 2 step procedure:
// 1. Aggregate trades for each (atomic) block.
// 2. Aggregate filter values obtained in step 1.
func Processor(
	exchangePairs []models.ExchangePair,
	tradesblockChannel chan map[string]models.SimulatedTradesBlock,
	filtersChannel chan []models.FilterPointPair,
	triggerChannel chan time.Time,
	wg *sync.WaitGroup,
) {

	log.Info("Processor - Start......")
	// Collector starts collecting trades in the background and sends atomic tradesblocks to @tradesblockChannel.
	go simulators.Collector(exchangePairs, tradesblockChannel, triggerChannel, wg)

	// As soon as the trigger channel receives input a processing step is initiated.
	for tradesblocks := range tradesblockChannel {

		var filterPoints []models.FilterPointPair
		// Renew the price cache in each iteration. Could be refined by adjusting to the frequency of the source.
		priceCacheMap := make(map[string]float64)

		// --------------------------------------------------------------------------------------------
		// 1. Compute an aggregated value for each pair using all collected trades.
		// --------------------------------------------------------------------------------------------
		for _, tb := range tradesblocks {

			var atomicFilterValue float64
			reducedTradesBlock := models.SimulatedTradesBlockToTradesBlock(tb)

			basePrice, err := models.GetPriceBaseAsset(reducedTradesBlock, priceCacheMap)
			if err != nil {
				log.Errorf("Processor - GetPriceBaseAsset: %v", err)
				continue
			}

			switch filterType {
			case "LastPrice":
				atomicFilterValue, _ = filters.LastPrice(reducedTradesBlock.Trades, basePrice)

			case "Average":
				atomicFilterValue, _, err = simulationfilters.Average(tb.Trades, true)
				if err != nil {
					log.Errorf("Processor - Average: %v.", err)
					continue
				}

			}
			log.Infof(
				"Processor - Atomic filter value for market %s with %v trades: %v.",
				tb.Trades[0].Exchange.Name+":"+tb.Trades[0].QuoteToken.Symbol+"-"+tb.Trades[0].BaseToken.Symbol,
				len(tb.Trades),
				atomicFilterValue,
			)

			// Identify @Pair and @SourceType from atomic tradesblock.
			filterPoint := models.FilterPointPair{
				Pool:  tb.Pool,
				Pair:  tb.Pair,
				Value: atomicFilterValue,
				Time:  tb.EndTime,
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

		switch metaFilterType {
		// TO DO: Add methodology for metafilters of simulated data.
		case "Median":
			filterPointsMedianized := metafilters.Median(filterPoints)
			filtersChannel <- filterPointsMedianized
			for _, fpm := range filterPointsMedianized {
				log.Infof("Processor - filter %s for %s: %v.", fpm.Name, fpm.Pair.QuoteToken.Symbol, fpm.Value)
			}
		}

	}

}
