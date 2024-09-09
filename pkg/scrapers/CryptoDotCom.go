package scrapers

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	ws "github.com/gorilla/websocket"

	"go.uber.org/ratelimit"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
)

const (
	cryptoDotComAPIEndpoint    = "https://api.crypto.com/v2"
	cryptoDotComWSEndpoint     = "wss://stream.crypto.com/v2/market"
	cryptoDotComSpotTradingBuy = "BUY"

	// cryptoDotComWSRateLimitPerSec is a max request per second for sending websocket requests.
	cryptoDotComWSRateLimitPerSec = 10

	// cryptoDotComTaskMaxRetry is a max retry value used when retrying subscribe/unsubscribe trades.
	cryptoDotComTaskMaxRetry = 20

	// cryptoDotComConnMaxRetry is a max retry value used when retrying to create a new connection.
	cryptoDotComConnMaxRetry = 50

	// cryptoDotComRateLimitError is a rate limit error code.
	cryptoDotComRateLimitError = 10006

	// cryptoDotComBackoffSeconds is the number of seconds it waits for the next ws reconnect.
	cryptoDotComBackoffSeconds = 5
)

var (
	cryptoDotComRun           bool
	cryptoDotComWatchdogDelay int64
	cryptoDotComLastTradeTime time.Time
)

type nothing struct{}

// cryptoDotComWSTask is a websocket task tracking subscription/unsubscription
type cryptoDotComWSTask struct {
	Method     string
	Params     cryptoDotComWSRequestParams
	RetryCount int
}

func (c *cryptoDotComWSTask) toString() string {
	return fmt.Sprintf("method=%s, param=%s, retry=%d", c.Method, c.Params.toString(), c.RetryCount)
}

// cryptoDotComWSRequest is a websocket request
type cryptoDotComWSRequest struct {
	ID     int                         `json:"id"`
	Method string                      `json:"method"`
	Params cryptoDotComWSRequestParams `json:"params,omitempty"`
	Nonce  int64                       `json:"nonce,omitempty"`
}

// cryptoDotComWSRequestParams is a websocket request param
type cryptoDotComWSRequestParams struct {
	Channels []string `json:"channels"`
}

func (c *cryptoDotComWSRequestParams) toString() string {
	length := len(c.Channels)
	if length == 1 {
		return c.Channels[0]
	}
	if length > 1 {
		return fmt.Sprintf("%s +%d more", c.Channels[0], length-1)
	}

	return ""
}

// cryptoDotComWSResponse is a websocket response
type cryptoDotComWSResponse struct {
	ID     int             `json:"id"`
	Code   int             `json:"code"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
}

// cryptoDotComWSSubscriptionResult is a trade result coming from websocket
type cryptoDotComWSSubscriptionResult struct {
	InstrumentName string            `json:"instrument_name"`
	Subscription   string            `json:"subscription"`
	Channel        string            `json:"channel"`
	Data           []json.RawMessage `json:"data"`
}

// cryptoDotComWSInstrument represents a trade
type cryptoDotComWSInstrument struct {
	Price     string `json:"p"`
	Quantity  string `json:"q"`
	Side      string `json:"s"`
	TradeID   string `json:"d"`
	TradeTime int64  `json:"t"`
}

// cryptoDotComInstrument represents a trading pair
type cryptoDotComInstrument struct {
	InstrumentName          string `json:"instrument_name"`
	QuoteCurrency           string `json:"quote_currency"`
	BaseCurrency            string `json:"base_currency"`
	PriceDecimals           int    `json:"price_decimals"`
	QuantityDecimals        int    `json:"quantity_decimals"`
	MarginTradingEnabled    bool   `json:"margin_trading_enabled"`
	MarginTradingEnabled5x  bool   `json:"margin_trading_enabled_5x"`
	MarginTradingEnabled10x bool   `json:"margin_trading_enabled_10x"`
	MaxQuantity             string `json:"max_quantity"`
	MinQuantity             string `json:"min_quantity"`
}

// cryptoDotComInstrumentResponse is an API response for retrieving instruments
type cryptoDotComInstrumentResponse struct {
	Code   int `json:"code"`
	Result struct {
		Instruments []cryptoDotComInstrument `json:"instruments"`
	} `json:"result"`
}

// CryptoDotComScraper is a scraper for Crypto.com
type CryptoDotComScraper struct {
	ws *ws.Conn
	rl ratelimit.Limiter

	// signaling channels for session initialization and finishing
	shutdown           chan nothing
	shutdownDone       chan nothing
	signalShutdown     sync.Once
	signalShutdownDone sync.Once

	// error handling; err should be read from error(), closed should be read from isClosed()
	// those two methods implement RW lock
	errMutex    sync.RWMutex
	err         error
	closedMutex sync.RWMutex
	closed      bool
	//consecutiveErrCount int

	// used to keep track of trading pairs that we subscribed to
	pairScrapers sync.Map
	exchangeName string
	chanTrades   chan *models.Trade
	taskCount    int32
	tasks        sync.Map

	// used to handle connection retry
	connMutex      sync.RWMutex
	connRetryCount int
}

func init() {
	var err error
	cryptoDotComWatchdogDelay, err = strconv.ParseInt(utils.Getenv("CRYPTODOTOM_WATCHDOGDELAY", "60"), 10, 64)
	if err != nil {
		log.Error("Parse CRYPTODOTOM_WATCHDOGDELAY: ", err)
	}
}

// NewCryptoDotComScraper returns a new Crypto.com scraper
func NewCryptoDotComScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, failoverChannel chan string, wg *sync.WaitGroup) string {
	defer wg.Done()
	log.Info("Started Crypto.com scraper.")
	cryptoDotComRun = true
	tickerPairMap := models.MakeTickerPairMap(pairs)

	s := &CryptoDotComScraper{
		shutdown:     make(chan nothing),
		shutdownDone: make(chan nothing),
		err:          nil,
	}
	if err := s.newConn(); err != nil {
		log.Error("Crypto.com - " + err.Error())
		log.Warn("Close Crypto.com scraper.")
		failoverChannel <- string(CRYPTODOTCOM_EXCHANGE)
		return "closed"
	}

	s.rl = ratelimit.New(cryptoDotComWSRateLimitPerSec)

	// ----------------------------------------
	// Subscribe to pairs
	// ----------------------------------------
	err := s.subscribe(pairs)
	if err != nil {
		log.Error("Crypto.com - send: ", err)
	}

	// ----------------------------------------
	//  Check for liveliness of the scraper.
	// ----------------------------------------
	cryptoDotComLastTradeTime = time.Now()
	log.Info("Crypto.com - Initialize cryptoDotComLastTradeTime after failover: ", cryptoDotComLastTradeTime)
	watchdogTicker := time.NewTicker(time.Duration(cryptoDotComWatchdogDelay) * time.Second)
	go globalWatchdog(watchdogTicker, &cryptoDotComLastTradeTime, cryptoDotComWatchdogDelay, &cryptoDotComRun)

	// ----------------------------------------
	// Fetch trades
	// ----------------------------------------
	defer s.cleanup()

	for cryptoDotComRun {
		select {
		case <-s.shutdown:
			log.Println("Crypto.com - Shutting down main loop")
			log.Warn("Close Crypto.com scraper.")
			failoverChannel <- string(CRYPTODOTCOM_EXCHANGE)
			return "closed"
		default:
		}

		var res cryptoDotComWSResponse
		if err := s.wsConn().ReadJSON(&res); err != nil {
			log.Warnf("Crypto.com - Creating a new connection caused by err=%s", err.Error())

			if retryErr := s.retryConnection(); retryErr != nil {
				s.setError(retryErr)
				log.Errorf("Crypto.com - Shutting down main loop after retrying to create a new connection, err=%s", retryErr.Error())
				failoverChannel <- string(CRYPTODOTCOM_EXCHANGE)
				return "closed"
			}

			log.Info("Crypto.com - Successfully created a new connection")
		}
		if res.Code == cryptoDotComRateLimitError {
			time.Sleep(time.Duration(cryptoDotComBackoffSeconds) * time.Second)
			if err := s.retryTask(res.ID); err != nil {
				s.setError(err)
				log.Errorf("Crypto.com - Shutting down main loop due to failing to retry a task, err=%s", err.Error())
			}
		}
		if res.Code != 0 {
			log.Errorf("Crypto.com - Shutting down main loop due to non-retryable response code %d", res.Code)
		}

		switch res.Method {
		case "public/heartbeat":
			if err := s.ping(res.ID); err != nil {
				s.setError(err)
				log.Errorf("Crypto.com - Shutting down main loop due to heartbeat failure, err=%s", err.Error())
			}
		case "subscribe":
			if len(res.Result) == 0 {
				continue
			}

			var subscription cryptoDotComWSSubscriptionResult
			if err := json.Unmarshal(res.Result, &subscription); err != nil {
				s.setError(err)
				log.Errorf("Crypto.com - Shutting down main loop due to response unmarshaling failure, err=%s", err.Error())
			}
			if subscription.Channel != "trade" {
				continue
			}

			// baseCurrency := strings.Split(subscription.InstrumentName, `_`)[0]

			exchangepair := tickerPairMap[strings.Split(subscription.InstrumentName, "_")[0]+strings.Split(subscription.InstrumentName, "_")[1]]

			for _, data := range subscription.Data {
				var i cryptoDotComWSInstrument
				if err := json.Unmarshal(data, &i); err != nil {
					s.setError(err)
					log.Errorf("Crypto.com - Shutting down main loop due to instrument unmarshaling failure, err=%s", err.Error())
				}

				volume, err := strconv.ParseFloat(i.Quantity, 64)
				if err != nil {
					log.Error("Crypto.com - parse volume: ", err)
					continue
				}
				if i.Side != cryptoDotComSpotTradingBuy {
					volume = -volume
				}

				price, err := strconv.ParseFloat(i.Price, 64)
				if err != nil {
					log.Error("Crypto.com - parse price: ", err)
					continue
				}

				trade := models.Trade{
					QuoteToken:     exchangepair.QuoteToken,
					BaseToken:      exchangepair.BaseToken,
					Price:          price,
					Volume:         volume,
					Time:           time.Unix(0, i.TradeTime*int64(time.Millisecond)),
					Exchange:       models.Exchange{Name: CRYPTODOTCOM_EXCHANGE},
					ForeignTradeID: i.TradeID,
				}

				select {
				case <-s.shutdown:
				case tradesChannel <- trade:
					cryptoDotComLastTradeTime = trade.Time
					// log.Info("Got trade: ", trade)
				}
			}
		}
	}

	log.Warn("Close Crypto.com scraper.")
	failoverChannel <- string(CRYPTODOTCOM_EXCHANGE)
	return "closed"

}

func (s *CryptoDotComScraper) newConn() error {
	conn, _, err := ws.DefaultDialer.Dial(cryptoDotComWSEndpoint, nil)
	if err != nil {
		return err
	}

	// Crypto.com recommends adding a 1-second sleep after establishing the websocket connection, and before requests are sent
	// to avoid occurrences of rate-limit (`TOO_MANY_REQUESTS`) errors.
	// https://exchange-docs.crypto.com/spot/index.html?javascript#websocket-subscriptions
	time.Sleep(time.Duration(cryptoDotComBackoffSeconds) * time.Second)

	defer s.connMutex.Unlock()
	s.connMutex.Lock()
	s.ws = conn

	return nil
}

func (s *CryptoDotComScraper) wsConn() *ws.Conn {
	defer s.connMutex.RUnlock()
	s.connMutex.RLock()

	return s.ws
}

func (s *CryptoDotComScraper) ping(id int) error {
	s.rl.Take()

	return s.wsConn().WriteJSON(&cryptoDotComWSRequest{
		ID:     id,
		Method: "public/respond-heartbeat",
	})
}

func (s *CryptoDotComScraper) cleanup() {
	if err := s.wsConn().Close(); err != nil {
		s.setError(err)
	}

	close(s.chanTrades)
	s.close()
	s.signalShutdownDone.Do(func() {
		close(s.shutdownDone)
	})
}

func (s *CryptoDotComScraper) error() error {
	s.errMutex.RLock()
	defer s.errMutex.RUnlock()

	return s.err
}

func (s *CryptoDotComScraper) setError(err error) {
	s.errMutex.Lock()
	defer s.errMutex.Unlock()

	s.err = err
}

func (s *CryptoDotComScraper) isClosed() bool {
	s.closedMutex.RLock()
	defer s.closedMutex.RUnlock()

	return s.closed
}

func (s *CryptoDotComScraper) close() {
	s.closedMutex.Lock()
	defer s.closedMutex.Unlock()

	s.closed = true
}

func (s *CryptoDotComScraper) subscribe(pairs []models.ExchangePair) error {

	channels := make([]string, len(pairs))
	for idx, pair := range pairs {
		log.Info("Crypto.com - subscribe to pair ", pair.ForeignName)
		channels[idx] = "trade." + strings.Split(pair.ForeignName, "-")[0] + "_" + strings.Split(pair.ForeignName, "-")[1]
		s.pairScrapers.Store(pair.ForeignName, pair)
	}

	taskID := int(atomic.AddInt32(&s.taskCount, 1))
	task := cryptoDotComWSTask{
		Method: "subscribe",
		Params: cryptoDotComWSRequestParams{
			Channels: channels,
		},
		RetryCount: 0,
	}
	s.tasks.Store(taskID, task)

	return s.send(taskID, task)

}

func (s *CryptoDotComScraper) unsubscribe(pairs []models.ExchangePair) error {
	channels := make([]string, len(pairs))
	for idx, pair := range pairs {
		channels[idx] = "trade." + pair.ForeignName
		s.pairScrapers.Delete(pair.ForeignName)
	}

	taskID := int(atomic.AddInt32(&s.taskCount, 1))
	task := cryptoDotComWSTask{
		Method: "unsubscribe",
		Params: cryptoDotComWSRequestParams{
			Channels: channels,
		},
		RetryCount: 0,
	}
	s.tasks.Store(taskID, task)

	return s.send(taskID, task)
}

func (s *CryptoDotComScraper) retryConnection() error {
	s.connRetryCount += 1
	if s.connRetryCount > cryptoDotComConnMaxRetry {
		return errors.New("Crypto.com - Reached max retry connection")
	}
	if err := s.wsConn().Close(); err != nil {
		return err
	}
	if err := s.newConn(); err != nil {
		return err
	}

	var pairs []models.ExchangePair
	s.pairScrapers.Range(func(key, value interface{}) bool {
		pair := value.(models.ExchangePair)
		pairs = append(pairs, pair)
		return true
	})
	if err := s.subscribe(pairs); err != nil {
		return err
	}

	return nil
}

func (s *CryptoDotComScraper) retryTask(taskID int) error {
	val, ok := s.tasks.Load(taskID)
	if !ok {
		return fmt.Errorf("Crypto.com - Facing unknown task id, taskId=%d", taskID)
	}

	task := val.(cryptoDotComWSTask)
	task.RetryCount += 1
	if task.RetryCount > cryptoDotComTaskMaxRetry {
		return fmt.Errorf("CCrypto.com - Exeeding max retry, taskId=%d, %s", taskID, task.toString())
	}

	log.Warnf("Crypto.com - Retrying a task, taskId=%d, %s", taskID, task.toString())
	s.tasks.Store(taskID, task)

	return s.send(taskID, task)
}

func (s *CryptoDotComScraper) send(taskID int, task cryptoDotComWSTask) error {
	s.rl.Take()

	return s.wsConn().WriteJSON(&cryptoDotComWSRequest{
		ID:     taskID,
		Method: task.Method,
		Params: task.Params,
		Nonce:  time.Now().UnixNano() / 1000,
	})
}

// Close unsubscribes data and closes any existing WebSocket connections, as well as channels of CryptoDotComScraper
func (s *CryptoDotComScraper) Close() error {
	if s.isClosed() {
		return errors.New("Crypto.com - Already closed")
	}

	s.signalShutdown.Do(func() {
		close(s.shutdown)
	})

	<-s.shutdownDone

	return s.error()
}
