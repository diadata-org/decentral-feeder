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

var (
	_GateIOsocketurl      string = "wss://api.gateio.ws/ws/v4/"
	gateIOMaxErrCount            = 20
	gateIORun             bool
	gateIOWatchdogDelay   int64
	gateIORestartWaitTime = 5
	gateIOLastTradeTime   time.Time
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

func init() {
	var err error
	gateIOWatchdogDelay, err = strconv.ParseInt(utils.Getenv("GATEIO_WATCHDOGDELAY", "60"), 10, 64)
	if err != nil {
		log.Error("Parse GATEIO_WATCHDOGDELAY: ", err)
	}
}

func NewGateIOScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, failoverChannel chan string, wg *sync.WaitGroup) string {
	defer wg.Done()
	log.Info("Started GateIO scraper.")
	gateIORun = true

	var wsDialer ws.Dialer
	wsClient, _, err := wsDialer.Dial(_GateIOsocketurl, nil)
	if err != nil {
		log.Error("Dial GateIO ws base string: " + err.Error())
		failoverChannel <- string(GATEIO_EXCHANGE)
		return "closed"
	}

	// In case this is the same for all exchanges we can put it to APIScraper.go.
	tickerPairMap := models.MakeTickerPairMap(pairs)

	for _, pair := range pairs {
		gateioPairTicker := strings.Split(pair.ForeignName, "-")[0] + "_" + strings.Split(pair.ForeignName, "-")[1]

		a := &SubscribeGate{
			Event:   "subscribe",
			Time:    time.Now().Unix(),
			Channel: "spot.trades",
			Payload: []string{gateioPairTicker},
		}
		log.Infof("GateIO - Subscribed for Pair %v", pair.ForeignName)
		if err := wsClient.WriteJSON(a); err != nil {
			log.Error("GateIO - WriteJSON: " + err.Error())
		}
	}

	gateIOLastTradeTime = time.Now()
	log.Info("GateIO - Initialize lastTradeTime after failover: ", gateIOLastTradeTime)
	watchdogTicker := time.NewTicker(time.Duration(gateIOWatchdogDelay) * time.Second)

	go func() {
		for range watchdogTicker.C {
			log.Info("GateIO - watchdogTicker - lastTradeTime: ", gateIOLastTradeTime)
			log.Info("GateIO - watchdogTicker - timeNow: ", time.Now())
			duration := time.Since(gateIOLastTradeTime)
			if duration > time.Duration(gateIOWatchdogDelay)*time.Second {
				log.Error("GateIO - watchdogTicker failover")
				gateIORun = false
				break
			}
		}
	}()

	var errCount int
	for gateIORun {

		var message GateIOResponseTrade
		if err = wsClient.ReadJSON(&message); err != nil {
			log.Error("GateIO - readJSON: " + err.Error())
			errCount++
			if errCount > gateIOMaxErrCount {
				log.Warnf("too many errors. wait for %v seconds and restart scraper.", gateIORestartWaitTime)
				time.Sleep(time.Duration(gateIORestartWaitTime) * time.Second)
				gateIORun = false
				break
			}
			continue
		}

		var (
			f64Price     float64
			f64Volume    float64
			exchangepair models.Pair
		)

		f64Price, err = strconv.ParseFloat(message.Result.Price, 64)
		if err != nil {
			log.Errorf("GateIO - error parsing float Price %v: %v", message.Result.Price, err)
			continue
		}

		f64Volume, err = strconv.ParseFloat(message.Result.Amount, 64)
		if err != nil {
			log.Errorln("GateIO - error parsing float Volume", err)
			continue
		}

		if message.Result.Side == "sell" {
			f64Volume = -f64Volume
		}
		exchangepair = tickerPairMap[strings.Split(message.Result.CurrencyPair, "_")[0]+strings.Split(message.Result.CurrencyPair, "_")[1]]

		t := models.Trade{
			QuoteToken:     exchangepair.QuoteToken,
			BaseToken:      exchangepair.BaseToken,
			Price:          f64Price,
			Volume:         f64Volume,
			Time:           time.Unix(int64(message.Result.CreateTime), 0),
			Exchange:       models.Exchange{Name: GATEIO_EXCHANGE},
			ForeignTradeID: strconv.FormatInt(int64(message.Result.ID), 16),
		}

		// log.Info("Got trade: ", t)
		gateIOLastTradeTime = t.Time
		tradesChannel <- t

	}

	log.Warn("Close GateIO scraper.")
	failoverChannel <- string(GATEIO_EXCHANGE)
	return "closed"

}
