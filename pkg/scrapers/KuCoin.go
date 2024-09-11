package scrapers

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
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

type kucoinScraper struct {
	wsClient         *ws.Conn
	tradesChannel    chan models.Trade
	subscribeChannel chan models.ExchangePair
	tickerPairMap    map[string]models.Pair
	lastTradeTimeMap map[string]time.Time
	maxErrCount      int
	restartWaitTime  int
	genesis          time.Time
}

var (
	kucoinWSBaseString    = "wss://ws-api-spot.kucoin.com/"
	kucoinTokenURL        = "https://api.kucoin.com/api/v1/bullet-public"
	kucoinPingIntervalFix = int64(10)
)

func NewKuCoinScraper(ctx context.Context, pairs []models.ExchangePair, failoverChannel chan string, wg *sync.WaitGroup) Scraper {
	defer wg.Done()
	var lock sync.RWMutex
	log.Info("Started KuCoin scraper.")

	token, pingInterval, err := getPublicKuCoinToken(kucoinTokenURL)
	if err != nil {
		log.Error("getPublicKuCoinToken: ", err)
	}

	scraper := kucoinScraper{
		tradesChannel:    make(chan models.Trade),
		subscribeChannel: make(chan models.ExchangePair),
		tickerPairMap:    models.MakeTickerPairMap(pairs),
		lastTradeTimeMap: make(map[string]time.Time),
		maxErrCount:      20,
		restartWaitTime:  5,
		genesis:          time.Now(),
	}

	var wsDialer ws.Dialer
	wsClient, _, err := wsDialer.Dial(kucoinWSBaseString+"?token="+token, nil)
	if err != nil {
		log.Error("Dial KuCoin ws base string: ", err)
		failoverChannel <- string(KUCOIN_EXCHANGE)
		return &scraper
	}
	scraper.wsClient = wsClient

	// Subscribe to pairs.
	for _, pair := range pairs {
		if err := scraper.subscribe(pair, true, &lock); err != nil {
			log.Errorf("KuCoin - subscribe to pair %s: %v", pair.ForeignName, err)
		} else {
			log.Infof("KuCoin - subscribe to pair %s", pair.ForeignName)
		}
	}

	go scraper.ping(ctx, pingInterval, time.Now(), &lock)
	go scraper.fetchTrades()

	// Check last trade time for each subscribed pair and resubscribe if no activity for more than @kucoinWatchdogDelayMap.
	for _, pair := range pairs {
		envVar := strings.ToUpper(KUCOIN_EXCHANGE) + "_WATCHDOG_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[0] + "_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[1]
		watchdogDelay, err := strconv.ParseInt(utils.Getenv(envVar, "60"), 10, 64)
		if err != nil {
			log.Error("Parse kucoinWatchdogDelay: ", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(watchdogDelay) * time.Second)
		go watchdog(ctx, pair, watchdogTicker, scraper.lastTradeTimeMap, watchdogDelay, scraper.subscribeChannel, &lock)
		go scraper.resubscribe(ctx, &lock)
	}

	return &scraper

}

func (scraper *kucoinScraper) fetchTrades() {
	// Read trades stream.
	var errCount int
	for {

		var message kuCoinWSResponse
		err := scraper.wsClient.ReadJSON(&message)
		if err != nil {
			if handleErrorReadJSON(err, &errCount, scraper.maxErrCount, scraper.restartWaitTime) {
				return
			}
			continue
		}

		if message.Type == "pong" {
			// log.Info("KuCoin - Successful ping: received pong.")
		} else if message.Type == "message" {
			scraper.handleWSResponse(message)
		}

	}
}

func (scraper *kucoinScraper) handleWSResponse(message kuCoinWSResponse) {
	// Parse trade quantities.
	price, volume, timestamp, foreignTradeID, err := parseKuCoinTradeMessage(message)
	if err != nil {
		log.Error("KuCoin - parseTradeMessage: ", err)
	}

	// Identify ticker symbols with underlying assets.
	pair := strings.Split(message.Data.Symbol, "-")
	var exchangepair models.Pair
	if len(pair) > 1 {
		exchangepair = scraper.tickerPairMap[pair[0]+pair[1]]
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
	scraper.lastTradeTimeMap[pair[0]+"-"+pair[1]] = trade.Time
	scraper.tradesChannel <- trade
}

func (scraper *kucoinScraper) Close(cancel context.CancelFunc) error {
	log.Warn("KuCoin - call scraper.Close()")
	cancel()
	return scraper.wsClient.Close()
}

func (scraper *kucoinScraper) TradesChannel() chan models.Trade {
	return scraper.tradesChannel
}

func (scraper *kucoinScraper) resubscribe(ctx context.Context, lock *sync.RWMutex) {
	for {
		select {
		case pair := <-scraper.subscribeChannel:
			err := scraper.subscribe(pair, false, lock)
			if err != nil {
				log.Errorf("KuCoin - Unsubscribe pair %s: %v", pair.ForeignName, err)
			} else {
				log.Infof("KuCoin - Unsubscribed pair %s:%s", KUCOIN_EXCHANGE, pair.ForeignName)
			}
			time.Sleep(2 * time.Second)
			err = scraper.subscribe(pair, true, lock)
			if err != nil {
				log.Errorf("KuCoin - Resubscribe pair %s: %v", pair.ForeignName, err)
			} else {
				log.Infof("KuCoin - Subscribed to pair %s:%s", KUCOIN_EXCHANGE, pair.ForeignName)
			}
		case <-ctx.Done():
			log.Warn("-------------------------------------KuCoin - close resubscribe routine of scraper with genesis: ", scraper.genesis)
			return
		}
	}
}

func (scraper *kucoinScraper) subscribe(pair models.ExchangePair, subscribe bool, lock *sync.RWMutex) error {
	defer lock.Unlock()
	subscribeType := "unsubscribe"
	if subscribe {
		subscribeType = "subscribe"
	}

	a := &kuCoinWSSubscribeMessage{
		Type:  subscribeType,
		Topic: "/market/match:" + pair.ForeignName,
	}
	lock.Lock()
	return scraper.wsClient.WriteJSON(a)
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
func (scraper *kucoinScraper) ping(ctx context.Context, pingInterval int64, starttime time.Time, lock *sync.RWMutex) {
	var ping kuCoinWSMessage
	ping.Type = "ping"
	tick := time.NewTicker(time.Duration(kucoinPingIntervalFix) * time.Second)

	for {
		select {
		case <-tick.C:
			lock.Lock()
			if err := scraper.wsClient.WriteJSON(ping); err != nil {
				log.Error("KuCoin - send ping: " + err.Error())
				lock.Unlock()
				return
			}
			lock.Unlock()
		case <-ctx.Done():
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
