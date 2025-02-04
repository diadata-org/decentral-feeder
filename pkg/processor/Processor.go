package processor

import (
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
	filtersChannel chan []models.FilterPointPair,
	triggerChannel chan time.Time,
	failoverChannel chan string,
	wg *sync.WaitGroup,
) {

	log.Info("Processor - Start......")
	// Collector starts collecting trades in the background and sends atomic tradesblocks to @tradesblockChannel.
	go scrapers.Collector(exchangePairs, pools, tradesblockChannel, triggerChannel, failoverChannel, wg)

	// As soon as the trigger channel receives input a processing step is initiated.
	for tradesblocks := range tradesblockChannel {

		var filterPoints []models.FilterPointPair

		// --------------------------------------------------------------------------------------------
		// 1. Compute an aggregated value for each pair on a given exchange using all collected trades.
		// --------------------------------------------------------------------------------------------
		for _, tb := range tradesblocks {

			// filter switch, for instance LastPrice, Median, Average, Minimum, etc.
			sourceType, err := tb.GetSourceType()
			if err != nil {
				log.Warn(err)
			}

			var atomicFilterValue float64
			switch sourceType {

			case models.CEX_SOURCE:

				switch filterTypeCEX {
				case string(FILTER_LAST_PRICE):
					atomicFilterValue, _, err = filters.LastPrice(tb.Trades, true)
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
				}

			case models.SIMULATION_SOURCE:

				switch filterTypeSimulation {
				// TO DO: Write filter for simulation.
				case string(FILTER_LAST_PRICE):
					atomicFilterValue, _, err = filters.LastPrice(tb.Trades, true)
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
				}

			}

			// Identify @Pair and @SourceType from atomic tradesblock.
			filterPoint := models.FilterPointPair{
				Pair:       tb.Pair,
				Value:      atomicFilterValue,
				Time:       tb.EndTime,
				SourceType: sourceType,
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

		// Group filter points by their @SourceType.
		filterMap := make(map[models.SourceType][]models.FilterPointPair)
		for _, fp := range filterPoints {
			switch fp.SourceType {
			case models.CEX_SOURCE:
				filterMap[models.CEX_SOURCE] = append(filterMap[models.CEX_SOURCE], fp)
			case models.SIMULATION_SOURCE:
				filterMap[models.SIMULATION_SOURCE] = append(filterMap[models.SIMULATION_SOURCE], fp)
			}
		}

		// TO DO: Range over source type and make switch for filter Type.
		for sourceType, filterPoints := range filterMap {

			switch sourceType {

			case models.CEX_SOURCE:

				switch metaFilterTypeCEX {
				case string(METAFILTER_MEDIAN):
					filterPointsMedianized := metafilters.Median(filterPoints)
					for _, fpm := range filterPointsMedianized {
						log.Infof("Processor - filter %s for %s: %v.", fpm.Name, fpm.Pair.QuoteToken.Symbol, fpm.Value)
					}
				}

			case models.SIMULATION_SOURCE:

				switch metaFilterTypeSimulation {
				// TO DO: Add methodology for metafilters of simulated data.
				case string(METAFILTER_MEDIAN):
					filterPointsMedianized := metafilters.Median(filterPoints)
					for _, fpm := range filterPointsMedianized {
						log.Infof("Processor - filter %s for %s: %v.", fpm.Name, fpm.Pair.QuoteToken.Symbol, fpm.Value)
					}
				}

			}

		}

		filterPointsMedianized := metafilters.Median(filterPoints)
		for _, fpm := range filterPointsMedianized {
			log.Infof("Processor - filter %s for %s: %v.", fpm.Name, fpm.Pair.QuoteToken.Symbol, fpm.Value)
		}

		filtersChannel <- filterPointsMedianized
	}

}
