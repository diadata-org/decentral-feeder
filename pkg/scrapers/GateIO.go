package scrapers

import (
	"strconv"
	"strings"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	ws "github.com/gorilla/websocket"
)

var _GateIOsocketurl string = "wss://api.gateio.ws/ws/v4/"

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

func NewGateIOScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Info("Started GateIO scraper.")

	var wsDialer ws.Dialer
	wsClient, _, err := wsDialer.Dial(_GateIOsocketurl, nil)
	if err != nil {
		println(err.Error())
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
		log.Infof("Subscribed for Pair %v", pair.ForeignName)
		if err := wsClient.WriteJSON(a); err != nil {
			log.Error(err.Error())
		}
	}

	for {

		var message GateIOResponseTrade
		if err = wsClient.ReadJSON(&message); err != nil {
			log.Error(err.Error())
			break
		}

		var (
			f64Price     float64
			f64Volume    float64
			exchangepair models.Pair
		)

		f64Price, err = strconv.ParseFloat(message.Result.Price, 64)
		if err != nil {
			log.Errorf("error parsing float Price %v: %v", message.Result.Price, err)
			continue
		}

		f64Volume, err = strconv.ParseFloat(message.Result.Amount, 64)
		if err != nil {
			log.Errorln("error parsing float Volume", err)
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

		log.Info("Got trade: ", t)
		tradesChannel <- t

	}

}
