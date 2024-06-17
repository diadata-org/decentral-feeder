package main

import (
	"context"
	"flag"
	"io/ioutil"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	filters "github.com/diadata-org/decentral-feeder/pkg/filters"
	metafilters "github.com/diadata-org/decentral-feeder/pkg/metafilters"
	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/onchain"
	scrapers "github.com/diadata-org/decentral-feeder/pkg/scraper"
	utils "github.com/diadata-org/decentral-feeder/pkg/utils"
	diaOracleV2MultiupdateService "github.com/diadata-org/diadata/pkg/dia/scraper/blockchain-scrapers/blockchains/ethereum/diaOracleV2MultiupdateService"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

const (
	// Separator for entries in the environment variables, i.e. Binance:BTC-USDT,KuCoin:BTC-USDT.
	ENV_SEPARATOR = ","
	// Separator for a pair ticker's assets, i.e. BTC-USDT.
	PAIR_TICKER_SEPARATOR = "-"
	// Separator for a pair on a given exchange, i.e. Binance:BTC-USDT.
	EXCHANGE_PAIR_SEPARATOR = ":"
)

var (
	env = flag.Bool("env", true, "Get pairs and pools from env variable if set to true. Otherwise, pairs are read from config file.")

	// Comma separated list of exchanges. Only used in case pairs are read from config files.
	exchanges = utils.Getenv("EXCHANGES", "UniswapV2,Binance")
	// Comma separated list of exchangepairs. Pairs must be capitalized and symbols separated by hyphen.
	// It is the responsability of each exchange scraper to determine the correct format for the corresponding API calls.
	// Format should be as follows Binance:ETH-USDT,Binance:BTC-USDT
	exchangePairsEnv = utils.Getenv("EXCHANGEPAIRS", "")
	// Comma separated list of pools.
	// The binary digit in the third position controls the order of the trades in the pool:
	// TO DO: For 0 the original order is taken into consideration, while for 1 the order of all trades in the pool is reversed.
	// Format should be as follows: UniswapV2:0x0d4a11d5EEaaC28EC3F61d100daF4d40471f1852:0,UniswapV2:0xc5be99A02C6857f9Eac67BbCE58DF5572498F40c:0
	poolsEnv = utils.Getenv("POOLS", "")

	exchangePairs []models.ExchangePair
	pools         []models.Pool
	// For processing, all filters with timestamp older than time.Now()-toleranceSeconds are discarded.
	toleranceSeconds int64
)

func init() {
	flag.Parse()
	var err error
	toleranceSeconds, err = strconv.ParseInt(utils.Getenv("TOLERANCE_SECONDS", "20"), 10, 64)
	if err != nil {
		log.Error("Parse TOLERANCE_SECONDS environment variable: ", err)
	}

	if *env {
		exchangePairs = models.ExchangePairsFromEnv(exchangePairsEnv, ENV_SEPARATOR, EXCHANGE_PAIR_SEPARATOR, PAIR_TICKER_SEPARATOR)

		// Extract pools from env var.
		if poolsEnv != "" {
			for _, p := range strings.Split(poolsEnv, ENV_SEPARATOR) {
				var pool models.Pool
				pool.Exchange = scrapers.Exchanges[strings.Split(p, EXCHANGE_PAIR_SEPARATOR)[0]]
				pool.Address = strings.Split(p, EXCHANGE_PAIR_SEPARATOR)[1]
				pool.Blockchain = models.Blockchain{Name: pool.Exchange.Blockchain}
				pools = append(pools, pool)
			}
		}

	} else {
		// Collect all CEX pairs and DEX pools for subsequent scraping.
		// CEX pairs are mapped onto the underlying assets as well.
		// It's the responsability of the corresponding DEX scraper to fetch pool asset information.
		for _, exchange := range strings.Split(exchanges, ",") {
			if _, ok := scrapers.Exchanges[exchange]; !ok {
				log.Fatalf("Scraper for %s not available.", exchange)
			}
			if scrapers.Exchanges[exchange].Centralized {
				ep, err := models.GetPairsFromConfig(exchange)
				if err != nil {
					log.Fatalf("GetPairsFromConfig for %s: %v", exchange, err)
				}
				exchangePairs = append(exchangePairs, ep...)
				continue
			}

			p, err := models.GetPoolsFromConfig(exchange)
			if err != nil {
				log.Fatalf("GetPoolsFromConfig for %s: %v", exchange, err)
			}
			pools = append(pools, p...)
		}
	}

}

func main() {

	wg := sync.WaitGroup{}
	tradesblockChannel := make(chan map[string]models.TradesBlock)
	filtersChannel := make(chan []models.FilterPointExtended)
	triggerChannel := make(chan time.Time)

	// ----------------------------
	// Feeder mechanics
	// ----------------------------
	key := utils.Getenv("PRIVATE_KEY", "")
	key_password := utils.Getenv("PRIVATE_KEY_PASSWORD", "")
	deployedContract := utils.Getenv("DEPLOYED_CONTRACT", "")
	blockchainNode := utils.Getenv("BLOCKCHAIN_NODE", "")
	backupNode := utils.Getenv("BACKUP_NODE", "")

	conn, err := ethclient.Dial(blockchainNode)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	connBackup, err := ethclient.Dial(backupNode)
	if err != nil {
		log.Fatalf("Failed to connect to the backup Ethereum client: %v", err)
	}
	chainId, err := strconv.ParseInt(utils.Getenv("CHAIN_ID", ""), 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse chainId: %v", err)
	}

	// frequency for the trigger ticker initiating the computation of filter values.
	frequencySeconds, err := strconv.Atoi(utils.Getenv("FREQUENCY_SECONDS", "60"))
	if err != nil {
		log.Fatalf("Failed to parse frequencySeconds: %v", err)
	}

	auth, err := bind.NewTransactorWithChainID(strings.NewReader(key), key_password, big.NewInt(chainId))
	if err != nil {
		log.Fatalf("Failed to create authorized transactor: %v", err)
	}

	var contract, contractBackup *diaOracleV2MultiupdateService.DiaOracleV2MultiupdateService
	err = onchain.DeployOrBindContract(deployedContract, conn, connBackup, auth, &contract, &contractBackup)
	if err != nil {
		log.Fatalf("Failed to Deploy or Bind primary and backup contract: %v", err)
	}

	// Use a ticker for triggering the processing.
	// This is for testing purposes for now. Could also be request based or other trigger types.
	triggerTick := time.NewTicker(time.Duration(frequencySeconds) * time.Second)
	go func() {
		for tick := range triggerTick.C {
			log.Warn("tick at: ", tick)
			triggerChannel <- tick
		}
	}()

	// Run Processor and subsequent routines.
	go Processor(exchangePairs, pools, tradesblockChannel, filtersChannel, triggerChannel, &wg)

	// Outlook/Alternative: The triggerChannel can also be filled by the oracle updater by any other mechanism.
	oracleUpdateExecutor(auth, contract, conn, chainId, filtersChannel)
}

func oracleUpdateExecutor(
	// publishedPrices map[string]float64,
	// newPrices map[string]float64,
	// deviationPermille int,
	auth *bind.TransactOpts,
	contract *diaOracleV2MultiupdateService.DiaOracleV2MultiupdateService,
	conn *ethclient.Client,
	chainId int64,
	// compatibilityMode bool,
	filtersChannel <-chan []models.FilterPointExtended,
) {

	for filterPoints := range filtersChannel {
		timestamp := time.Now().Unix()
		var keys []string
		var values []int64
		for _, fp := range filterPoints {
			log.Infof(
				"filterPoint received at %v: %v -- %v -- %v -- %v",
				time.Unix(timestamp, 0),
				fp.Source,
				fp.Pair.QuoteToken.Symbol+"-"+fp.Pair.BaseToken.Symbol,
				fp.Value,
				fp.Time,
			)
			keys = append(keys, fp.Pair.QuoteToken.Symbol+"/USD")
			values = append(values, int64(fp.Value*100000000))
		}
		err := updateOracleMultiValues(conn, contract, auth, chainId, keys, values, timestamp)
		if err != nil {
			log.Printf("Failed to update Oracle: %v", err)
			return
		}
	}

}

// Processor handles blocks from @tradesblockChannel.
// More precisley, it does so in a 2 step procedure:
// 1. Aggregate trades for each (atomic) block.
// 2. Aggregate filter values obtained in step 1.
func Processor(
	exchangePairs []models.ExchangePair,
	pools []models.Pool,
	tradesblockChannel chan map[string]models.TradesBlock,
	filtersChannel chan []models.FilterPointExtended,
	triggerChannel chan time.Time,
	wg *sync.WaitGroup,
) {

	log.Info("Start Processor......")
	// Collector starts collecting trades in the background and sends atomic tradesblocks to @tradesblockChannel.
	go Collector(exchangePairs, pools, tradesblockChannel, triggerChannel, wg)

	// As soon as the trigger channel receives input a processing step is initiated.
	for tradesblocks := range tradesblockChannel {

		var filterPoints []models.FilterPointExtended

		for exchangepairIdentifier, tb := range tradesblocks {

			log.Info("length tradesblock: ", len(tb.Trades))

			// TO DO: Set flag for trades' filter switch. For instance Median, Average, Minimum, etc.
			atomicFilterValue, timestamp, err := filters.LastPrice(tb.Trades, true)
			if err != nil {
				log.Error("GetLastPrice: ", err)
			}

			// Identify Pair from tradesblock
			// TO DO: There should be a better way. Maybe add as a field to tradesblock?
			// Alternatively we could use a simple FilterPoint. As of now, the base asset is not needed in subsequent computations.
			var pair models.Pair
			if len(tb.Trades) > 0 {
				pair = models.Pair{QuoteToken: tb.Trades[0].QuoteToken, BaseToken: tb.Trades[0].BaseToken}
			}

			filterPoint := models.FilterPointExtended{
				Pair:   pair,
				Value:  atomicFilterValue,
				Time:   timestamp,
				Source: strings.Split(exchangepairIdentifier, "-")[0],
			}
			filterPoints = append(filterPoints, filterPoint)

		}

		var removedFilterPoints int
		filterPoints, removedFilterPoints = models.RemoveOldFilters(filterPoints, toleranceSeconds, time.Now())
		log.Warnf("Removed %v old filter points.", removedFilterPoints)

		// TO DO: Set flag for metafilter switch. For instance Median, Average, Minimum, etc.
		filterPointsMedianized := metafilters.Median(filterPoints)

		filtersChannel <- filterPointsMedianized
	}

}

// Collector starts scrapers for all exchanges given by @exchangePairs.
// Outlook: Collector starts a dedicated pod for each scraper.
func Collector(
	exchangePairs []models.ExchangePair,
	pools []models.Pool,
	tradesblockChannel chan map[string]models.TradesBlock,
	triggerChannel chan time.Time,
	wg *sync.WaitGroup,
) {

	// exchangepairMap maps a centralized exchange onto the given pairs.
	exchangepairMap := models.MakeExchangepairMap(exchangePairs)
	log.Info("exchangepairMap: ", exchangepairMap)
	// poolMap maps a decentralized exchange onto the given pools.
	poolMap := models.MakePoolMap(pools)
	log.Info("poolMap: ", poolMap)

	// Start all needed scrapers.
	// @tradesChannelIn collects trades from the started scrapers.
	tradesChannelIn := make(chan models.Trade)
	for exchange := range exchangepairMap {
		wg.Add(1)
		go scrapers.RunScraper(exchange, exchangepairMap[exchange], []models.Pool{}, tradesChannelIn, wg)
	}
	for exchange := range poolMap {
		wg.Add(1)
		go scrapers.RunScraper(exchange, []models.ExchangePair{}, poolMap[exchange], tradesChannelIn, wg)
	}

	// tradesblockMap maps an exchangpair identifier onto a TradesBlock.
	// This also means that each value consists of trades of only one exchangepair.
	tradesblockMap := make(map[string]models.TradesBlock)

	go func() {
		for {
			select {
			case trade := <-tradesChannelIn:

				// Determine exchangepair and the corresponding identifier in order to assign the tradesBlockMap.
				exchangepair := models.Pair{QuoteToken: trade.QuoteToken, BaseToken: trade.BaseToken}
				exchangepairIdentifier := exchangepair.ExchangePairIdentifier(trade.Exchange.Name)

				if _, ok := tradesblockMap[exchangepairIdentifier]; !ok {
					tradesblockMap[exchangepairIdentifier] = models.TradesBlock{
						Trades: []models.Trade{trade},
					}
				} else {
					tradesblock := tradesblockMap[exchangepairIdentifier]
					tradesblock.Trades = append(tradesblock.Trades, trade)
					tradesblockMap[exchangepairIdentifier] = tradesblock
				}

			case timestamp := <-triggerChannel:

				log.Info("triggered at : ", timestamp)
				tradesblockChannel <- tradesblockMap
				log.Info("number of tradesblocks: ", len(tradesblockMap))

				// Make a new tradesblockMap for the next trigger period.
				tradesblockMap = make(map[string]models.TradesBlock)

			}
		}
	}()

	defer wg.Wait()
}

func updateOracleMultiValues(
	client *ethclient.Client,
	contract *diaOracleV2MultiupdateService.DiaOracleV2MultiupdateService,
	auth *bind.TransactOpts,
	chainId int64,
	keys []string,
	values []int64,
	timestamp int64) error {

	var cValues []*big.Int
	var gasPrice *big.Int
	var err error

	// Get proper gas price depending on chainId
	switch chainId {
	/*case 288: //Boba
	gasPrice = big.NewInt(1000000000)*/
	case 592: //Astar
		response, err := http.Get("https://gas.astar.network/api/gasnow?network=astar")
		if err != nil {
			return err
		}

		defer response.Body.Close()
		if 200 != response.StatusCode {
			return err
		}
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return err
		}

		gasSuggestion := gjson.Get(string(contents), "data.fast")
		gasPrice = big.NewInt(gasSuggestion.Int())
	default:
		// Get gas price suggestion
		gasPrice, err = client.SuggestGasPrice(context.Background())
		if err != nil {
			log.Print(err)
			return err
		}

		// Get 110% of the gas price
		fGas := new(big.Float).SetInt(gasPrice)
		fGas.Mul(fGas, big.NewFloat(1.1))
		gasPrice, _ = fGas.Int(nil)
	}

	for _, value := range values {
		// Create compressed argument with values/timestamps
		cValue := big.NewInt(value)
		cValue = cValue.Lsh(cValue, 128)
		cValue = cValue.Add(cValue, big.NewInt(timestamp))
		cValues = append(cValues, cValue)
	}

	// Write values to smart contract
	tx, err := contract.SetMultipleValues(&bind.TransactOpts{
		From:     auth.From,
		Signer:   auth.Signer,
		GasPrice: gasPrice,
	}, keys, cValues)
	// check if tx is sendable then fgo backup
	if err != nil {
		// backup in here
		return err
	}

	log.Printf("Gas price: %d\n", tx.GasPrice())
	log.Printf("Data: %x\n", tx.Data())
	log.Printf("Nonce: %d\n", tx.Nonce())
	log.Printf("Tx To: %s\n", tx.To().String())
	log.Printf("Tx Hash: 0x%x\n", tx.Hash())
	return nil
}
