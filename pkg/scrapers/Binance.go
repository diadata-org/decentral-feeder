package scrapers

import (
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

var (
	binanceWSBaseString     = "wss://stream.binance.com:9443/ws"
	binanceMaxErrCount      = 20
	binanceWatchdogDelay    int64
	binanceRestartWaitTime  = 5
	binanceRun              bool
	binanceLastTradeTime    time.Time
	binanceLastTradeTimeMap = make(map[string]time.Time)
	binanceSubscribeChannel = make(chan models.ExchangePair)
)

func init() {
	var err error
	binanceWatchdogDelay, err = strconv.ParseInt(utils.Getenv("BINANCE_WATCHDOGDELAY", "60"), 10, 64)
	if err != nil {
		log.Error("Parse BINANCE_WATCHDOGDELAY: ", err)
	}
}

func NewBinanceScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, failoverChannel chan string, wg *sync.WaitGroup) string {
	defer wg.Done()
	defer close(binanceSubscribeChannel)
	var lock sync.RWMutex
	log.Info("Started Binance scraper at: ", time.Now())
	binanceRun = true
	// Make tickerPairMap for identification of exchangepairs.
	tickerPairMap := models.MakeTickerPairMap(pairs)

	// Establish a connection to binance websocket API.
	conn, _, err := ws.DefaultDialer.Dial(binanceWSBaseString, nil)
	if err != nil {
		log.Errorf("Connect to Binance API: %s.", err.Error())
		failoverChannel <- string(BINANCE_EXCHANGE)
		return "closed"
	}
	defer conn.Close()

	//Subscribe to pairs.
	for _, pair := range pairs {
		err = binanceSubscribe(pair, &lock, conn)
		if err != nil {
			log.Errorf("Subscribe to %s: %v", pair.ForeignName, err)
		}
	}

	// Check last trade time across all pairs and restart the scraper if no activity for more than @binanceWatchdogDelay.
	binanceLastTradeTime = time.Now()
	log.Info("Binance - Initialize lastTradeTime after failover: ", binanceLastTradeTime)
	watchdogTicker := time.NewTicker(time.Duration(binanceWatchdogDelay) * time.Second)
	log.Info("watchdogDelay: ", time.Duration(binanceWatchdogDelay)*time.Second)
	go globalWatchdog(watchdogTicker, &binanceLastTradeTime, binanceWatchdogDelay, &binanceRun)

	// Check last trade time for each subscribed pair and resubscribe if no activity for more than @binanceWatchdogDelay[pair].
	for _, pair := range pairs {
		envVar := strings.ToUpper(BINANCE_EXCHANGE) + "_WATCHDOG_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[0] + "_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[1]
		binanceWatchdogDelay, err = strconv.ParseInt(utils.Getenv(envVar, "60"), 10, 64)
		if err != nil {
			log.Error("Parse binanceWatchdogDelayMap: ", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(binanceWatchdogDelay) * time.Second)
		go watchdog(pair, watchdogTicker, binanceLastTradeTimeMap, binanceWatchdogDelay, binanceSubscribeChannel, &lock)
		go binanceResubscribe(binanceSubscribeChannel, &lock, conn)
	}

	var errCount int
	for binanceRun {

		var message binanceWSResponse
		err := conn.ReadJSON(&message)
		if err != nil {
			readJSONError(BINANCE_EXCHANGE, err, &errCount, &binanceRun, binanceRestartWaitTime, binanceMaxErrCount)
			continue
		}

		if message.Type == nil {
			log.Warn("subscribe message: ", message)
			continue
		}

		trade := binanceParseWSResponse(message)
		trade.QuoteToken = tickerPairMap[message.ForeignName].QuoteToken
		trade.BaseToken = tickerPairMap[message.ForeignName].BaseToken
		binanceLastTradeTime = trade.Time
		binanceLastTradeTimeMap[trade.QuoteToken.Symbol+"-"+trade.BaseToken.Symbol] = trade.Time
		// log.Infof("Got trade %s -- %v -- %v", trade.QuoteToken.Symbol+"-"+trade.BaseToken.Symbol, trade.Price, trade.Volume)
		tradesChannel <- trade

	}

	log.Warn("Close Binance scraper.")
	failoverChannel <- string(BINANCE_EXCHANGE)
	return "closed"
}

func binanceResubscribe(subscribeChannel chan models.ExchangePair, lock *sync.RWMutex, conn *ws.Conn) {
	for {
		select {
		case pair := <-subscribeChannel:
			err := binanceUnsubscribe(pair, lock, conn)
			if err != nil {
				log.Errorf("binance - Unsubscribe pair %s: %v", pair.ForeignName, err)
			}
			time.Sleep(2 * time.Second)
			err = binanceSubscribe(pair, lock, conn)
			if err != nil {
				log.Errorf("binance - Resubscribe pair %s: %v", pair.ForeignName, err)
			}
		}
	}
}

func binanceSubscribe(pair models.ExchangePair, lock *sync.RWMutex, wsClient *ws.Conn) error {
	defer lock.Unlock()
	pairTicker := strings.ToLower(strings.Split(pair.ForeignName, "-")[0] + strings.Split(pair.ForeignName, "-")[1])
	subscribeMessage := &binanceWSSubscribeMessage{
		Method: "SUBSCRIBE",
		Params: []string{pairTicker + "@trade"},
		ID:     1,
	}
	lock.Lock()
	return wsClient.WriteJSON(subscribeMessage)
}

func binanceUnsubscribe(pair models.ExchangePair, lock *sync.RWMutex, wsClient *ws.Conn) error {
	defer lock.Unlock()
	pairTicker := strings.ToLower(strings.Split(pair.ForeignName, "-")[0] + strings.Split(pair.ForeignName, "-")[1])
	unsubscribeMessage := &binanceWSSubscribeMessage{
		Method: "UNSUBSCRIBE",
		Params: []string{pairTicker + "@trade"},
		ID:     1,
	}
	lock.Lock()
	return wsClient.WriteJSON(unsubscribeMessage)
}

func binanceParseWSResponse(message binanceWSResponse) (trade models.Trade) {
	var err error
	trade.Exchange = models.Exchange{Name: BINANCE_EXCHANGE}
	trade.Time = time.Unix(0, message.Timestamp*1000000)
	trade.Price, err = strconv.ParseFloat(message.Price, 64)
	if err != nil {
		log.Error("Binance - Parse price: ", err)
	}
	trade.Volume, err = strconv.ParseFloat(message.Volume, 64)
	if err != nil {
		log.Error("Binance - Parse volume: ", err)
	}
	if !message.Buy {
		trade.Volume -= 1
	}
	trade.ForeignTradeID = strconv.Itoa(int(message.ForeignTradeID))
	return
}
