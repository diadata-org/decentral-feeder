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
	coinbaseWSBaseString     = "wss://ws-feed.exchange.coinbase.com"
	coinbaseMaxErrCount      = 20
	coinbaseRun              bool
	coinbaseWatchdogDelay    int64
	coinbaseRestartWaitTime  = 5
	coinbaseLastTradeTime    time.Time
	coinbaseLastTradeTimeMap = make(map[string]time.Time)
	coinbaseWatchdogDelayMap = make(map[string]int64)
	coinbaseRunChannel       chan models.ExchangePair
)

func init() {
	var err error
	coinbaseWatchdogDelay, err = strconv.ParseInt(utils.Getenv("COINBASE_WATCHDOGDELAY", "240"), 10, 64)
	if err != nil {
		log.Error("Parse COINBASE_WATCHDOGDELAY: ", err)
	}
}

func NewCoinBaseScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, failoverChannel chan string, wg *sync.WaitGroup) string {
	defer wg.Done()
	defer close(coinbaseRunChannel)
	var lock sync.RWMutex
	log.Info("Started CoinBase scraper.")
	coinbaseRun = true

	var wsDialer ws.Dialer
	wsClient, _, err := wsDialer.Dial(coinbaseWSBaseString, nil)
	if err != nil {
		log.Error("Dial CoinBase ws base string: ", err)
		failoverChannel <- string(COINBASE_EXCHANGE)
		return "closed"
	}

	// Subscribe to pairs and initialize coinbaseLastTradeTimeMap.
	for _, pair := range pairs {
		if err := coinbaseSubscribe(pair, wsClient); err != nil {
			log.Errorf("CoinBase - subscribe to pair %s: %v", pair.ForeignName, err)
		} else {
			coinbaseLastTradeTimeMap[pair.ForeignName] = time.Now()
		}
	}

	// Check last trade time across all pairs and restart if no activity for more than @coinbaseWatchdogDelay.
	coinbaseLastTradeTime = time.Now()
	log.Info("CoinBase - Initialize coinbaseLastTradeTime after failover: ", coinbaseLastTradeTime)
	watchdogTicker := time.NewTicker(time.Duration(coinbaseWatchdogDelay) * time.Second)

	go func() {
		for range watchdogTicker.C {
			duration := time.Since(coinbaseLastTradeTime)
			if duration > time.Duration(coinbaseWatchdogDelay)*time.Second {
				log.Error("CoinBase - watchdogTicker failover")
				coinbaseRun = false
				break
			}
		}
	}()

	// Check last trade time for each subscribed pair and resubscribe if no activity for more than @coinbaseWatchdogDelayMap.
	for _, pair := range pairs {
		coinbaseWatchdogDelayMap[pair.ForeignName], err = strconv.ParseInt(utils.Getenv(COINBASE_EXCHANGE+strings.ToUpper(pair.ForeignName), "60"), 10, 64)
		if err != nil {
			log.Error("Parse coinbaseWatchdogDelayMap: ", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(coinbaseWatchdogDelayMap[pair.ForeignName]) * time.Second)
		go watchdog(pair, watchdogTicker, coinbaseLastTradeTimeMap, coinbaseWatchdogDelayMap, coinbaseRunChannel, &lock)
		go coinbaseResubscribe(coinbaseRunChannel, wsClient)
	}

	// Read trades stream.
	var errCount int
	for coinbaseRun {
		var message coinBaseWSResponse
		err = wsClient.ReadJSON(&message)
		if err != nil {
			log.Errorf("CoinBase - ReadMessage: %v", err)
			errCount++
			if errCount > coinbaseMaxErrCount {
				log.Warnf("too many errors. wait for %v seconds and restart scraper.", coinbaseRestartWaitTime)
				time.Sleep(time.Duration(coinbaseRestartWaitTime) * time.Second)
				coinbaseRun = false
				break
			}
			continue
		}

		if message.Type == "match" {

			// Parse trade quantities.
			price, volume, timestamp, foreignTradeID, err := parseCoinBaseTradeMessage(message)
			if err != nil {
				log.Error("CoinBase - parseTradeMessage: ", err)
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
			// log.Info("Got trade: ", trade)
			coinbaseLastTradeTime = trade.Time
			tradesChannel <- trade
		}
	}

	log.Warn("Close CoinBase scraper.")
	failoverChannel <- string(COINBASE_EXCHANGE)
	return "closed"

}

func coinbaseResubscribe(runChannel chan models.ExchangePair, conn *ws.Conn) {
	for {
		select {
		case pair := <-runChannel:
			err := coinbaseUnsubscribe(pair, conn)
			if err != nil {
				log.Errorf("CoinBase - Unsubscribe pair %s: %v", pair.ForeignName, err)
			}
			time.Sleep(2 * time.Second)
			err = coinbaseSubscribe(pair, conn)
			if err != nil {
				log.Errorf("CoinBase - Resubscribe pair %s: %v", pair.ForeignName, err)
			}
		}
	}
}

func coinbaseSubscribe(pair models.ExchangePair, wsClient *ws.Conn) error {
	a := &coinBaseWSSubscribeMessage{
		Type: "subscribe",
		Channels: []coinBaseChannel{
			{
				Name:       "matches",
				ProductIDs: []string{pair.ForeignName},
			},
		},
	}
	log.Infof("CoinBase - Subscribed for Pair %s:%s", COINBASE_EXCHANGE, pair.ForeignName)
	return wsClient.WriteJSON(a)
}

func coinbaseUnsubscribe(pair models.ExchangePair, wsClient *ws.Conn) error {
	a := &coinBaseWSSubscribeMessage{
		Type: "unsubscribe",
		Channels: []coinBaseChannel{
			{
				Name:       "matches",
				ProductIDs: []string{pair.ForeignName},
			},
		},
	}
	log.Infof("CoinBase - Unsubscribed Pair %s:%s", COINBASE_EXCHANGE, pair.ForeignName)
	return wsClient.WriteJSON(a)
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
