package scrapers

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	ws "github.com/gorilla/websocket"
)

// A WebSocketSubscribeMessage represents a message to subscribe the public/private channel.
type kuCoinWSSubscribeMessage struct {
	Id             string `json:"id"`
	Type           string `json:"type"`
	Topic          string `json:"topic"`
	PrivateChannel bool   `json:"privateChannel"`
	Response       bool   `json:"response"`
}

type kuCoinWSResponse struct {
	Type    string       `json:"type"`
	Topic   string       `json:"topic"`
	Subject string       `json:"subject"`
	Data    kuCoinWSData `json:"data"`
}

type kuCoinWSData struct {
	Sequence string `json:"sequence"`
	Type     string `json:"type"`
	Symbol   string `json:"symbol"`
	Side     string `json:"side"`
	Price    string `json:"price"`
	Size     string `json:"size"`
	TradeID  string `json:"tradeId"`
	Time     string `json:"time"`
}

var (
	kucoinWSBaseString = "wss://ws-api-spot.kucoin.com/"
	kucoinTokenURL     = "https://api.kucoin.com/api/v1/bullet-public"
)

func NewKuCoinScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Info("Started KuCoin scraper.")

	token, pingInterval, err := getPublicKuCoinToken(kucoinTokenURL)
	if err != nil {
		log.Fatal("getPublicKuCoinToken: ", err)
	}

	var wsDialer ws.Dialer
	wsClient, _, err := wsDialer.Dial(kucoinWSBaseString+"?token="+token, nil)
	if err != nil {
		log.Fatal("Dial KuCoin ws base string: ", err)
	}

	// Subscribe to pairs.
	for _, pair := range pairs {

		a := &kuCoinWSSubscribeMessage{
			Type:  "subscribe",
			Topic: "/market/match:" + pair.ForeignName,
		}
		log.Infof("Subscribed for Pair %s:%s", KUCOIN_EXCHANGE, pair.ForeignName)
		if err := wsClient.WriteJSON(a); err != nil {
			log.Error(err.Error())
		}
	}

	go ping(wsClient, pingInterval)

	// Read trades stream.
	for {
		var message kuCoinWSResponse
		err = wsClient.ReadJSON(&message)
		if err != nil {
			log.Errorf("ReadMessage: %v", err)
			continue
		}

		if message.Type == "pong" {
			log.Info("Successful ping: received pong.")
		} else if message.Type == "message" {

			// Parse trade quantities.
			price, volume, timestamp, foreignTradeID, err := parseKuCoinTradeMessage(message)
			if err != nil {
				log.Error("parseTradeMessage: ", err)
			}

			// Identify ticker symbols with underlying assets.
			tickerPairMap := models.MakeTickerPairMap(pairs)
			pair := strings.Split(message.Data.Symbol, "-")
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
				Exchange:       models.Exchange{Name: KUCOIN_EXCHANGE},
				ForeignTradeID: foreignTradeID,
			}
			log.Info("Got trade: ", trade)
			tradesChannel <- trade
		}
	}

}

func parseKuCoinTradeMessage(message kuCoinWSResponse) (price float64, volume float64, timestamp time.Time, foreignTradeID string, err error) {
	price, err = strconv.ParseFloat(message.Data.Price, 64)
	if err != nil {
		return
	}
	volume, err = strconv.ParseFloat(message.Data.Size, 64)
	if err != nil {
		return
	}
	if message.Data.Side == "sell" {
		volume -= 1
	}
	timeMilliseconds, err := strconv.Atoi(message.Data.Time)
	if err != nil {
		return
	}
	timestamp = time.Unix(0, int64(timeMilliseconds))
	foreignTradeID = message.Data.TradeID
	return
}

// A WebSocketMessage represents a message between the WebSocket client and server.
type kuCoinWSMessage struct {
	Id   string `json:"id"`
	Type string `json:"type"`
}

type kuCoinPostResponse struct {
	Code string `json:"code"`
	Data struct {
		Token           string            `json:"token"`
		InstanceServers []instanceServers `json:"instanceServers"`
	} `json:"data"`
}

type instanceServers struct {
	PingInterval int64 `json:"pingInterval"`
}

// Send ping to server.
func ping(wsClient *ws.Conn, pingInterval int64) {
	var ping kuCoinWSMessage
	ping.Type = "ping"
	tick := time.NewTicker(time.Duration(pingInterval/2) * time.Second)

	for range tick.C {
		log.Infof("send ping.")
		if err := wsClient.WriteJSON(ping); err != nil {
			log.Error(err.Error())
		}
	}
}

// getPublicKuCoinToken returns a token for public market data along with the pingInterval in seconds.
func getPublicKuCoinToken(url string) (token string, pingInterval int64, err error) {
	postBody, _ := json.Marshal(map[string]string{})
	responseBody := bytes.NewBuffer(postBody)
	data, err := http.Post(url, "application/json", responseBody)
	if err != nil {
		return
	}
	defer data.Body.Close()
	body, err := ioutil.ReadAll(data.Body)
	if err != nil {
		return
	}

	var postResp kuCoinPostResponse
	err = json.Unmarshal(body, &postResp)
	if err != nil {
		return
	}
	if len(postResp.Data.InstanceServers) > 0 {
		pingInterval = postResp.Data.InstanceServers[0].PingInterval
	}
	token = postResp.Data.Token
	return
}
