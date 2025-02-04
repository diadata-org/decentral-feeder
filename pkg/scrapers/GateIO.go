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

type SubscribeGate struct {
	Time    int64    `json:"time"`
	Channel string   `json:"channel"`
	Event   string   `json:"event"`
	Payload []string `json:"payload"`
}

type GateIOResponseTrade struct {
	Time    int    `json:"time"`
	Channel string `json:"channel"`
	Event   string `json:"event"`
	Result  struct {
		ID           int    `json:"id"`
		CreateTime   int    `json:"create_time"`
		CreateTimeMs string `json:"create_time_ms"`
		Side         string `json:"side"`
		CurrencyPair string `json:"currency_pair"`
		Amount       string `json:"amount"`
		Price        string `json:"price"`
	} `json:"result"`
}

type gateIOScraper struct {
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
	_GateIOsocketurl string = "wss://api.gateio.ws/ws/v4/"
)

func NewGateIOScraper(ctx context.Context, pairs []models.ExchangePair, failoverChannel chan string, wg *sync.WaitGroup) Scraper {
	defer wg.Done()
	var lock sync.RWMutex
	log.Info("GateIO - Started scraper.")

	scraper := gateIOScraper{
		tradesChannel:    make(chan models.Trade),
		subscribeChannel: make(chan models.ExchangePair),
		tickerPairMap:    models.MakeTickerPairMap(pairs),
		lastTradeTimeMap: make(map[string]time.Time),
		maxErrCount:      20,
		restartWaitTime:  5,
		genesis:          time.Now(),
	}

	var wsDialer ws.Dialer
	wsClient, _, err := wsDialer.Dial(_GateIOsocketurl, nil)
	if err != nil {
		log.Errorf("GateIO - Dial ws base string: %s." + err.Error())
		failoverChannel <- string(GATEIO_EXCHANGE)
		return &scraper
	}
	scraper.wsClient = wsClient

	for _, pair := range pairs {
		if err := scraper.subscribe(pair, true, &lock); err != nil {
			log.Errorf("GateIO - subscribe to pair %s: %v.", pair.ForeignName, err)
		} else {
			log.Debugf("GateIO - Subscribed to pair %s.", pair.ForeignName)
			scraper.lastTradeTimeMap[pair.ForeignName] = time.Now()
		}
	}

	go scraper.fetchTrades(&lock)

	// Check last trade time for each subscribed pair and resubscribe if no activity for more than @gateIOWatchdogDelayMap.
	for _, pair := range pairs {
		envVar := strings.ToUpper(GATEIO_EXCHANGE) + "_WATCHDOG_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[0] + "_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[1]
		watchdogDelay, err := strconv.ParseInt(utils.Getenv(envVar, "300"), 10, 64)
		if err != nil {
			log.Errorf("GateIO - Parse gateIOWatchdogDelay: %v.", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
		go watchdog(ctx, pair, watchdogTicker, scraper.lastTradeTimeMap, watchdogDelay, scraper.subscribeChannel, &lock)
		go scraper.resubscribe(ctx, &lock)
	}

	return &scraper

}

func (scraper *gateIOScraper) Close(cancel context.CancelFunc) error {
	log.Warn("GateIO - call scraper.Close().")
	cancel()
	if scraper.wsClient == nil {
		return nil
	}
	return scraper.wsClient.Close()
}

func (scraper *gateIOScraper) TradesChannel() chan models.Trade {
	return scraper.tradesChannel
}

func (scraper *gateIOScraper) fetchTrades(lock *sync.RWMutex) {
	var errCount int
	for {

		var message GateIOResponseTrade
		if err := scraper.wsClient.ReadJSON(&message); err != nil {
			if handleErrorReadJSON(err, &errCount, scraper.maxErrCount, GATEIO_EXCHANGE, scraper.restartWaitTime) {
				return
			}
			continue
		}

		trade := scraper.handleWSResponse(message)

		lock.Lock()
		scraper.lastTradeTimeMap[trade.QuoteToken.Symbol+"-"+trade.BaseToken.Symbol] = trade.Time
		lock.Unlock()

		log.Tracef("GateIO - got trade: %s -- %v -- %v -- %s.", trade.QuoteToken.Symbol+"-"+trade.BaseToken.Symbol, trade.Price, trade.Volume, trade.ForeignTradeID)

		scraper.tradesChannel <- trade

	}
}

func (scraper *gateIOScraper) handleWSResponse(message GateIOResponseTrade) models.Trade {
	var (
		f64Price     float64
		f64Volume    float64
		exchangepair models.Pair
		err          error
	)

	f64Price, err = strconv.ParseFloat(message.Result.Price, 64)
	if err != nil {
		log.Errorf("GateIO - error parsing float Price %v: %v.", message.Result.Price, err)
		return models.Trade{}
	}

	f64Volume, err = strconv.ParseFloat(message.Result.Amount, 64)
	if err != nil {
		log.Errorf("GateIO - error parsing float Volume %v: %v.", message.Result.Amount, err)
		return models.Trade{}
	}

	if message.Result.Side == "sell" {
		f64Volume = -f64Volume
	}
	exchangepair = scraper.tickerPairMap[strings.Split(message.Result.CurrencyPair, "_")[0]+strings.Split(message.Result.CurrencyPair, "_")[1]]

	t := models.Trade{
		QuoteToken:     exchangepair.QuoteToken,
		BaseToken:      exchangepair.BaseToken,
		Price:          f64Price,
		Volume:         f64Volume,
		Time:           time.Unix(int64(message.Result.CreateTime), 0),
		Exchange:       Exchanges[GATEIO_EXCHANGE],
		ForeignTradeID: strconv.FormatInt(int64(message.Result.ID), 16),
	}

	return t
}

func (scraper *gateIOScraper) resubscribe(ctx context.Context, lock *sync.RWMutex) {
	for {
		select {
		case pair := <-scraper.subscribeChannel:
			err := scraper.subscribe(pair, false, lock)
			if err != nil {
				log.Errorf("GateIO - Unsubscribe pair %s: %v.", pair.ForeignName, err)
			} else {
				log.Debugf("GateIO - Unsubscribed pair %s.", pair.ForeignName)
			}
			time.Sleep(2 * time.Second)
			err = scraper.subscribe(pair, true, lock)
			if err != nil {
				log.Errorf("GateIO - Resubscribe pair %s: %v", pair.ForeignName, err)
			} else {
				log.Debugf("GateIO - Subscribed to pair %s.", pair.ForeignName)
			}
		case <-ctx.Done():
			log.Warnf("GateIO - Close resubscribe routine of scraper with genesis: %v.", scraper.genesis)
			return
		}
	}
}

func (scraper *gateIOScraper) subscribe(pair models.ExchangePair, subscribe bool, lock *sync.RWMutex) error {
	defer lock.Unlock()
	gateioPairTicker := strings.Split(pair.ForeignName, "-")[0] + "_" + strings.Split(pair.ForeignName, "-")[1]
	subscribeType := "unsubscribe"
	if subscribe {
		subscribeType = "subscribe"
	}
	a := &SubscribeGate{
		Event:   subscribeType,
		Time:    time.Now().Unix(),
		Channel: "spot.trades",
		Payload: []string{gateioPairTicker},
	}
	lock.Lock()
	return scraper.wsClient.WriteJSON(a)
}
