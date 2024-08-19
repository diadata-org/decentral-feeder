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
	kucoinWSBaseString    = "wss://ws-api-spot.kucoin.com/"
	kucoinTokenURL        = "https://api.kucoin.com/api/v1/bullet-public"
	kucoinPingIntervalFix = int64(10)
	kucoinMaxErrCount     = 20
	kucoinRun             bool
	kucoinWatchdogDelay   int64
	kucoinRestartWaitTime = 5
	kucoinLastTradeTime   time.Time
)

func NewKuCoinScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, failoverChannel chan string, wg *sync.WaitGroup) string {
	defer wg.Done()
	kucoinRun = true
	log.Info("Started KuCoin scraper.")

	token, pingInterval, err := getPublicKuCoinToken(kucoinTokenURL)
	if err != nil {
		log.Error("getPublicKuCoinToken: ", err)
	}

	var wsDialer ws.Dialer
	wsClient, _, err := wsDialer.Dial(kucoinWSBaseString+"?token="+token, nil)
	if err != nil {
		log.Error("Dial KuCoin ws base string: ", err)
		failoverChannel <- string(KUCOIN_EXCHANGE)
		return "closed"
	}

	// Subscribe to pairs.
	for _, pair := range pairs {
		if err := kucoinSubscribe(pair, wsClient); err != nil {
			log.Errorf("KuCoin - subscribe to pair %s: %v", pair.ForeignName, err)
		}
	}

	closePingChannel := make(chan bool)
	go ping(wsClient, pingInterval, time.Now(), closePingChannel)

	// Check for liveliness of the scraper.
	kucoinLastTradeTime = time.Now()
	log.Info("KuCoin - Initialize kucoinLastTradeTime after failover: ", kucoinLastTradeTime)
	watchdogTicker := time.NewTicker(time.Duration(kucoinWatchdogDelay) * time.Second)
	go globalWatchdog(watchdogTicker, &kucoinLastTradeTime, kucoinWatchdogDelay, &kucoinRun)

	// Read trades stream.
	var errCount int
	for kucoinRun {

		var message kuCoinWSResponse
		err = wsClient.ReadJSON(&message)
		if err != nil {
			log.Errorf("KuCoin - ReadMessage: %v", err)
			errCount++
			if errCount > kucoinMaxErrCount {
				log.Warnf("too many errors. wait for %v seconds and restart scraper.", kucoinRestartWaitTime)
				time.Sleep(time.Duration(kucoinRestartWaitTime) * time.Second)
				kucoinRun = false
				break
			}
			continue
		}

		if message.Type == "pong" {
			log.Info("KuCoin - Successful ping: received pong.")
		} else if message.Type == "message" {

			// Parse trade quantities.
			price, volume, timestamp, foreignTradeID, err := parseKuCoinTradeMessage(message)
			if err != nil {
				log.Error("KuCoin - parseTradeMessage: ", err)
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
			kucoinLastTradeTime = trade.Time
			tradesChannel <- trade
		}
	}

	log.Warn("Close KuCoin scraper.")
	closePingChannel <- true
	failoverChannel <- string(KUCOIN_EXCHANGE)
	return "closed"

}

func kucoinSubscribe(pair models.ExchangePair, client *ws.Conn) error {
	a := &kuCoinWSSubscribeMessage{
		Type:  "subscribe",
		Topic: "/market/match:" + pair.ForeignName,
	}
	log.Infof("Subscribed for Pair %s:%s", KUCOIN_EXCHANGE, pair.ForeignName)
	return client.WriteJSON(a)
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
func ping(wsClient *ws.Conn, pingInterval int64, starttime time.Time, closeChannel chan bool) {
	var ping kuCoinWSMessage
	ping.Type = "ping"
	tick := time.NewTicker(time.Duration(kucoinPingIntervalFix) * time.Second)

	for {
		select {
		case <-tick.C:
			// log.Infof("KuCoin - send ping.")
			if err := wsClient.WriteJSON(ping); err != nil {
				log.Error("KuCoin - send ping: " + err.Error())
				return
			}
		case <-closeChannel:
			log.Warn("close ping.")
			return
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
