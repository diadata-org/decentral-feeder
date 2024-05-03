package scrapers

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	models "github.com/diadata-org/diaprotocol/pkg/models"
	ws "github.com/gorilla/websocket"
)

var tickerPairMap = make(map[string]models.Pair)

func init() {
	tickerPairMap["BTCUSDT"] = models.Pair{
		QuoteToken: models.Asset{Address: "0x0000000000000000000000000000000000000000", Blockchain: "Bitcoin", Symbol: "BTC"},
		BaseToken:  models.Asset{Address: "0xdAC17F958D2ee523a2206206994597C13D831ec7", Blockchain: "Ethereum", Symbol: "USDT"},
	}

}

func NewBinanceScraper(exchange string, pairs []string, tradesChannel chan models.Trade, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Info("Entered Binance handler")

	wsBaseString := "wss://stream.binance.com:9443/ws/"
	wsAssetsString := ""

	for _, pair := range pairs {
		wsAssetsString += strings.ToLower(strings.Split(pair, "-")[0]) + strings.ToLower(strings.Split(pair, "-")[1]) + "@trade" + "/"
	}
	// Remove trailing slash
	wsAssetsString = wsAssetsString[:len(wsAssetsString)-1]
	conn, _, err := ws.DefaultDialer.Dial(wsBaseString+wsAssetsString, nil)
	if err != nil {
		log.Fatal("connect to Binance API.")
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Errorln("read:", err)
		}
		//log.Printf("recv Binance: %s", message)
		messageMap := make(map[string]interface{})
		err = json.Unmarshal(message, &messageMap)
		if err != nil {
			continue
		}
		var trade models.Trade

		trade.Time = time.Unix(int64(messageMap["T"].(float64))/1000, 0)
		// TO DO: Correct parsing of timestamp

		trade.Price, err = strconv.ParseFloat(messageMap["p"].(string), 64)
		if err != nil {
			log.Error("Parse price: ", err)
		}

		trade.Volume, err = strconv.ParseFloat(messageMap["q"].(string), 64)
		if err != nil {
			log.Error("Parse volume: ", err)
		}
		if !messageMap["m"].(bool) {
			trade.Volume -= 1
		}

		trade.ForeignTradeID = strconv.Itoa(int(messageMap["a"].(float64)))
		trade.QuoteToken = tickerPairMap[messageMap["s"].(string)].QuoteToken
		trade.BaseToken = tickerPairMap[messageMap["s"].(string)].BaseToken

		tradesChannel <- trade

	}
}
