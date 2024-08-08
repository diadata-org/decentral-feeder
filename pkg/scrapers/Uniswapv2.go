package scrapers

import (
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/diadata-org/decentral-feeder/pkg/contracts/uniswap"
	"github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	restDial = ""
	wsDial   = ""
	poolMap  = make(map[string]UniswapPair)
)

type UniswapToken struct {
	Address  common.Address
	Symbol   string
	Decimals uint8
	Name     string
}

type UniswapPair struct {
	Token0      UniswapToken
	Token1      UniswapToken
	ForeignName string
	Address     common.Address
}

type UniswapSwap struct {
	ID         string
	Timestamp  int64
	Pair       UniswapPair
	Amount0In  float64
	Amount0Out float64
	Amount1In  float64
	Amount1Out float64
}

type UniswapV2Scraper struct {
	pools      []models.Pool
	wsClient   *ethclient.Client
	restClient *ethclient.Client
	waitTime   int
}

func NewUniswapV2Scraper(pools []models.Pool, tradesChannel chan models.Trade, wg *sync.WaitGroup) {
	var err error
	var scraper UniswapV2Scraper
	log.Info("Started UniswapV2 scraper.")

	scraper.restClient, err = ethclient.Dial(utils.Getenv(UNISWAPV2_EXCHANGE+"_URI_REST", restDial))
	if err != nil {
		log.Error("UniswapV2 - init rest client: ", err)
	}
	scraper.wsClient, err = ethclient.Dial(utils.Getenv(UNISWAPV2_EXCHANGE+"_URI_WS", wsDial))
	if err != nil {
		log.Error("UniswapV2 - init ws client: ", err)
	}

	// TO DO: Import through env var.
	scraper.waitTime = 500
	// Fetch all pool with given liquidity threshold from database.
	poolMap, err = scraper.makeUniPoolMap(pools)
	if err != nil {
		log.Error("UniswapV2 - build poolMap: ", err)
	}

	go scraper.mainLoop(pools, tradesChannel)
}

// runs in a goroutine until s is closed
func (scraper *UniswapV2Scraper) mainLoop(pools []models.Pool, tradesChannel chan models.Trade) {

	// wait for all pairs have added into s.PairScrapers
	time.Sleep(4 * time.Second)

	var wg sync.WaitGroup
	for _, pool := range pools {
		time.Sleep(time.Duration(scraper.waitTime) * time.Millisecond)
		wg.Add(1)
		go func(address common.Address, w *sync.WaitGroup) {
			defer w.Done()
			scraper.ListenToPair(address, tradesChannel)
		}(common.HexToAddress(pool.Address), &wg)
	}
	wg.Wait()

}

// makeUniPoolMap returns a map with pool addresses as keys and the underlying UniswapPair as values.
func (scraper *UniswapV2Scraper) makeUniPoolMap(pools []models.Pool) (map[string]UniswapPair, error) {
	pm := make(map[string]UniswapPair)
	var err error

	for _, p := range pools {
		pm[p.Address], err = scraper.GetPairByAddress(common.HexToAddress(p.Address))
		if err != nil {
			log.Error("UniswapV2 - GetPairByAddress for ", p.Address)
			return pm, err
		}
	}
	return pm, nil
}

// ListenToPair subscribes to a uniswap pool.
// If @byAddress is true, it listens by pool address, otherwise by index.
func (scraper *UniswapV2Scraper) ListenToPair(address common.Address, tradesChannel chan models.Trade) {
	var err error

	// Relevant pool info is retrieved from @poolMap.
	pair := poolMap[address.Hex()]

	sink, err := scraper.GetSwapsChannel(address)
	if err != nil {
		log.Error("UniswapV2 - error fetching swaps channel: ", err)
	}

	go func() {
		for {
			rawSwap, ok := <-sink
			if ok {
				swap, err := scraper.normalizeUniswapSwap(*rawSwap, pair)
				if err != nil {
					log.Error("UniswapV2 - error normalizing swap: ", err)
				}
				price, volume := getSwapData(swap)
				token0 := models.Asset{
					Address:    pair.Token0.Address.Hex(),
					Symbol:     pair.Token0.Symbol,
					Name:       pair.Token0.Name,
					Decimals:   pair.Token0.Decimals,
					Blockchain: utils.ETHEREUM,
				}
				token1 := models.Asset{
					Address:    pair.Token1.Address.Hex(),
					Symbol:     pair.Token1.Symbol,
					Name:       pair.Token1.Name,
					Decimals:   pair.Token1.Decimals,
					Blockchain: utils.ETHEREUM,
				}
				t := models.Trade{
					Price:          price,
					Volume:         volume,
					BaseToken:      token1,
					QuoteToken:     token0,
					Time:           time.Unix(swap.Timestamp, 0),
					PoolAddress:    rawSwap.Raw.Address.Hex(),
					ForeignTradeID: swap.ID,
					Exchange:       models.Exchange{Name: UNISWAPV2_EXCHANGE, Blockchain: utils.ETHEREUM},
				}

				// log.Info("tx hash: ", swap.ID)
				// log.Infof(
				// 	"Got trade at time %v - symbol: %s, pair: %s, price: %v, volume:%v",
				// 	t.Time,
				// 	t.QuoteToken.Symbol,
				// 	t.QuoteToken.Symbol+"-"+t.BaseToken.Symbol,
				// 	t.Price,
				// 	t.Volume,
				// )
				tradesChannel <- t

			}
		}
	}()
}

// GetSwapsChannel returns a channel for swaps of the pair with address @pairAddress
func (scraper *UniswapV2Scraper) GetSwapsChannel(pairAddress common.Address) (chan *uniswap.UniswapV2PairSwap, error) {

	sink := make(chan *uniswap.UniswapV2PairSwap)
	var pairFiltererContract *uniswap.UniswapV2PairFilterer
	pairFiltererContract, err := uniswap.NewUniswapV2PairFilterer(pairAddress, scraper.wsClient)
	if err != nil {
		log.Error("UniswapV2 - pair filterer: ", err)
	}

	_, err = pairFiltererContract.WatchSwap(&bind.WatchOpts{}, sink, []common.Address{}, []common.Address{})
	if err != nil {
		log.Error("UniswapV2 - error in get swaps channel: ", err)
	}

	return sink, nil
}

// GetPairByAddress returns the UniswapPair with pair address @pairAddress
func (scraper *UniswapV2Scraper) GetPairByAddress(pairAddress common.Address) (pair UniswapPair, err error) {
	connection := scraper.restClient
	var pairContract *uniswap.IUniswapV2PairCaller
	pairContract, err = uniswap.NewIUniswapV2PairCaller(pairAddress, connection)
	if err != nil {
		log.Error("UniswapV2 - " + err.Error())
		return UniswapPair{}, err
	}

	// Getting tokens from pair ---------------------
	address0, _ := pairContract.Token0(&bind.CallOpts{})
	address1, _ := pairContract.Token1(&bind.CallOpts{})
	var token0Contract *uniswap.IERC20Caller
	var token1Contract *uniswap.IERC20Caller
	token0Contract, err = uniswap.NewIERC20Caller(address0, connection)
	if err != nil {
		log.Error("UniswapV2 - " + err.Error())
	}
	token1Contract, err = uniswap.NewIERC20Caller(address1, connection)
	if err != nil {
		log.Error("UniswapV2 - " + err.Error())
	}
	symbol0, err := token0Contract.Symbol(&bind.CallOpts{})
	if err != nil {
		log.Error("UniswapV2 - " + err.Error())
	}
	symbol1, err := token1Contract.Symbol(&bind.CallOpts{})
	if err != nil {
		log.Error("UniswapV2 - " + err.Error())
	}
	decimals0, err := scraper.GetDecimals(address0)
	if err != nil {
		log.Error("UniswapV2 - " + err.Error())
		return UniswapPair{}, err
	}
	decimals1, err := scraper.GetDecimals(address1)
	if err != nil {
		log.Error("UniswapV2 - " + err.Error())
		return UniswapPair{}, err
	}

	name0, err := scraper.GetName(address0)
	if err != nil {
		log.Error("UniswapV2 - " + err.Error())
		return UniswapPair{}, err
	}
	name1, err := scraper.GetName(address1)
	if err != nil {
		log.Error("UniswapV2 - " + err.Error())
		return UniswapPair{}, err
	}
	token0 := UniswapToken{
		Address:  address0,
		Symbol:   symbol0,
		Decimals: decimals0,
		Name:     name0,
	}
	token1 := UniswapToken{
		Address:  address1,
		Symbol:   symbol1,
		Decimals: decimals1,
		Name:     name1,
	}
	foreignName := symbol0 + "-" + symbol1
	pair = UniswapPair{
		ForeignName: foreignName,
		Address:     pairAddress,
		Token0:      token0,
		Token1:      token1,
	}
	return pair, nil
}

// GetDecimals returns the decimals of the token with address @tokenAddress
func (scraper *UniswapV2Scraper) GetDecimals(tokenAddress common.Address) (decimals uint8, err error) {

	var contract *uniswap.IERC20Caller
	contract, err = uniswap.NewIERC20Caller(tokenAddress, scraper.restClient)
	if err != nil {
		log.Error("UniswapV2 - " + err.Error())
		return
	}
	decimals, err = contract.Decimals(&bind.CallOpts{})

	return
}

func (scraper *UniswapV2Scraper) GetName(tokenAddress common.Address) (name string, err error) {

	var contract *uniswap.IERC20Caller
	contract, err = uniswap.NewIERC20Caller(tokenAddress, scraper.restClient)
	if err != nil {
		log.Error("UniswapV2 - " + err.Error())
		return
	}
	name, err = contract.Name(&bind.CallOpts{})

	return
}

// normalizeUniswapSwap takes a swap as returned by the swap contract's channel and converts it to a UniswapSwap type
func (scraper *UniswapV2Scraper) normalizeUniswapSwap(swap uniswap.UniswapV2PairSwap, pair UniswapPair) (normalizedSwap UniswapSwap, err error) {

	decimals0 := int(pair.Token0.Decimals)
	decimals1 := int(pair.Token1.Decimals)
	amount0In, _ := new(big.Float).Quo(big.NewFloat(0).SetInt(swap.Amount0In), new(big.Float).SetFloat64(math.Pow10(decimals0))).Float64()
	amount0Out, _ := new(big.Float).Quo(big.NewFloat(0).SetInt(swap.Amount0Out), new(big.Float).SetFloat64(math.Pow10(decimals0))).Float64()
	amount1In, _ := new(big.Float).Quo(big.NewFloat(0).SetInt(swap.Amount1In), new(big.Float).SetFloat64(math.Pow10(decimals1))).Float64()
	amount1Out, _ := new(big.Float).Quo(big.NewFloat(0).SetInt(swap.Amount1Out), new(big.Float).SetFloat64(math.Pow10(decimals1))).Float64()

	normalizedSwap = UniswapSwap{
		ID:         swap.Raw.TxHash.Hex(),
		Timestamp:  time.Now().Unix(),
		Pair:       pair,
		Amount0In:  amount0In,
		Amount0Out: amount0Out,
		Amount1In:  amount1In,
		Amount1Out: amount1Out,
	}
	return
}

func asset2UniAsset(asset models.Asset) UniswapToken {
	return UniswapToken{
		Address:  common.HexToAddress(asset.Address),
		Decimals: asset.Decimals,
		Symbol:   asset.Symbol,
		Name:     asset.Name,
	}
}

// getSwapData returns price, volume and sell/buy information of @swap
func getSwapData(swap UniswapSwap) (price float64, volume float64) {
	if swap.Amount0In == float64(0) {
		volume = swap.Amount0Out
		price = swap.Amount1In / swap.Amount0Out
		return
	}
	volume = -swap.Amount0In
	price = swap.Amount1Out / swap.Amount0In
	return
}
