package simulators

import (
	"math"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/diadata-org/decentral-feeder/pkg/contracts/uniswap"
	"github.com/diadata-org/decentral-feeder/pkg/models"
	simulation "github.com/diadata-org/decentral-feeder/pkg/simulations/simulators/uniswap"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type SimulationScraper struct {
	waitTime   int
	restClient *ethclient.Client
	simulator  *simulation.Simulator
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

var (
	restDial = ""
	// fees are ints with precision 6.
	fees     = big.NewInt(3000)
	amountIn = 1000
)

func NewUniswapSimulator(exchangepairs []models.ExchangePair, tradesChannel chan models.SimulatedTrade, wg *sync.WaitGroup) {
	var (
		err     error
		scraper SimulationScraper
	)
	scraper.restClient, err = ethclient.Dial(utils.Getenv(UNISWAP_SIMULATION+"_URI_REST", restDial))
	if err != nil {
		log.Error("init rest client: ", err)
	}

	scraper.simulator = simulation.New(scraper.restClient, log)
	scraper.initAssets(&exchangepairs)

	log.Info("Started Simulation scraper for assets: ", exchangepairs)

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			log.Info("Simulate trades.")
			scraper.simulateTrades(exchangepairs, tradesChannel)
		}
	}()

}

func (scraper *SimulationScraper) simulateTrades(exchangePairs []models.ExchangePair, tradesChannel chan models.SimulatedTrade) {

	// wait for all pairs have added into s.PairScrapers
	time.Sleep(4 * time.Second)

	var wg sync.WaitGroup

	for _, exchangePair := range exchangePairs {
		time.Sleep(time.Duration(scraper.waitTime) * time.Millisecond)
		wg.Add(1)
		go func(w *sync.WaitGroup) {
			defer w.Done()

			amountOutString, err := scraper.simulator.Execute(exchangePair.UnderlyingPair.QuoteToken, exchangePair.UnderlyingPair.BaseToken, strconv.Itoa(amountIn), fees)
			if err != nil {
				log.Errorf("error getting price of %s - %s ", exchangePair.UnderlyingPair.QuoteToken.Symbol, exchangePair.UnderlyingPair.BaseToken.Symbol)
				return
			}
			amountOut, _ := strconv.ParseFloat(amountOutString, 64)

			t := models.SimulatedTrade{
				Price:      float64(amountIn) / amountOut,
				Volume:     amountOut,
				QuoteToken: exchangePair.UnderlyingPair.QuoteToken,
				BaseToken:  exchangePair.UnderlyingPair.BaseToken,
				Time:       time.Now(),
				Exchange:   Exchanges[UNISWAP_SIMULATION],
			}

			log.Info("got trade: ", t)
			tradesChannel <- t

		}(&wg)
	}
	wg.Wait()

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

func (scraper *SimulationScraper) GetAsset(address common.Address) (asset models.Asset, err error) {
	var contract *uniswap.IERC20Caller
	contract, err = uniswap.NewIERC20Caller(address, scraper.restClient)
	if err != nil {
		log.Error("NewIERC20Caller: ", err)
		return
	}

	asset.Symbol, err = contract.Symbol(&bind.CallOpts{})
	if err != nil {
		log.Warnf("Get Symbol from on-chain for address %s", address)
	}
	asset.Name, err = contract.Name(&bind.CallOpts{})
	if err != nil {
		log.Warnf("Get Name from on-chain for address %s", address)
	}
	asset.Decimals, err = contract.Decimals(&bind.CallOpts{})
	if err != nil {
		log.Errorf("Get Decimals from on-chain for address %s", address)
		return
	}
	asset.Address = address.Hex()
	asset.Blockchain = Exchanges[UNISWAP_SIMULATION].Blockchain

	return
}

// initAssets fetches complete asset data from on-chain for all assets in exchangepairs
func (scraper *SimulationScraper) initAssets(exchangePairs *[]models.ExchangePair) (err error) {
	memoryMap := make(map[string]models.Asset)

	for i, ep := range *exchangePairs {

		if _, ok := memoryMap[ep.UnderlyingPair.QuoteToken.Address]; !ok {
			(*exchangePairs)[i].UnderlyingPair.QuoteToken, err = scraper.GetAsset(common.HexToAddress(ep.UnderlyingPair.QuoteToken.Address))
			if err != nil {
				return
			}
			memoryMap[ep.UnderlyingPair.QuoteToken.Address] = (*exchangePairs)[i].UnderlyingPair.QuoteToken
		} else {
			(*exchangePairs)[i].UnderlyingPair.QuoteToken = memoryMap[ep.UnderlyingPair.QuoteToken.Address]
		}

		if _, ok := memoryMap[ep.UnderlyingPair.BaseToken.Address]; !ok {
			(*exchangePairs)[i].UnderlyingPair.BaseToken, err = scraper.GetAsset(common.HexToAddress(ep.UnderlyingPair.BaseToken.Address))
			if err != nil {
				return
			}
			memoryMap[ep.UnderlyingPair.BaseToken.Address] = (*exchangePairs)[i].UnderlyingPair.BaseToken
		} else {
			(*exchangePairs)[i].UnderlyingPair.BaseToken = memoryMap[ep.UnderlyingPair.BaseToken.Address]
		}
	}
	return
}
