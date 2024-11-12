package scrapers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
	ws "github.com/gorilla/websocket"
)

type binanceWSSubscribeMessage struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	ID     int      `json:"id"`
}

type binanceWSResponse struct {
	Timestamp      int64       `json:"T"`
	Price          string      `json:"p"`
	Volume         string      `json:"q"`
	ForeignTradeID int         `json:"t"`
	ForeignName    string      `json:"s"`
	Type           interface{} `json:"e"`
	Buy            bool        `json:"m"`
}

type binanceScraper struct {
	wsClient          *ws.Conn
	tradesChannel     chan models.Trade
	subscribeChannel  chan models.ExchangePair
	tickerPairMap     map[string]models.Pair
	lastTradeTimeMap  map[string]time.Time
	maxErrCount       int
	restartWaitTime   int
	genesis           time.Time
	apiConnectRetries int
	proxyIndex        int
}

const (
	BINANCE_API_MAX_RETRIES = 5
)

var (
	binanceWSBaseString   = "wss://stream.binance.com:9443/ws"
	binanceApiWaitSeconds = 5
)

func NewBinanceScraper(ctx context.Context, pairs []models.ExchangePair, failoverChannel chan string, wg *sync.WaitGroup) Scraper {
	defer wg.Done()
	var lock sync.RWMutex
	log.Infof("Binance - Started scraper at %v.", time.Now())

	scraper := binanceScraper{
		tradesChannel:    make(chan models.Trade),
		subscribeChannel: make(chan models.ExchangePair),
		tickerPairMap:    models.MakeTickerPairMap(pairs),
		lastTradeTimeMap: make(map[string]time.Time),
		maxErrCount:      20,
		restartWaitTime:  5,
		genesis:          time.Now(),
		proxyIndex:       0,
	}

	err := errors.New("cannot connect to API")
	var errCount int
	for err != nil {

		if errCount > 2*scraper.apiConnectRetries {
			failoverChannel <- BINANCE_EXCHANGE
			return &scraper
		}

		err = scraper.connectToAPI(pairs)
		if err != nil {
			errCount++
			scraper.apiConnectRetries++
			time.Sleep(time.Duration(binanceApiWaitSeconds) * time.Second)
		}
	}

	go scraper.fetchTrades(&lock)

	// Check last trade time for each subscribed pair and resubscribe if no activity for more than @binanceWatchdogDelay[pair].
	for _, pair := range pairs {
		envVar := strings.ToUpper(BINANCE_EXCHANGE) + "_WATCHDOG_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[0] + "_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[1]
		watchdogDelay, err := strconv.ParseInt(utils.Getenv(envVar, "60"), 10, 64)
		if err != nil {
			log.Errorf("Binance - Parse binanceWatchdogDelayMap: %v.", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
		go watchdog(ctx, pair, watchdogTicker, scraper.lastTradeTimeMap, watchdogDelay, scraper.subscribeChannel, &lock)
		go scraper.resubscribe(ctx, &lock)
	}

	return &scraper

}

func (scraper *binanceScraper) Close(cancel context.CancelFunc) error {
	log.Warn("Binance - call scraper.Close().")
	cancel()
	if scraper.wsClient == nil {
		return nil
	}
	return scraper.wsClient.Close()
}

func (scraper *binanceScraper) TradesChannel() chan models.Trade {
	return scraper.tradesChannel
}

func (scraper *binanceScraper) fetchTrades(lock *sync.RWMutex) {
	var errCount int

	for {

		var message binanceWSResponse
		err := scraper.wsClient.ReadJSON(&message)
		if err != nil {
			if handleErrorReadJSON(err, &errCount, scraper.maxErrCount, BINANCE_EXCHANGE, scraper.restartWaitTime) {
				return
			}
			continue
		}

		if message.Type == nil {
			continue
		}

		trade := binanceParseWSResponse(message)
		trade.QuoteToken = scraper.tickerPairMap[message.ForeignName].QuoteToken
		trade.BaseToken = scraper.tickerPairMap[message.ForeignName].BaseToken

		log.Tracef("Binance - got trade %s -- %v -- %v -- %v.", trade.QuoteToken.Symbol+"-"+trade.BaseToken.Symbol, trade.Price, trade.Volume, trade.ForeignTradeID)
		lock.Lock()
		scraper.lastTradeTimeMap[trade.QuoteToken.Symbol+"-"+trade.BaseToken.Symbol] = trade.Time
		lock.Unlock()

		scraper.tradesChannel <- trade
	}

}

func (scraper *binanceScraper) resubscribe(ctx context.Context, lock *sync.RWMutex) {
	for {
		select {
		case pair := <-scraper.subscribeChannel:
			log.Debugf("Binance - scraper with genesis %v: Resubscribe pair %s.", scraper.genesis, pair.ForeignName)
			err := scraper.subscribe(pair, false, lock)
			if err != nil {
				log.Errorf("Binance - scraper with genesis %v: Unsubscribe pair %s: %v.", scraper.genesis, pair.ForeignName, err)
			}
			time.Sleep(2 * time.Second)
			err = scraper.subscribe(pair, true, lock)
			if err != nil {
				log.Errorf("Binance - Resubscribe pair %s: %v.", pair.ForeignName, err)
			}
		case <-ctx.Done():
			log.Debugf("Binance - Close resubscribe routine of scraper with genesis: %v.", scraper.genesis)
			return
		}
	}
}

func (scraper *binanceScraper) subscribe(pair models.ExchangePair, subscribe bool, lock *sync.RWMutex) error {
	defer lock.Unlock()
	subscribeType := "UNSUBSCRIBE"
	if subscribe {
		subscribeType = "SUBSCRIBE"
	}
	pairTicker := strings.ToLower(strings.Split(pair.ForeignName, "-")[0] + strings.Split(pair.ForeignName, "-")[1])
	subscribeMessage := &binanceWSSubscribeMessage{
		Method: subscribeType,
		Params: []string{pairTicker + "@trade"},
		ID:     1,
	}
	lock.Lock()
	return scraper.wsClient.WriteJSON(subscribeMessage)
}

func (scraper *binanceScraper) connectToAPI(pairs []models.ExchangePair) error {

	// Switch to alternative Proxy whenever too many retries on the first.
	if scraper.apiConnectRetries > BINANCE_API_MAX_RETRIES {
		log.Errorf("too many timeouts for Binance api connection with proxy %v. Switch to alternative proxy.", scraper.proxyIndex)
		scraper.apiConnectRetries = 0
		scraper.proxyIndex = (scraper.proxyIndex + 1) % 2
	}

	username := utils.Getenv("BINANCE_PROXY"+strconv.Itoa(scraper.proxyIndex)+"_USERNAME", "")
	password := utils.Getenv("BINANCE_PROXY"+strconv.Itoa(scraper.proxyIndex)+"_PASSWORD", "")
	user := url.UserPassword(username, password)

	host := utils.Getenv("BINANCE_PROXY"+strconv.Itoa(scraper.proxyIndex)+"_HOST", "")

	log.Errorf("Binance username -- password -- user -- host: %s -- %s -- %s -- %s", username, password, user, host)

	var d = ws.Dialer{
		Proxy: http.ProxyURL(&url.URL{
			Scheme: "http", // or "https" depending on your proxy
			User:   user,
			Host:   host,
			Path:   "/",
		}),
	}

	wsAssetsString := ""
	for _, pair := range pairs {
		wsAssetsString += "/" + strings.ToLower(strings.Split(pair.ForeignName, "-")[0]) + strings.ToLower(strings.Split(pair.ForeignName, "-")[1]) + "@trade"
	}

	// Connect to Binance API.
	conn, _, err := d.Dial(binanceWSBaseString+wsAssetsString, nil)
	if err != nil {
		log.Errorf("Binance - Connect to API: %s.", err.Error())
		return err
	}
	scraper.wsClient = conn
	return nil

}

func binanceParseWSResponse(message binanceWSResponse) (trade models.Trade) {
	var err error
	trade.Exchange = models.Exchange{Name: BINANCE_EXCHANGE}
	trade.Time = time.Unix(0, message.Timestamp*1000000)
	trade.Price, err = strconv.ParseFloat(message.Price, 64)
	if err != nil {
		log.Errorf("Binance - Parse price: %v.", err)
	}
	trade.Volume, err = strconv.ParseFloat(message.Volume, 64)
	if err != nil {
		log.Errorf("Binance - Parse volume: %v.", err)
	}
	if !message.Buy {
		trade.Volume -= 1
	}
	trade.ForeignTradeID = strconv.Itoa(int(message.ForeignTradeID))
	return
}
