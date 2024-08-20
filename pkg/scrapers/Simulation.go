package scrapers

import (
	"math"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/diadata-org/decentral-feeder/pkg/contracts/uniswap"
	"github.com/diadata-org/decentral-feeder/pkg/models"
	simulation "github.com/diadata-org/decentral-feeder/pkg/scrapers/simulator"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type SimulationScraper struct {
	pools         []models.Pool
	waitTime      int
	restClient    *ethclient.Client
	simulator     *simulation.Simulator
	allowedTokens map[string]map[string]string
}

type SwapEvents struct {
	Amount0In  int64 `json:"Amount0In"`
	Amount0Out int64 `json:"Amount0Out"`
	Amount1In  int64 `json:"Amount1In"`
	Amount1Out int64 `json:"Amount1Out"`
}

type SimulationResponse struct {
	Blocknumber string       `json:"blocknumber"`
	Events      []SwapEvents `json:"events"`
	Output      float64      `json:"output"`
	TokenIn     string       `json:"tokenInStr"`
	TokenOut    string       `json:"tokenOutStr"`
}

func NewSimulationScraper(pools []models.Pool, tradesChannel chan models.Trade, wg *sync.WaitGroup) {
	var (
		err     error
		scraper SimulationScraper
	)
	scraper.restClient, err = ethclient.Dial(utils.Getenv(UNISWAPV2_EXCHANGE+"_URI_REST", restDial))
	if err != nil {
		log.Error("init rest client: ", err)
	}
	scraper.pools = pools
	// scraper.tradeSimulationRPC = "http://localhost:8085/tradesimulator/symbol" //?symbol=UNI&blocknumber=20333049
	scraper.simulator = simulation.New(scraper.restClient)
	scraper.initTokens()

	log.Info("Started Simulation scraper.")

	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {

			case <-ticker.C:
				log.Info("RUN Simulation scraper.")

				go scraper.mainLoop(pools, tradesChannel)
			}
		}
	}()

}

func (scraper *SimulationScraper) mainLoop(pools []models.Pool, tradesChannel chan models.Trade) {

	// wait for all pairs have added into s.PairScrapers
	time.Sleep(4 * time.Second)

	var wg sync.WaitGroup
	for _, pool := range pools {
		time.Sleep(time.Duration(scraper.waitTime) * time.Millisecond)
		wg.Add(1)
		go func(symbol string, w *sync.WaitGroup) {
			defer w.Done()

			tokens := scraper.allowedTokens[symbol]
			tokenInDecimal, _ := scraper.GetDecimals(common.HexToAddress(tokens["tokenInStr"]))
			tokenOutDecimal, _ := scraper.GetDecimals(common.HexToAddress(tokens["tokenOutStr"]))

			token0 := models.Asset{
				Symbol:   "USDC",
				Name:     "USDC",
				Address:  tokens["tokenInStr"],
				Decimals: tokenInDecimal,

				Blockchain: utils.ETHEREUM,
			}

			token1 := models.Asset{
				Address:  tokens["tokenOutStr"],
				Symbol:   symbol,
				Name:     symbol,
				Decimals: tokenOutDecimal,

				Blockchain: utils.ETHEREUM,
			}
			price, err := scraper.simulator.Execute(token1, token0)
			if err != nil {
				log.Errorf("error getting price of %s ", symbol)
				return

			}

			f, _ := strconv.ParseFloat(price, 64)
			t := models.Trade{
				Price:      1000 / f,
				Volume:     float64(1),
				BaseToken:  token0,
				QuoteToken: token1,
				Time:       time.Now(),
				Exchange:   models.Exchange{Name: Simulation, Blockchain: utils.ETHEREUM},
			}

			tradesChannel <- t

		}(pool.Address, &wg)
	}
	wg.Wait()

}

// func (scraper *SimulationScraper) getSimulatedResult(symbol string, blocknumber uint64) (sr SimulationResponse, err error) {

// 	url := scraper.tradeSimulationRPC + "?symbol=" + symbol + "&blocknumber=" + strconv.Itoa(int(blocknumber))
// 	method := "GET"

// 	client := &http.Client{}
// 	req, err := http.NewRequest(method, url, nil)

// 	if err != nil {
// 		return
// 	}
// 	res, err := client.Do(req)
// 	if err != nil {
// 		return
// 	}
// 	defer res.Body.Close()

// 	body, err := io.ReadAll(res.Body)
// 	if err != nil {
// 		return
// 	}

// 	err = json.Unmarshal(body, &sr)
// 	return
// }

func (scraper *SimulationScraper) GetDecimals(tokenAddress common.Address) (decimals uint8, err error) {

	var contract *uniswap.IERC20Caller
	contract, err = uniswap.NewIERC20Caller(tokenAddress, scraper.restClient)
	if err != nil {
		log.Error(err)
		return
	}
	decimals, err = contract.Decimals(&bind.CallOpts{})

	return
}

func getSimulationSwapData(events []SwapEvents, tokenInDecimal, tokenOutDecimal uint8) (float64, float64) {
	if len(events) == 0 {
		return 0, 0
	}

	decimalsout := int(tokenOutDecimal)
	decimalsin := int(tokenInDecimal)

	firstEvent := events[0]

	lastEvent := events[len(events)-1]

	var totalInput float64

	if firstEvent.Amount0In != int64(0) {
		totalInput, _ = new(big.Float).Quo(big.NewFloat(0).SetInt(big.NewInt(firstEvent.Amount0In)), new(big.Float).SetFloat64(math.Pow10(decimalsin))).Float64()
	} else {
		totalInput, _ = new(big.Float).Quo(big.NewFloat(0).SetInt(big.NewInt(firstEvent.Amount1In)), new(big.Float).SetFloat64(math.Pow10(decimalsin))).Float64()
	}

	var totalOutput float64
	if lastEvent.Amount1Out != int64(0) {
		totalOutput, _ = new(big.Float).Quo(big.NewFloat(0).SetInt(big.NewInt(lastEvent.Amount1Out)), new(big.Float).SetFloat64(math.Pow10(decimalsout))).Float64()
	} else {
		totalOutput, _ = new(big.Float).Quo(big.NewFloat(0).SetInt(big.NewInt(lastEvent.Amount0Out)), new(big.Float).SetFloat64(math.Pow10(decimalsout))).Float64()

	}

	if totalInput == 0 {
		return 0, 0
	}

	price := float64(totalInput) / float64(totalOutput)

	return price, 1000
}

func (scraper *SimulationScraper) initTokens() {
	scraper.allowedTokens = make(map[string]map[string]string)
	var wethConfig = make(map[string]string)
	wethConfig["tokenInStr"] = "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
	wethConfig["tokenOutStr"] = "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"
	wethConfig["amountStr"] = "100000000"
	wethConfig["recipient"] = "0xD6153F5af5679a75cC85D8974463545181f48772"
	scraper.allowedTokens["WETH"] = wethConfig

	var wbtcConfig = make(map[string]string)
	wbtcConfig["tokenInStr"] = "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
	wbtcConfig["amountStr"] = "100000000"
	wbtcConfig["recipient"] = "0xD6153F5af5679a75cC85D8974463545181f48772"

	wbtcConfig["tokenOutStr"] = "0x2260fac5e5542a773aa44fbcfedf7c193bc2c599"
	scraper.allowedTokens["WBTC"] = wbtcConfig

	var uniConfig = make(map[string]string)
	uniConfig["tokenInStr"] = "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
	uniConfig["amountStr"] = "1000000000"
	uniConfig["recipient"] = "0xD6153F5af5679a75cC85D8974463545181f48772"

	uniConfig["tokenOutStr"] = "0x1f9840a85d5af5bf1d1762f925bdaddc4201f984"
	scraper.allowedTokens["UNI"] = uniConfig

	var pepeConfig = make(map[string]string)
	pepeConfig["tokenInStr"] = "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
	pepeConfig["amountStr"] = "100000000"
	pepeConfig["recipient"] = "0xD6153F5af5679a75cC85D8974463545181f48772"

	pepeConfig["tokenOutStr"] = "0x6982508145454ce325ddbe47a25d4ec3d2311933"
	scraper.allowedTokens["PEPE"] = pepeConfig

	var diaConfig = make(map[string]string)
	diaConfig["tokenInStr"] = "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
	diaConfig["amountStr"] = "1000000000"
	diaConfig["recipient"] = "0xD6153F5af5679a75cC85D8974463545181f48772"
	diaConfig["tokenOutStr"] = "0x84cA8bc7997272c7CfB4D0Cd3D55cd942B3c9419"
	scraper.allowedTokens["DIA"] = diaConfig

}
