package scrapers

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
	ws "github.com/gorilla/websocket"
)

// A coinBaseWSSubscribeMessage represents a message to subscribe the public/private channel.
type coinBaseWSSubscribeMessage struct {
	Type     string            `json:"type"`
	Channels []coinBaseChannel `json:"channels"`
}

type coinBaseChannel struct {
	Name       string   `json:"name"`
	ProductIDs []string `json:"product_ids"`
}

type coinBaseWSResponse struct {
	Type         string `json:"type"`
	TradeID      int64  `json:"trade_id"`
	Sequence     int64  `json:"sequence"`
	MakerOrderID string `json:"maker_order_id"`
	TakerOrderID string `json:"taker_order_id"`
	Time         string `json:"time"`
	ProductID    string `json:"product_id"`
	Size         string `json:"size"`
	Price        string `json:"price"`
	Side         string `json:"side"`
}

type coinbaseScraper struct {
	wsClient         *ws.Conn
	tradesChannel    chan models.Trade
	subscribeChannel chan models.ExchangePair
	tickerPairMap    map[string]models.Pair
	lastTradeTimeMap map[string]time.Time
	maxErrCount      int
	restartWaitTime  int
	genesis          time.Time
}

var (
	coinbaseWSBaseString = "wss://ws-feed.exchange.coinbase.com"
)

func NewCoinBaseScraper(ctx context.Context, pairs []models.ExchangePair, failoverChannel chan string, wg *sync.WaitGroup) Scraper {
	defer wg.Done()
	var lock sync.RWMutex
	log.Info("CoinBase - Started scraper.")

	scraper := coinbaseScraper{
		tradesChannel:    make(chan models.Trade),
		subscribeChannel: make(chan models.ExchangePair),
		tickerPairMap:    models.MakeTickerPairMap(pairs),
		lastTradeTimeMap: make(map[string]time.Time),
		maxErrCount:      20,
		restartWaitTime:  5,
		genesis:          time.Now(),
	}

	// Dial websocket API.
	var wsDialer ws.Dialer
	wsClient, _, err := wsDialer.Dial(coinbaseWSBaseString, nil)
	if err != nil {
		log.Errorf("CoinBase - Dial ws base string: %v.", err)
		failoverChannel <- string(COINBASE_EXCHANGE)
		return &scraper
	}
	scraper.wsClient = wsClient

	// Subscribe to pairs and initialize coinbaseLastTradeTimeMap.
	for _, pair := range pairs {
		if err := scraper.subscribe(pair, true, &lock); err != nil {
			log.Errorf("CoinBase - subscribe to pair %s: %v.", pair.ForeignName, err)
		} else {
			log.Debugf("CoinBase - Subscribed to pair %s:%s.", COINBASE_EXCHANGE, pair.ForeignName)
			scraper.lastTradeTimeMap[pair.ForeignName] = time.Now()
		}
	}

	go scraper.fetchTrades()

	// Check last trade time for each subscribed pair and resubscribe if no activity for more than @coinbaseWatchdogDelayMap.
	for _, pair := range pairs {
		envVar := strings.ToUpper(COINBASE_EXCHANGE) + "_WATCHDOG_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[0] + "_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[1]
		watchdogDelay, err := strconv.ParseInt(utils.Getenv(envVar, "60"), 10, 64)
		if err != nil {
			log.Errorf("CoinBase - Parse coinbaseWatchdogDelay: %v.", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
		go watchdog(ctx, pair, watchdogTicker, scraper.lastTradeTimeMap, watchdogDelay, scraper.subscribeChannel, &lock)
		go scraper.resubscribe(ctx, &lock)
	}

	return &scraper
}

func (scraper *coinbaseScraper) Close(cancel context.CancelFunc) error {
	log.Warn("CoinBase - call scraper.Close().")
	cancel()
	return scraper.wsClient.Close()
}

func (scraper *coinbaseScraper) TradesChannel() chan models.Trade {
	return scraper.tradesChannel
}

func (scraper *coinbaseScraper) fetchTrades() {
	// Read trades stream.
	var errCount int

	for {

		var message coinBaseWSResponse
		err := scraper.wsClient.ReadJSON(&message)
		if err != nil {
			if handleErrorReadJSON(err, &errCount, scraper.maxErrCount, COINBASE_EXCHANGE, scraper.restartWaitTime) {
				return
			}
			continue
		}

		if message.Type == "match" {
			scraper.handleWSResponse(message)
		}

	}

}

func (scraper *coinbaseScraper) handleWSResponse(message coinBaseWSResponse) {
	trade, err := coinbaseParseTradeMessage(message)
	if err != nil {
		log.Errorf("CoinBase - parseCoinBaseTradeMessage: %s.", err.Error())
		return
	}

	// Identify ticker symbols with underlying assets.
	pair := strings.Split(message.ProductID, "-")
	if len(pair) > 1 {
		trade.QuoteToken = scraper.tickerPairMap[pair[0]+pair[1]].QuoteToken
		trade.BaseToken = scraper.tickerPairMap[pair[0]+pair[1]].BaseToken
	}

	log.Tracef("CoinBase - got trade: %s -- %v -- %v -- %s.", trade.QuoteToken.Symbol+"-"+trade.BaseToken.Symbol, trade.Price, trade.Volume, trade.ForeignTradeID)
	scraper.lastTradeTimeMap[pair[0]+"-"+pair[1]] = trade.Time

	scraper.tradesChannel <- trade

}

func (scraper *coinbaseScraper) resubscribe(ctx context.Context, lock *sync.RWMutex) {
	for {
		select {
		case pair := <-scraper.subscribeChannel:
			err := scraper.subscribe(pair, false, lock)
			if err != nil {
				log.Errorf("CoinBase - Unsubscribe pair %s: %v.", pair.ForeignName, err)
			} else {
				log.Debugf("CoinBase - Unsubscribed pair %s.", pair.ForeignName)
			}
			time.Sleep(2 * time.Second)
			err = scraper.subscribe(pair, true, lock)
			if err != nil {
				log.Errorf("CoinBase - Resubscribe pair %s: %v.", pair.ForeignName, err)
			} else {
				log.Debugf("CoinBase - Subscribed to pair %s.", pair.ForeignName)
			}
		case <-ctx.Done():
			log.Debugf("CoinBase - Close resubscribe routine of scraper with genesis: %v.", scraper.genesis)
			return
		}
	}
}

func (scraper *coinbaseScraper) subscribe(pair models.ExchangePair, subscribe bool, lock *sync.RWMutex) error {
	defer lock.Unlock()
	subscribeType := "unsubscribe"
	if subscribe {
		subscribeType = "subscribe"
	}
	a := &coinBaseWSSubscribeMessage{
		Type: subscribeType,
		Channels: []coinBaseChannel{
			{
				Name:       "matches",
				ProductIDs: []string{pair.ForeignName},
			},
		},
	}
	lock.Lock()
	return scraper.wsClient.WriteJSON(a)
}

func coinbaseParseTradeMessage(message coinBaseWSResponse) (models.Trade, error) {
	price, err := strconv.ParseFloat(message.Price, 64)
	if err != nil {
		return models.Trade{}, nil
	}
	volume, err := strconv.ParseFloat(message.Size, 64)
	if err != nil {
		return models.Trade{}, nil
	}
	if message.Side == "sell" {
		volume -= 1
	}
	timestamp, err := time.Parse("2006-01-02T15:04:05.000000Z", message.Time)
	if err != nil {
		return models.Trade{}, nil
	}

	foreignTradeID := strconv.Itoa(int(message.TradeID))

	trade := models.Trade{
		Price:          price,
		Volume:         volume,
		Time:           timestamp,
		Exchange:       models.Exchange{Name: COINBASE_EXCHANGE},
		ForeignTradeID: foreignTradeID,
	}

	return trade, nil
}
