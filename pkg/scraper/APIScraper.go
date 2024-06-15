package scrapers

import (
	"sync"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
)

// RunScraper starts a scraper for @exchange.
func RunScraper(exchange string, pairs []models.ExchangePair, pools []models.Pool, tradesChannel chan models.Trade, wg *sync.WaitGroup) {
	switch exchange {
	case BINANCE_EXCHANGE:
		NewBinanceScraper(pairs, tradesChannel, wg)
	case CRYPTODOTCOM_EXCHANGE:
		NewCryptoDotComScraper(pairs, tradesChannel, wg)
	case GATEIO_EXCHANGE:
		NewGateIOScraper(pairs, tradesChannel, wg)
	case KUCOIN_EXCHANGE:
		NewKuCoinScraper(pairs, tradesChannel, wg)

	case UNISWAPV2_EXCHANGE:
		NewUniswapV2Scraper(pools, tradesChannel, wg)

	}
}
