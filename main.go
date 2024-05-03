package main

import (
	"sync"
	"time"

	processing "github.com/diadata-org/decentral-feeder/pkg/processing"
	scrapers "github.com/diadata-org/decentral-feeder/pkg/scraper"
	models "github.com/diadata-org/diaprotocol/pkg/models"
	log "github.com/sirupsen/logrus"
)

func main() {
	exchanges := []models.Exchange{{Name: scrapers.BINANCE_EXCHANGE}}
	pairs := []string{"BTC-USDT"}

	wg := sync.WaitGroup{}
	tradesblockChannel := make(chan []models.Trade)
	filtersChannel := make(chan float64)
	triggerChannel := make(chan time.Time)

	// Use a ticker for triggering the processing.
	// This is for testing purposes for now.
	triggerTick := time.NewTicker(time.Duration(5 * time.Second))
	go func() {
		for tick := range triggerTick.C {
			log.Warn("tick at: ", tick)
			triggerChannel <- tick
		}
	}()

	// Run Processor and subsequent routines.
	Processor(exchanges, pairs, tradesblockChannel, filtersChannel, triggerChannel, &wg)

}

func Processor(exchanges []models.Exchange,
	pairs []string,
	tradesblockChannel chan []models.Trade,
	filterChannel chan float64,
	triggerChannel chan time.Time,
	wg *sync.WaitGroup,
) {

	// Collector starts collecting trades in the background.
	go Collector(exchanges, pairs, tradesblockChannel, triggerChannel, wg)

	// As soon as the trigger channel receives input a processing step is initiated.
	for range triggerChannel {
		trades := <-tradesblockChannel
		log.Infof("received %v trades for further processing.", len(trades))
		asset := models.Asset{Blockchain: "Bitcoin", Address: "0x0000000000000000000000000000000000000000"}
		latestPrice, timestamp, err := processing.LastPrice(trades, asset, true)
		if err != nil {
			log.Error("GetLastPrice: ", err)
		}
		log.Infof("LatestPrice -- timestamp: %v -- %v", latestPrice, timestamp)
	}

}

// Collector starts a scraper for given @exchanges
func Collector(
	exchanges []models.Exchange,
	pairs []string,
	tradesblockChannel chan []models.Trade,
	triggerChannel chan time.Time,
	wg *sync.WaitGroup,
) {

	tradesChannelIn := make(chan models.Trade)
	for _, exchange := range exchanges {
		wg.Add(1)
		go scrapers.RunScraper(exchange.Name, pairs, tradesChannelIn, wg)
	}

	var collectedTrades []models.Trade

	go func() {
		for {
			select {
			case trade := <-tradesChannelIn:
				collectedTrades = append(collectedTrades, trade)

			case timestamp := <-triggerChannel:
				tradesblockChannel <- collectedTrades
				log.Info("triggered at : ", timestamp)
				collectedTrades = []models.Trade{}

			}
		}
	}()

	defer wg.Wait()
}
