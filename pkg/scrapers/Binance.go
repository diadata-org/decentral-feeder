package scrapers

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	ws "github.com/gorilla/websocket"
)

var (
	binanceWSBaseString  = "wss://stream.binance.com:9443/ws/"
	watchdogDelayBinance = 60
	lastTradeTime        time.Time
	run                  bool
)

func NewBinanceScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, failoverChannel chan string, wg *sync.WaitGroup) string {
	run = true
	defer wg.Done()
	log.Info("Started Binance scraper at: ", time.Now())

	wsAssetsString := ""
	for _, pair := range pairs {
		wsAssetsString += strings.ToLower(strings.Split(pair.ForeignName, "-")[0]) + strings.ToLower(strings.Split(pair.ForeignName, "-")[1]) + "@trade" + "/"
	}

	// Make tickerPairMap for identification of exchangepairs.
	tickerPairMap := models.MakeTickerPairMap(pairs)

	// Remove trailing slash
	wsAssetsString = wsAssetsString[:len(wsAssetsString)-1]
	conn, _, err := ws.DefaultDialer.Dial(binanceWSBaseString+wsAssetsString, nil)
	if err != nil {
		log.Error("connect to Binance API.")
	}
	defer conn.Close()

	lastTradeTime = time.Now()
	log.Info("Initialize lastTradeTime after failover: ", lastTradeTime)
	watchdogTicker := time.NewTicker(time.Duration(watchdogDelayBinance) * time.Second)

	// Check for liveliness of the scraper.
	// More precisely, if there is no trades for a period longer than @watchdogDelayBinance the scraper is stopped
	// and the exchange name is sent to the failover channel.
	go func() {
		for range watchdogTicker.C {
			log.Info("watchdogTicker - lastTradeTime: ", lastTradeTime)
			log.Info("watchdogTicker - timeNow: ", time.Now())
			duration := time.Since(lastTradeTime)
			if duration > time.Duration(watchdogDelayBinance)*time.Second {
				log.Error("watchdogTicker - failover")
				run = false
				break
			}
		}
	}()

	for run {

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Errorln("read:", err)
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
			log.Error("Parse price: ", err)
		}

		trade.Volume, err = strconv.ParseFloat(messageMap["q"].(string), 64)
		if err != nil {
			log.Error("Parse volume: ", err)
		}
		if !messageMap["m"].(bool) {
			trade.Volume -= 1
		}

		if messageMap["t"] != nil {
			trade.ForeignTradeID = strconv.Itoa(int(messageMap["t"].(float64)))
		}

		trade.QuoteToken = tickerPairMap[messageMap["s"].(string)].QuoteToken
		trade.BaseToken = tickerPairMap[messageMap["s"].(string)].BaseToken

		lastTradeTime = trade.Time

		// Send message to @failoverChannel in case there is no trades for at least @watchdogDelayBinance seconds.
		log.Infof("%v -- Got trade: time -- price -- ID: %v -- %v -- %s", time.Now(), trade.Time, trade.Price, trade.ForeignTradeID)

		tradesChannel <- trade

	}

	log.Warn("Close Binance scraper.")
	failoverChannel <- string(BINANCE_EXCHANGE)
	return "closed"

}
