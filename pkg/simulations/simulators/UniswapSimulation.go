package simulators

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/daoleno/uniswapv3-sdk/examples/helper"
	"github.com/diadata-org/decentral-feeder/pkg/contracts/uniswap/univ3factory"
	"github.com/diadata-org/decentral-feeder/pkg/contracts/uniswap/univ3pool"
	"github.com/diadata-org/decentral-feeder/pkg/models"
	simulation "github.com/diadata-org/decentral-feeder/pkg/simulations/simulators/uniswap"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type SimulationScraper struct {
	waitTime      int
	restClient    *ethclient.Client
	luminaClient  *ethclient.Client
	simulator     *simulation.Simulator
	exchangepairs []models.ExchangePair
	// priceMap maps an asset on the current quotation as published by DIA lumina meta contract.
	priceMap map[models.Asset]models.AssetQuotation
	// feesMap maps an exchangepair on the set of admissible fees. More precisely, the set of fees
	// such that a corresponding pool exists and fulfills liquidity requirements.
	feesMap map[models.ExchangePair][]UniV3PoolFee
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

type UniV3PoolFee struct {
	fee      *big.Int
	address  common.Address
	amountIn float64
}

const (
	// Maximal spacing in UniV3 pools with fees of 1%.
	max_spacing = int32(200)
)

var (
	restDial       = ""
	restDialLumina = ""
	// Amount in USD that is used to simulate trades.
	amountIn_USD = float64(100)
	// fees are ints with precision 6.
	allFees = []*big.Int{big.NewInt(100), big.NewInt(500), big.NewInt(3000), big.NewInt(10000)}

	// TO DO: Put the following variables to environment variables.
	DIA_Meta_Contract_Address   = "0x90857994FA959d77728067A71EFA8dc154D89813"
	DIA_Meta_Contract_Precision = 8
	priceMap_Update_Seconds     = 30 * 60
	simulation_Update_Seconds   = 30
	liquidity_Threshold_USD     = float64(50000)
	liquidity_Threshold_Native  = float64(2)
	threshold_Price_Deviation   = float64(0.05)
	word_Range                  = int32(20)
	// Minimal count of active ticks within tick range.
	admissible_Count = 10
	// tick range around the current tick that is taken into account for tick check.
	considered_tick_range int32

	// minWordPosition = int16(-3466)
	// maxWordPosition = int16(3465)
)

func init() {
	var err error
	// Import and cast environment variables.
	DIA_Meta_Contract_Address = utils.Getenv("DIA_META_CONTRACT_ADDRESS", DIA_Meta_Contract_Address)
	DIA_Meta_Contract_Precision, err = strconv.Atoi(utils.Getenv("DIA_META_CONTRACT_PRECISION", strconv.Itoa(DIA_Meta_Contract_Precision)))
	if err != nil {
		log.Errorf("DIA_META_CONTRACT_PRECISION: %v", err)
	}
	priceMap_Update_Seconds, err = strconv.Atoi(utils.Getenv(strings.ToUpper(UNISWAP_SIMULATION)+"_PRICE_MAP_UPDATE_SECONDS", strconv.Itoa(priceMap_Update_Seconds)))
	if err != nil {
		log.Errorf(strings.ToUpper(UNISWAP_SIMULATION)+"_PRICE_MAP_UPDATE_SECONDS: %v", err)
	}
	simulation_Update_Seconds, err = strconv.Atoi(utils.Getenv(strings.ToUpper(UNISWAP_SIMULATION)+"_SIMULATION_UPDATE_SECONDS", strconv.Itoa(simulation_Update_Seconds)))
	if err != nil {
		log.Errorf(strings.ToUpper(UNISWAP_SIMULATION)+"_SIMULATION_UPDATE_SECONDS: %v", err)
	}
	liquidity_Threshold_USD, err = strconv.ParseFloat(utils.Getenv(strings.ToUpper(UNISWAP_SIMULATION)+"_LIQUIDITY_THRESHOLD_USD", strconv.Itoa(int(liquidity_Threshold_USD))), 64)
	if err != nil {
		log.Errorf(strings.ToUpper(UNISWAP_SIMULATION)+"_LIQUIDITY_THRESHOLD_USD: %v", err)
	}
	liquidity_Threshold_Native, err = strconv.ParseFloat(utils.Getenv(strings.ToUpper(UNISWAP_SIMULATION)+"_LIQUIDITY_THRESHOLD_NATIVE", strconv.Itoa(int(liquidity_Threshold_Native))), 64)
	if err != nil {
		log.Errorf(strings.ToUpper(UNISWAP_SIMULATION)+"_LIQUIDITY_THRESHOLD_NATIVE: %v", err)
	}
	threshold_Price_Deviation, err = strconv.ParseFloat(utils.Getenv(strings.ToUpper(UNISWAP_SIMULATION)+"_THRESHOLD_PRICE_DEVIATION", strconv.Itoa(int(threshold_Price_Deviation))), 64)
	if err != nil {
		log.Errorf(strings.ToUpper(UNISWAP_SIMULATION)+"_THRESHOLD_PRICE_DEVIATION: %v", err)
	}
	admissible_Count, err = strconv.Atoi(utils.Getenv(strings.ToUpper(UNISWAP_SIMULATION)+"_ADMISSIBLE_COUNT", strconv.Itoa(admissible_Count)))
	if err != nil {
		log.Errorf(strings.ToUpper(UNISWAP_SIMULATION)+"_ADMISSIBLE_COUNT: %v", err)
	}
	ctr, err := strconv.Atoi(utils.Getenv(strings.ToUpper(UNISWAP_SIMULATION)+"_CONSIDERED_TICK_RANGE", "10"))
	if err != nil {
		log.Errorf(strings.ToUpper(UNISWAP_SIMULATION)+"_CONSIDERED_TICK_RANGE: %v", err)
	}
	considered_tick_range = int32(ctr) * max_spacing

}

func NewUniswapSimulator(exchangepairs []models.ExchangePair, tradesChannel chan models.SimulatedTrade) {
	var (
		err     error
		scraper SimulationScraper
	)

	scraper.restClient, err = ethclient.Dial(utils.Getenv(strings.ToUpper(UNISWAP_SIMULATION)+"_URI_REST", restDial))
	if err != nil {
		log.Error("init rest client: ", err)
	}
	scraper.luminaClient, err = ethclient.Dial(utils.Getenv(strings.ToUpper(UNISWAP_SIMULATION)+"_LUMINA_URI_REST", restDialLumina))
	if err != nil {
		log.Error("init lumina client: ", err)
	}

	scraper.simulator = simulation.New(scraper.restClient, log)
	scraper.exchangepairs = exchangepairs
	scraper.initAssetsAndMaps()

	var lock sync.RWMutex
	scraper.updatePriceMap(&lock)
	// map exchangepairs to list of fees corresponding to deployed pools such that all mapped pools are admissible.
	scraper.updateFeesMap(&lock)

	for key, poolFee := range scraper.feesMap {
		log.Infof("admissible pair %s -- %s : ", key.UnderlyingPair.QuoteToken.Symbol, key.UnderlyingPair.BaseToken.Symbol)
		for _, v := range poolFee {
			log.Infof("fee -- address: %s -- %s", v.fee.String(), v.address.Hex())
		}
		log.Info("-------------------------------------------------------")
	}

	priceTicker := time.NewTicker(time.Duration(priceMap_Update_Seconds) * time.Second)
	go func() {
		for range priceTicker.C {
			scraper.updatePriceMap(&lock)
			scraper.updateFeesMap(&lock)
		}
	}()

	for pair := range scraper.feesMap {
		log.Infof("Start Simulation scraper for pair: %s-%s", pair.UnderlyingPair.QuoteToken.Symbol, pair.UnderlyingPair.BaseToken.Symbol)
	}
	ticker := time.NewTicker(time.Duration(simulation_Update_Seconds) * time.Second)
	for range ticker.C {
		log.Info("Simulate trades.")
		scraper.simulateTrades(tradesChannel)
	}

}

func (scraper *SimulationScraper) simulateTrades(tradesChannel chan models.SimulatedTrade) {

	var wg sync.WaitGroup

	for exchangePair, fees := range scraper.feesMap {
		time.Sleep(time.Duration(scraper.waitTime) * time.Millisecond)
		for _, poolFee := range fees {
			wg.Add(1)

			go func(ep models.ExchangePair, fee *big.Int, w *sync.WaitGroup) {
				defer w.Done()
				amountIn := strconv.FormatFloat(poolFee.amountIn, 'f', -1, 64)

				amountOutString, err := scraper.simulator.Execute(
					ep.UnderlyingPair.QuoteToken,
					ep.UnderlyingPair.BaseToken,
					amountIn,
					fee,
				)
				if err != nil {
					log.Errorf("error getting price of %s - %s ", ep.UnderlyingPair.QuoteToken.Symbol, ep.UnderlyingPair.BaseToken.Symbol)
					return
				}
				amountOut, _ := strconv.ParseFloat(amountOutString, 64)
				var address common.Address
				poolFees := scraper.feesMap[ep]
				for _, pf := range poolFees {
					if pf.fee == fee {
						address = pf.address
					}
				}

				t := models.SimulatedTrade{
					Price:       poolFee.amountIn / amountOut,
					Volume:      amountOut,
					QuoteToken:  ep.UnderlyingPair.QuoteToken,
					BaseToken:   ep.UnderlyingPair.BaseToken,
					PoolAddress: address.Hex(),
					Time:        time.Now(),
					Exchange:    Exchanges[UNISWAP_SIMULATION],
				}

				log.Infof("Got trade in pool %v%%: %s-%s -- %v -- %v", float64(fee.Int64())/float64(10000), t.QuoteToken.Symbol, t.BaseToken.Symbol, t.Price, t.Volume)
				tradesChannel <- t

			}(exchangePair, poolFee.fee, &wg)
		}
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

// initAssets fetches complete asset data from on-chain for all assets in exchangepairs.
// It also initializes the keys of @scraper.priceMap and keys of @scraper.feesMap.
func (scraper *SimulationScraper) initAssetsAndMaps() error {

	// memoryMap prevents from iterating over assets twice.
	memoryMap := make(map[string]models.Asset)
	scraper.priceMap = make(map[models.Asset]models.AssetQuotation)
	scraper.feesMap = make(map[models.ExchangePair][]UniV3PoolFee)

	for i, ep := range scraper.exchangepairs {

		if _, ok := memoryMap[ep.UnderlyingPair.QuoteToken.Address]; !ok {

			quoteToken, err := models.GetAsset(common.HexToAddress(ep.UnderlyingPair.QuoteToken.Address), Exchanges[UNISWAP_SIMULATION].Blockchain, scraper.restClient)
			if err != nil {
				return err
			}
			// symbol in Maker contract is null string.
			if quoteToken.Address == "0x9f8F72aA9304c8B593d555F12eF6589cC3A579A2" {
				quoteToken.Symbol = "MKR"
			}
			scraper.exchangepairs[i].UnderlyingPair.QuoteToken = quoteToken
			scraper.priceMap[quoteToken] = models.AssetQuotation{}
			memoryMap[ep.UnderlyingPair.QuoteToken.Address] = quoteToken

		} else {
			scraper.exchangepairs[i].UnderlyingPair.QuoteToken = memoryMap[ep.UnderlyingPair.QuoteToken.Address]
		}

		if _, ok := memoryMap[ep.UnderlyingPair.BaseToken.Address]; !ok {

			baseToken, err := models.GetAsset(common.HexToAddress(ep.UnderlyingPair.BaseToken.Address), Exchanges[UNISWAP_SIMULATION].Blockchain, scraper.restClient)
			if err != nil {
				return err
			}
			scraper.exchangepairs[i].UnderlyingPair.BaseToken = baseToken
			scraper.priceMap[baseToken] = models.AssetQuotation{}
			memoryMap[ep.UnderlyingPair.BaseToken.Address] = baseToken

		} else {
			scraper.exchangepairs[i].UnderlyingPair.BaseToken = memoryMap[ep.UnderlyingPair.BaseToken.Address]
		}
		// Initialize @scraper.feesMap.
		scraper.feesMap[scraper.exchangepairs[i]] = []UniV3PoolFee{}

	}
	return nil
}

// updatePriceMap fetches the current price of each of the involved assets from DIA lumina meta contract.
func (scraper *SimulationScraper) updatePriceMap(lock *sync.RWMutex) {
	for asset := range scraper.priceMap {
		quotation, err := asset.GetOnchainPrice(common.HexToAddress(DIA_Meta_Contract_Address), DIA_Meta_Contract_Precision, scraper.luminaClient)
		if err != nil {
			log.Errorf("GetOnchainPrice for %s -- %s: %v", asset.Symbol, asset.Address, err)
			continue
		} else {
			log.Infof("USD price for (base-)token %s: %v", asset.Symbol, quotation.Price)
		}
		lock.Lock()
		scraper.priceMap[asset] = quotation
		lock.Unlock()
	}
}

// updateFeesMap updates values in scraper.feesMap.
func (scraper *SimulationScraper) updateFeesMap(lock *sync.RWMutex) {

	// Remark: In case initial load is too slow, this loop can be parallelized. Not sure if it works with ETH requests though.
	for _, ep := range scraper.exchangepairs {
		quoteToken := ep.UnderlyingPair.QuoteToken
		baseToken := ep.UnderlyingPair.BaseToken

		var indexFeeMap = make(map[int]*big.Int)
		var prices0, prices1 []float64
		var poolMap = make(map[int]common.Address)

		for i, fee := range allFees {

			poolAddress, err := scraper.getPool(ep, fee)
			if err != nil {
				log.Errorf("getPool with address %s and fee %v: %v", poolAddress.Hex(), float64(fee.Int64())/float64(10000), err)
				continue
			} else if (poolAddress == common.Address{}) {
				log.Warnf("pool for %s-%s with fees %v does not exist.", quoteToken.Symbol, baseToken.Symbol, float64(fee.Int64())/float64(10000))
				continue
			} else {
				log.Infof("Start checking admissibility for pool %s-%s with fee %v%% and address %s ", ep.UnderlyingPair.QuoteToken.Symbol, ep.UnderlyingPair.BaseToken.Symbol, float64(fee.Int64())/float64(10000), poolAddress.Hex())
			}

			// Check if pool is admissible, i.e.
			// 1. check balance(s).
			// 2. check distribution of active ticks.
			// 3. check prices in current tick across pools/fees.

			balanceOk := scraper.checkBalances(quoteToken, baseToken, poolAddress)
			if !balanceOk {
				log.Warnf("Balances not ok for pool with fee %v%%", float64(fee.Int64())/float64(10000))
				// Remove from scraper.feesMap[ep] if existent.
				if containsAddress(poolAddress, scraper.feesMap[ep]) {
					log.Warn("low balance - remove pool from set of admissible pools: ", poolAddress)
					cleanedFees := removeFeeByAddress(poolAddress, scraper.feesMap[ep])
					scraper.feesMap[ep] = cleanedFees
				}
				continue
			}

			ticksOk, currentTick := scraper.checkTicks(poolAddress, word_Range, considered_tick_range, admissible_Count)
			if !ticksOk {
				log.Warnf("ticks not ok for %s with fee %s", poolAddress.Hex(), fee.String())
				// Remove from scraper.feesMap[ep] if existent.
				if containsAddress(poolAddress, scraper.feesMap[ep]) {
					log.Warn("poor tick distribution - remove pool from set of admissible pools: ", poolAddress)
					cleanedFees := removeFeeByAddress(poolAddress, scraper.feesMap[ep])
					scraper.feesMap[ep] = cleanedFees
				}
				continue
			}

			indexFeeMap[i] = fee
			poolMap[i] = poolAddress

			// logging ------------
			priceMin, _ := computeTickPrices(currentTick-considered_tick_range, int8(quoteToken.Decimals), int8(baseToken.Decimals))
			priceMax, _ := computeTickPrices(currentTick+considered_tick_range, int8(quoteToken.Decimals), int8(baseToken.Decimals))
			log.Infof("corresponding price range in tick range: %s -- %s", priceMin.String(), priceMax.String())
			log.Infof("pool admitted %s for balances and ticks.", poolAddress.Hex())
			// --------------------

			p0, p1, err := scraper.getActivePrices(poolAddress, int8(quoteToken.Decimals), int8(baseToken.Decimals))
			if err != nil {
				log.Errorf("getActivePrices on %s: %v", poolAddress.Hex(), err)
			}
			prices0 = append(prices0, p0)
			prices1 = append(prices1, p1)
			log.Debugf("prices in current tick price0 -- price1: %v -- %v", prices0, prices1)
		}

		// Outlier detection on prices.
		// TO DO: Should we also check for prices1?
		scraper.checkPrices(prices0, ep, indexFeeMap, poolMap)

		// Compute amountIn in such that it corresponds to @amountIn_USD amount in USD.
		baseTokenPrice := scraper.priceMap[ep.UnderlyingPair.BaseToken].Price
		if baseTokenPrice == 0 {
			log.Warnf("Could not determine price of base token %s. Continue with native volume of 1.", ep.UnderlyingPair.BaseToken.Symbol)
			baseTokenPrice = 100
		}
		for i := range scraper.feesMap[ep] {
			lock.Lock()
			scraper.feesMap[ep][i].amountIn = amountIn_USD / baseTokenPrice
			lock.Unlock()
		}

		// Remove from map in case no admissible pool/fee was found.
		if len(scraper.feesMap[ep]) == 0 {
			delete(scraper.feesMap, ep)
		}

	}

}

// --------------------------------------------------------------------------------------------------------------

func (scraper *SimulationScraper) checkBalances(quoteToken models.Asset, baseToken models.Asset, poolAddress common.Address) bool {
	balance0, err := quoteToken.GetBalance(poolAddress, scraper.restClient)
	if err != nil {
		log.Errorf("GetBalance of %s: %v", quoteToken.Address, err)
	}
	balance1, err := baseToken.GetBalance(poolAddress, scraper.restClient)
	if err != nil {
		log.Errorf("GetBalance of %s: %v", baseToken.Address, err)
	}
	if balance0 < liquidity_Threshold_Native {
		log.Warnf("native liquidity for %s in %s-%s with address %s not sufficient: %v", quoteToken.Symbol, quoteToken.Symbol, baseToken.Symbol, poolAddress.Hex(), balance0)
		return false
	}
	if balance1 < liquidity_Threshold_Native {
		log.Warnf("native liquidity for %s in %s-%s  with address %s not sufficient: %v", baseToken.Symbol, quoteToken.Symbol, baseToken.Symbol, poolAddress.Hex(), balance1)
		return false
	}

	balance0USD := balance0 * scraper.priceMap[quoteToken].Price
	balance1USD := balance1 * scraper.priceMap[baseToken].Price
	if 0 < balance0USD && balance0USD < liquidity_Threshold_USD {
		log.Warnf("USD liquidity for %s in %s-%s  with address %s not sufficient: %v", quoteToken.Symbol, quoteToken.Symbol, baseToken.Symbol, poolAddress.Hex(), balance0USD)
		return false
	}
	if 0 < balance1USD && balance1USD < liquidity_Threshold_USD {
		log.Warnf("USD liquidity for %s in %s-%s  with address %s not sufficient: %v", baseToken.Symbol, quoteToken.Symbol, baseToken.Symbol, poolAddress.Hex(), balance1USD)
		return false
	}
	return true
}

// checkTicks counts active ticks within given @wordRange.
// @admissibleSteps is the number of steps within which range ticks are taken into account.
// @admissibleCount is the number of active ticks which have to lie within the given range.
func (scraper *SimulationScraper) checkTicks(poolAddress common.Address, wordRange int32, admissibleRange int32, admissibleCount int) (ok bool, currentTick int32) {
	ticks, err := scraper.getCurrentTicks(poolAddress, wordRange)
	if err != nil {
		log.Error("getCurrentTicks: ", err)
	} else {
		log.Debug("ticks: ", ticks)
	}

	var caller *univ3pool.Univ3poolCaller
	caller, err = univ3pool.NewUniv3poolCaller(poolAddress, scraper.restClient)
	if err != nil {
		return
	}
	slot0, err := caller.Slot0(&bind.CallOpts{})
	if err != nil {
		return
	}

	currentTick = int32(slot0.Tick.Int64())
	minTick := currentTick - admissibleRange
	maxTick := currentTick + admissibleRange
	ok = scraper.checkTickCount(ticks, minTick, maxTick, admissibleCount)
	return
}

func (scraper *SimulationScraper) checkPrices(prices []float64, ep models.ExchangePair, indexMap map[int]*big.Int, poolMap map[int]common.Address) {
	log.Infof("checking price outliers for %v pools %s-%s ", len(prices), ep.UnderlyingPair.QuoteToken.Symbol, ep.UnderlyingPair.BaseToken.Symbol)
	// TO DO: Only append if not existent yet.
	if len(prices) == 1 {
		for index, fee := range indexMap {
			if !containsFee(fee, scraper.feesMap[ep]) {
				scraper.feesMap[ep] = append(scraper.feesMap[ep], UniV3PoolFee{fee: fee, address: poolMap[index]})
			}
		}
	}
	if len(prices) == 2 {
		// If deviation too large store both and print warning.
		dist := utils.AvgDistances(prices)
		if dist[0] > threshold_Price_Deviation {
			log.Warnf("prices in pools %s and %s differ by more than %v: %v -- %v ",
				"poolAddress",
				"poolAddress",
				threshold_Price_Deviation,
				prices[0],
				prices[1],
			)
		} else {
			for index, fee := range indexMap {
				if !containsFee(fee, scraper.feesMap[ep]) {
					scraper.feesMap[ep] = append(scraper.feesMap[ep], UniV3PoolFee{fee: fee, address: poolMap[index]})
				}
			}
		}
	}
	if len(prices) > 2 {
		_, indices := utils.RemoveOutliers(prices, threshold_Price_Deviation)
		for _, ind := range indices {
			if !containsFee(indexMap[ind], scraper.feesMap[ep]) {
				scraper.feesMap[ep] = append(scraper.feesMap[ep], UniV3PoolFee{fee: indexMap[ind], address: poolMap[ind]})
			}
		}
	}
}

// --------------------------------------------------------------------------------------------------------------

// checkTicks returns true, if @tickCount is greater or equal to @tickThreshold. tickCount is measured
// by counting, how many active ticks lie within @minTick and @maxTick.
// steps measured with respect to @tickSpacing.
func (scraper *SimulationScraper) checkTickCount(ticks []int32, minTick int32, maxTick int32, admissibleCount int) (ok bool) {
	var tickCount int
	for _, tick := range ticks {
		if minTick <= tick && tick <= maxTick {
			tickCount++
		}
	}

	log.Infof("%v active ticks out of total %v", tickCount, len(ticks))
	if tickCount > admissibleCount {
		ok = true
	}
	return
}

// getActivePrices returns prices of pool tokens in the currently active tick. Prices denomination is of course native.
func (scraper *SimulationScraper) getActivePrices(poolAddress common.Address, decimals0 int8, decimals1 int8) (price0 float64, price1 float64, err error) {
	var caller *univ3pool.Univ3poolCaller
	caller, err = univ3pool.NewUniv3poolCaller(poolAddress, scraper.restClient)
	if err != nil {
		return
	}
	slot0, err := caller.Slot0(&bind.CallOpts{})
	if err != nil {
		return
	}
	currentTick := slot0.Tick

	price0Big, price1Big := computeTickPrices(int32(currentTick.Int64()), decimals0, decimals1)
	price0, _ = price0Big.Float64()
	price1, _ = price1Big.Float64()
	return
}

// getCurrentTicks returns active ticks within a range of @wordRange around slot0.
func (scraper *SimulationScraper) getCurrentTicks(poolAddress common.Address, wordRange int32) (ticks []int32, err error) {

	var caller *univ3pool.Univ3poolCaller
	caller, err = univ3pool.NewUniv3poolCaller(poolAddress, scraper.restClient)
	if err != nil {
		return
	}
	slot0, err := caller.Slot0(&bind.CallOpts{})
	if err != nil {
		return
	}
	tickSpacing, err := caller.TickSpacing(&bind.CallOpts{})
	if err != nil {
		return
	}
	log.Debug("ticks spacing: ", tickSpacing.String())

	currentWordPosition := getWordPosition(int32(slot0.Tick.Int64()), int32(tickSpacing.Int64()))
	ticks, err = scraper.getActiveTicksInRange(poolAddress, currentWordPosition-wordRange, currentWordPosition+wordRange)
	if err != nil {
		log.Fatal("getAllActiveTicks: ", err)
	}

	return
}

// getPool returns the unique pool containing both assets from @exchangepair with given @fee if it exists.
func (scraper *SimulationScraper) getPool(exchangepair models.ExchangePair, fee *big.Int) (poolAddress common.Address, err error) {
	var caller *univ3factory.Univ3factoryCaller
	caller, err = univ3factory.NewUniv3factoryCaller(common.HexToAddress(helper.ContractV3Factory), scraper.restClient)
	if err != nil {
		return
	}
	log.Debugf("get pool for addresses %s -- %s", exchangepair.UnderlyingPair.QuoteToken.Address, exchangepair.UnderlyingPair.BaseToken.Address)
	poolAddress, err = caller.GetPool(
		&bind.CallOpts{},
		common.HexToAddress(exchangepair.UnderlyingPair.QuoteToken.Address),
		common.HexToAddress(exchangepair.UnderlyingPair.BaseToken.Address),
		fee,
	)
	return
}

// --------------------------------------------------------------------------------------------------------------

// getWordPosition calculates the correct word position in tickBitmap taking tickSpacing into account.
func getWordPosition(tickIndex int32, tickSpacing int32) int32 {
	// Normalize tickIndex by tickSpacing before computing word position
	scaledTickIndex := tickIndex / tickSpacing
	if scaledTickIndex < 0 {
		return (scaledTickIndex / 256) - 1 // Handle negative tick indices correctly
	}
	return scaledTickIndex / 256
}

// getActiveTicksInRange retrieves all active ticks using the tickBitmap structure
func (scraper *SimulationScraper) getActiveTicksInRange(poolAddress common.Address, wordPositionLeft int32, wordPositionRight int32) (ticks []int32, err error) {

	caller, err := univ3pool.NewUniv3poolCaller(poolAddress, scraper.restClient)
	if err != nil {
		return
	}
	tickSpacing, err := caller.TickSpacing(&bind.CallOpts{})
	if err != nil {
		return
	}
	log.Debug("tickSpacing: ", tickSpacing)
	log.Debug("poolAddress for following ticks: ", poolAddress.Hex())

	for wordPosition := wordPositionLeft; wordPosition <= wordPositionRight; wordPosition++ {

		var tickBitmap *big.Int
		tickBitmap, err = caller.TickBitmap(&bind.CallOpts{}, int16(wordPosition))
		if err != nil {
			return
		}

		// Convert bitmap to active tick indices
		activeTicks := extractActiveTicksFromBitmap(wordPosition, int32(tickSpacing.Int64()), tickBitmap)
		log.Debugf("activeTicks at wordPosition %v: %v", wordPosition, activeTicks)
		ticks = append(ticks, activeTicks...)

	}
	return
}

// extractActiveTicksFromBitmap extracts active ticks from a Uniswap tickBitmap word
func extractActiveTicksFromBitmap(wordPosition int32, tickSpacing int32, tickBitmap *big.Int) []int32 {
	var activeTicks []int32

	for i := 0; i < 256; i++ {
		// Check if the bit at position `i` is set in the bitmap
		if tickBitmap.Bit(i) == 1 {
			// Compute the tick index correctly, considering tickSpacing
			scaledTickIndex := (wordPosition * 256) + int32(i)
			tickIndex := scaledTickIndex * tickSpacing // Scale back to actual tick index

			activeTicks = append(activeTicks, tickIndex)
		}
	}

	return activeTicks
}

func containsFee(fee *big.Int, fees []UniV3PoolFee) bool {
	for _, f := range fees {
		if fee.Cmp(f.fee) == 0 {
			return true
		}
	}
	return false
}

func containsAddress(address common.Address, fees []UniV3PoolFee) bool {
	for _, f := range fees {
		if address == f.address {
			return true
		}
	}
	return false
}

func removeFeeByAddress(address common.Address, fees []UniV3PoolFee) []UniV3PoolFee {
	for i, f := range fees {
		if address == f.address {
			return append(fees[:i], fees[i+1:]...)
		}
	}
	return fees
}

// ------------------------------------------------------------------------------------------------------------------------

// computePrices calculates prices of token0 and token1 with respect to vice versa.
func computeTickPrices(tick int32, decimals0 int8, decimals1 int8) (*big.Float, *big.Float) {
	base := big.NewFloat(1.0001)
	numerator := powBigFloat(base, big.NewFloat(float64(tick)))
	denominator := powBigFloat(big.NewFloat(10), big.NewFloat(float64(decimals1-decimals0)))
	return big.NewFloat(0).Quo(numerator, denominator), big.NewFloat(0).Quo(denominator, numerator)
}

// powBigFloat calculates x^y using ln(x) * y and exp(result)
func powBigFloat(x, y *big.Float) *big.Float {
	// Ensure high precision
	precision := uint(100)
	x.SetPrec(precision)
	y.SetPrec(precision)

	// Convert x to float64 for math.Log
	xFloat64, _ := x.Float64()
	if xFloat64 <= 0 {
		panic("x must be greater than 0 for real-valued exponentiation")
	}

	// Compute ln(x) * y
	lnX := math.Log(xFloat64)  // Compute ln(x) as float64
	yFloat64, _ := y.Float64() // Convert y to float64
	result := big.NewFloat(0).SetPrec(precision)
	result.SetFloat64(lnX * yFloat64) // Multiply ln(x) * y

	// Compute exp(result)
	expFloat, _ := result.Float64()
	expResult := big.NewFloat(math.Exp(expFloat))

	return expResult
}
