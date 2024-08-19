package scrapers

import (
	"encoding/json"
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

var (
	binanceWSBaseString    = "wss://stream.binance.com:9443/ws/"
	binanceMaxErrCount     = 20
	binanceWatchdogDelay   int64
	binanceLastTradeTime   time.Time
	binanceRestartWaitTime = 5
	binanceRun             bool
)

func NewBinanceScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, failoverChannel chan string, wg *sync.WaitGroup) string {
	binanceRun = true
	defer wg.Done()
	log.Info("Started Binance scraper at: ", time.Now())

	wsAssetsString := ""
	for _, pair := range pairs {
		wsAssetsString += strings.ToLower(strings.Split(pair.ForeignName, "-")[0]) + strings.ToLower(strings.Split(pair.ForeignName, "-")[1]) + "@trade" + "/"
	}

	// Make tickerPairMap for identification of exchangepairs.
	tickerPairMap := models.MakeTickerPairMap(pairs)

	// Set up websocket dialer with proxy.
	proxyURL, err := url.Parse(utils.Getenv("BINANCE_PROXY_URL", "http://samuelbrack:hD3bfFBVLg@193.124.16.228:50100"))
	if err != nil {
		log.Error("Binance - parse proxy url: %v", err)
	}

	var d = ws.Dialer{
		Proxy: http.ProxyURL(&url.URL{
			Scheme: "http", // or "https" depending on your proxy
			User:   proxyURL.User,
			Host:   proxyURL.Host,
			Path:   "/",
		}),
	}

	// pw, _ := u.User.Password()
	// log.Infof("user -- password: %s -- %s", u.User.Username(), pw)
	// conn1, _, err := ws.DefaultDialer.Dial(binanceWSBaseString+wsAssetsString, nil)
	// if err != nil {
	// 	log.Error("DefaultDialer: ", err)
	// }
	// log.Info("conn1: ", conn1)

	// Connect to Binance API.
	conn, _, err := d.Dial(binanceWSBaseString+wsAssetsString, nil)
	if err != nil {
		log.Errorf("Connect to Binance API: %s.", err.Error())
		failoverChannel <- string(BINANCE_EXCHANGE)
		return "closed"
	}

	defer conn.Close()

	binanceLastTradeTime = time.Now()
	log.Info("Binance - Initialize lastTradeTime after failover: ", binanceLastTradeTime)
	watchdogTicker := time.NewTicker(time.Duration(binanceWatchdogDelay) * time.Second)
	log.Info("watchdogDelay: ", time.Duration(binanceWatchdogDelay)*time.Second)

	// Check for liveliness of the scraper.
	// More precisely, if there is no trades for a period longer than @watchdogDelayBinance the scraper is stopped
	// and the exchange name is sent to the failover channel.
	go globalWatchdog(watchdogTicker, &binanceLastTradeTime, binanceWatchdogDelay, &binanceRun)

	var errCount int
	for binanceRun {

		_, message, err := conn.ReadMessage()
		if err != nil {
			readJSONError(BINANCE_EXCHANGE, err, &errCount, &binanceRun, binanceRestartWaitTime, binanceMaxErrCount)
			continue
		}

		messageMap := make(map[string]interface{})
		err = json.Unmarshal(message, &messageMap)
		if err != nil {
			continue
		}

		var trade models.Trade

		trade.Exchange = models.Exchange{Name: BINANCE_EXCHANGE}
		trade.Time = time.Unix(int64(messageMap["T"].(float64))/1000, 0)
		// TO DO: Improve parsing of timestamp

		trade.Price, err = strconv.ParseFloat(messageMap["p"].(string), 64)
		if err != nil {
			log.Error("Binance - Parse price: ", err)
		}

		trade.Volume, err = strconv.ParseFloat(messageMap["q"].(string), 64)
		if err != nil {
			log.Error("Binance - Parse volume: ", err)
		}
		if !messageMap["m"].(bool) {
			trade.Volume -= 1
		}

		if messageMap["t"] != nil {
			trade.ForeignTradeID = strconv.Itoa(int(messageMap["t"].(float64)))
		}

		trade.QuoteToken = tickerPairMap[messageMap["s"].(string)].QuoteToken
		trade.BaseToken = tickerPairMap[messageMap["s"].(string)].BaseToken

		binanceLastTradeTime = trade.Time

		log.Infof("%v -- Got trade: time -- price -- ID: %v -- %v -- %s", time.Now(), trade.Time, trade.Price, trade.ForeignTradeID)
		tradesChannel <- trade

	}

	log.Warn("Close Binance scraper.")
	failoverChannel <- string(BINANCE_EXCHANGE)
	return "closed"

}
