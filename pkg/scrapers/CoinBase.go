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
	coinbaseSubscribeChannel = make(chan models.ExchangePair)
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
	var lock sync.RWMutex
	log.Info("Started CoinBase scraper.")
	coinbaseRun = true
	tickerPairMap := models.MakeTickerPairMap(pairs)

	// Dial websocket API.
	var wsDialer ws.Dialer
	wsClient, _, err := wsDialer.Dial(coinbaseWSBaseString, nil)
	if err != nil {
		log.Error("Dial CoinBase ws base string: ", err)
		failoverChannel <- string(COINBASE_EXCHANGE)
		return "closed"
	}

	// Subscribe to pairs and initialize coinbaseLastTradeTimeMap.
	for _, pair := range pairs {
		if err := coinbaseSubscribe(pair, &lock, wsClient); err != nil {
			log.Errorf("CoinBase - subscribe to pair %s: %v", pair.ForeignName, err)
		} else {
			log.Infof("CoinBase - Subscribed to pair %s:%s", COINBASE_EXCHANGE, pair.ForeignName)
			coinbaseLastTradeTimeMap[pair.ForeignName] = time.Now()
		}
	}

	// Check last trade time across all pairs and restart if no activity for more than @coinbaseWatchdogDelay.
	coinbaseLastTradeTime = time.Now()
	log.Info("CoinBase - Initialize coinbaseLastTradeTime after failover: ", coinbaseLastTradeTime)
	watchdogTicker := time.NewTicker(time.Duration(coinbaseWatchdogDelay) * time.Second)
	go globalWatchdog(watchdogTicker, &coinbaseLastTradeTime, coinbaseWatchdogDelay, &coinbaseRun)

	// Check last trade time for each subscribed pair and resubscribe if no activity for more than @coinbaseWatchdogDelayMap.
	for _, pair := range pairs {
		envVar := strings.ToUpper(COINBASE_EXCHANGE) + "_WATCHDOG_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[0] + "_" + strings.Split(strings.ToUpper(pair.ForeignName), "-")[1]
		coinbaseWatchdogDelay, err = strconv.ParseInt(utils.Getenv(envVar, "60"), 10, 64)
		if err != nil {
			log.Error("Parse coinbaseWatchdogDelay: ", err)
		}
		watchdogTicker := time.NewTicker(time.Duration(coinbaseWatchdogDelay) * time.Second)
		go watchdog(pair, watchdogTicker, coinbaseLastTradeTimeMap, coinbaseWatchdogDelay, coinbaseSubscribeChannel, &coinbaseRun, &lock)
		go coinbaseResubscribe(coinbaseSubscribeChannel, &lock, &coinbaseRun, wsClient)
	}

	// Read trades stream.
	var errCount int
	for coinbaseRun {
		var message coinBaseWSResponse
		err = wsClient.ReadJSON(&message)
		if err != nil {
			readJSONError(COINBASE_EXCHANGE, err, &errCount, &coinbaseRun, coinbaseRestartWaitTime, coinbaseMaxErrCount)
			continue
		}

		if message.Type == "match" {

			trade, err := coinbaseParseTradeMessage(message)
			if err != nil {
				log.Errorf("parseCoinBaseTradeMessage: %s", err.Error())
				continue
			}

			// Identify ticker symbols with underlying assets.
			pair := strings.Split(message.ProductID, "-")
			if len(pair) > 1 {
				trade.QuoteToken = tickerPairMap[pair[0]+pair[1]].QuoteToken
				trade.BaseToken = tickerPairMap[pair[0]+pair[1]].BaseToken
			}

			// log.Infof("Got trade: %s -- %v", trade.QuoteToken.Symbol+"-"+trade.BaseToken.Symbol, trade.Price)
			coinbaseLastTradeTime = trade.Time
			coinbaseLastTradeTimeMap[pair[0]+"-"+pair[1]] = trade.Time
			tradesChannel <- trade
		}
	}

	log.Warn("Close CoinBase scraper.")
	failoverChannel <- string(COINBASE_EXCHANGE)
	return "closed"

}

func coinbaseResubscribe(subscribeChannel chan models.ExchangePair, lock *sync.RWMutex, scraperRun *bool, conn *ws.Conn) {
	for *scraperRun {
		select {
		case pair := <-subscribeChannel:
			err := coinbaseUnsubscribe(pair, lock, conn)
			if err != nil {
				log.Errorf("CoinBase - Unsubscribe pair %s: %v", pair.ForeignName, err)
			} else {
				log.Infof("CoinBase - Unsubscribed pair %s:%s", COINBASE_EXCHANGE, pair.ForeignName)
			}
			time.Sleep(2 * time.Second)
			err = coinbaseSubscribe(pair, lock, conn)
			if err != nil {
				log.Errorf("CoinBase - Resubscribe pair %s: %v", pair.ForeignName, err)
			} else {
				log.Infof("CoinBase - Subscribed to pair %s:%s", COINBASE_EXCHANGE, pair.ForeignName)
			}
		}
	}
}

func coinbaseSubscribe(pair models.ExchangePair, lock *sync.RWMutex, wsClient *ws.Conn) error {
	defer lock.Unlock()
	a := &coinBaseWSSubscribeMessage{
		Type: "subscribe",
		Channels: []coinBaseChannel{
			{
				Name:       "matches",
				ProductIDs: []string{pair.ForeignName},
			},
		},
	}
	lock.Lock()
	return wsClient.WriteJSON(a)
}

func coinbaseUnsubscribe(pair models.ExchangePair, lock *sync.RWMutex, wsClient *ws.Conn) error {
	defer lock.Unlock()
	a := &coinBaseWSSubscribeMessage{
		Type: "unsubscribe",
		Channels: []coinBaseChannel{
			{
				Name:       "matches",
				ProductIDs: []string{pair.ForeignName},
			},
		},
	}
	lock.Lock()
	return wsClient.WriteJSON(a)
}

func coinbaseParseTradeMessage(message coinBaseWSResponse) (models.Trade, error) {
	price, err := strconv.ParseFloat(message.Price, 64)
	if err != nil {
		return models.Trade{}, nil
	}
	volume, err := strconv.ParseFloat(message.Size, 64)
	if err != nil {
		return models.Trade{}, nil
	}
	if message.Side == "sell" {
		volume -= 1
	}
	timestamp, err := time.Parse("2006-01-02T15:04:05.000000Z", message.Time)
	if err != nil {
		return models.Trade{}, nil
	}

	foreignTradeID := strconv.Itoa(int(message.TradeID))

	trade := models.Trade{
		Price:          price,
		Volume:         volume,
		Time:           timestamp,
		Exchange:       models.Exchange{Name: COINBASE_EXCHANGE},
		ForeignTradeID: foreignTradeID,
	}

	return trade, nil
}
