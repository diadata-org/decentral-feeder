package scrapers

import (
	"strconv"
	"strings"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
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

var (
	coinbaseWSBaseString = "wss://ws-feed.exchange.coinbase.com"
)

func NewCoinBaseScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Info("Started CoinBase scraper.")

	var wsDialer ws.Dialer
	wsClient, _, err := wsDialer.Dial(coinbaseWSBaseString, nil)
	if err != nil {
		log.Error("Dial CoinBase ws base string: ", err)
	}

	// Subscribe to pairs.
	for _, pair := range pairs {
		a := &coinBaseWSSubscribeMessage{
			Type: "subscribe",
			Channels: []coinBaseChannel{
				{
					Name:       "matches",
					ProductIDs: []string{pair.ForeignName},
				},
			},
		}
		log.Infof("Subscribed for Pair %s:%s", COINBASE_EXCHANGE, pair.ForeignName)
		if err := wsClient.WriteJSON(a); err != nil {
			log.Error(err.Error())
		}
	}

	// Read trades stream.
	for {
		var message coinBaseWSResponse
		err = wsClient.ReadJSON(&message)
		if err != nil {
			log.Errorf("ReadMessage: %v", err)
			continue
		}

		if message.Type == "match" {

			// Parse trade quantities.
			price, volume, timestamp, foreignTradeID, err := parseCoinBaseTradeMessage(message)
			if err != nil {
				log.Error("parseTradeMessage: ", err)
			}

			// Identify ticker symbols with underlying assets.
			tickerPairMap := models.MakeTickerPairMap(pairs)
			pair := strings.Split(message.ProductID, "-")
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
				Exchange:       models.Exchange{Name: COINBASE_EXCHANGE},
				ForeignTradeID: foreignTradeID,
			}
			log.Info("Got trade: ", trade)
			tradesChannel <- trade
		}
	}

}

func parseCoinBaseTradeMessage(message coinBaseWSResponse) (price float64, volume float64, timestamp time.Time, foreignTradeID string, err error) {
	price, err = strconv.ParseFloat(message.Price, 64)
	if err != nil {
		return
	}
	volume, err = strconv.ParseFloat(message.Size, 64)
	if err != nil {
		return
	}
	if message.Side == "sell" {
		volume -= 1
	}
	timestamp, err = time.Parse("2006-01-02T15:04:05.000000Z", message.Time)
	if err != nil {
		return
	}

	foreignTradeID = strconv.Itoa(int(message.TradeID))
	return
}
