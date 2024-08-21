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

func init() {
	var err error
	binanceWatchdogDelay, err = strconv.ParseInt(utils.Getenv("BINANCE_WATCHDOGDELAY", "60"), 10, 64)
	if err != nil {
		log.Error("Parse BINANCE_WATCHDOGDELAY: ", err)
	}
}

func NewBinanceScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, failoverChannel chan string, wg *sync.WaitGroup) string {
	defer wg.Done()
	log.Info("Started Binance scraper at: ", time.Now())
	binanceRun = true
	// Make tickerPairMap for identification of exchangepairs.
	tickerPairMap := models.MakeTickerPairMap(pairs)

	wsAssetsString := ""
	for _, pair := range pairs {
		wsAssetsString += strings.ToLower(strings.Split(pair.ForeignName, "-")[0]) + strings.ToLower(strings.Split(pair.ForeignName, "-")[1]) + "@trade" + "/"
	}
	wsAssetsString = wsAssetsString[:len(wsAssetsString)-1]

	// Set up websocket dialer with proxy.
	proxyURL, err := url.Parse(utils.Getenv("BINANCE_PROXY_URL", ""))
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

	// Check for liveliness of the scraper.
	// More precisely, if there is no trades for a period longer than @watchdogDelayBinance the scraper is stopped
	// and the exchange name is sent to the failover channel.
	go func() {
		for range watchdogTicker.C {
			duration := time.Since(binanceLastTradeTime)
			if duration > time.Duration(binanceWatchdogDelay)*time.Second {
				log.Error("Binance - watchdogTicker failover")
				binanceRun = false
				break
			}
		}
	}()

	var errCount int
	for binanceRun {

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Errorln("Binance - ReadMessage:", err)
			errCount++
			if errCount > binanceMaxErrCount {
				log.Warnf("too many errors. wait for %v seconds and restart scraper.", binanceRestartWaitTime)
				time.Sleep(time.Duration(binanceRestartWaitTime) * time.Second)
				binanceRun = false
				break
			}
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

		// log.Infof("%v -- Got trade: time -- price -- ID: %v -- %v -- %s", time.Now(), trade.Time, trade.Price, trade.ForeignTradeID)
		tradesChannel <- trade

	}

	log.Warn("Close Binance scraper.")
	failoverChannel <- string(BINANCE_EXCHANGE)
	return "closed"

}
