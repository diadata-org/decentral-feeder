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

// A krakenWSSubscribeMessage represents a message to subscribe the public/private channel.
type krakenWSSubscribeMessage struct {
	Method string       `json:"method"`
	Params krakenParams `json:"params"`
}

type krakenParams struct {
	Channel string   `json:"channel"`
	Symbol  []string `json:"symbol"`
}

type krakenWSResponse struct {
	Channel string                 `json:"channel"`
	Type    string                 `json:"type"`
	Data    []krakenWSResponseData `json:"data"`
}

type krakenWSResponseData struct {
	Symbol    string  `json:"symbol"`
	Side      string  `json:"side"`
	Price     float64 `json:"price"`
	Size      float64 `json:"qty"`
	OrderType string  `json:"order_type"`
	TradeID   int     `json:"trade_id"`
	Time      string  `json:"timestamp"`
}

var (
	krakenWSBaseString    = "wss://ws.kraken.com/v2"
	krakenMaxErrCount     = 20
	krakenRun             bool
	krakenWatchdogDelay   int64
	krakenRestartWaitTime = 5
	krakenLastTradeTime   time.Time
)

func init() {
	var err error
	krakenWatchdogDelay, err = strconv.ParseInt(utils.Getenv("KRAKEN_WATCHDOGDELAY", "60"), 10, 64)
	if err != nil {
		log.Error("Parse KRAKEN_WATCHDOGDELAY: ", err)
	}
}

func NewKrakenScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, failoverChannel chan string, wg *sync.WaitGroup) string {
	defer wg.Done()
	log.Info("Started Kraken scraper.")
	krakenRun = true
	tickerPairMap := models.MakeTickerPairMap(pairs)

	var wsDialer ws.Dialer
	wsClient, _, err := wsDialer.Dial(krakenWSBaseString, nil)
	if err != nil {
		log.Error("Dial Kraken ws base string: ", err)
		failoverChannel <- string(KRAKEN_EXCHANGE)
		return "closed"
	}

	// Subscribe to pairs.
	for _, pair := range pairs {
		a := &krakenWSSubscribeMessage{
			Method: "subscribe",
			Params: krakenParams{
				Channel: "trade",
				Symbol:  []string{pair.UnderlyingPair.QuoteToken.Symbol + "/" + pair.UnderlyingPair.BaseToken.Symbol},
			},
		}
		log.Infof("Subscribed for Pair %s:%s", KRAKEN_EXCHANGE, pair.ForeignName)
		if err := wsClient.WriteJSON(a); err != nil {
			log.Error("Kraken - " + err.Error())
		}
	}

	krakenLastTradeTime = time.Now()
	log.Info("Kraken - Initialize lastTradeTime after failover: ", krakenLastTradeTime)
	watchdogTicker := time.NewTicker(time.Duration(krakenWatchdogDelay) * time.Second)

	go globalWatchdog(watchdogTicker, &krakenLastTradeTime, krakenWatchdogDelay, &krakenRun)

	// Read trades stream.
	var errCount int
	for krakenRun {

		var message krakenWSResponse
		err = wsClient.ReadJSON(&message)
		if err != nil {
			readJSONError(KRAKEN_EXCHANGE, err, &errCount, &krakenRun, krakenRestartWaitTime, krakenMaxErrCount)
			continue
		}

		if message.Channel == "trade" {
			for _, data := range message.Data {

				// Parse trade quantities.
				price, volume, timestamp, foreignTradeID, err := parseKrakenTradeMessage(data)
				if err != nil {
					log.Error("Kraken - parseTradeMessage: ", err)
				}

				// Identify ticker symbols with underlying assets.
				pair := strings.Split(data.Symbol, "/")
				var exchangepair models.Pair
				if len(pair) > 1 {
					exchangepair = tickerPairMap[pair[0]+pair[1]]
				}

				trade := models.Trade{
					QuoteToken:     exchangepair.QuoteToken,
					BaseToken:      exchangepair.BaseToken,
					Price:          price,
					Volume:         volume,
					Time:           timestamp,
					Exchange:       models.Exchange{Name: KRAKEN_EXCHANGE},
					ForeignTradeID: foreignTradeID,
				}
				// log.Info("Got trade: ", trade)
				krakenLastTradeTime = trade.Time
				tradesChannel <- trade
			}
		}
	}

	log.Warn("Close Kraken scraper.")
	failoverChannel <- string(KRAKEN_EXCHANGE)
	return "closed"

}

func parseKrakenTradeMessage(message krakenWSResponseData) (price float64, volume float64, timestamp time.Time, foreignTradeID string, err error) {
	price = message.Price
	volume = message.Size
	if message.Side == "sell" {
		volume -= 1
	}
	timestamp, err = time.Parse("2006-01-02T15:04:05.000000Z", message.Time)
	if err != nil {
		return
	}

	foreignTradeID = strconv.Itoa(message.TradeID)
	return
}
