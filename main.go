package main

import (
	"context"
	"io/ioutil"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/diadata-org/decentral-feeder/pkg/onchain"
	"github.com/diadata-org/decentral-feeder/pkg/processing"
	scrapers "github.com/diadata-org/decentral-feeder/pkg/scraper"
	utils "github.com/diadata-org/decentral-feeder/pkg/utils"
	diaOracleV2MultiupdateService "github.com/diadata-org/diadata/pkg/dia/scraper/blockchain-scrapers/blockchains/ethereum/diaOracleV2MultiupdateService"
	models "github.com/diadata-org/diaprotocol/pkg/models"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

func main() {
	exchangePairs := []models.ExchangePair{
		{
			Exchange:    scrapers.BINANCE_EXCHANGE,
			Symbol:      "BTC",
			ForeignName: "BTC-USDT",
			UnderlyingPair: models.Pair{
				QuoteToken: models.Asset{
					Symbol:     "BTC",
					Blockchain: "Bitcoin",
					Address:    "0x0000000000000000000000000000000000000000",
				},
				BaseToken: models.Asset{
					Symbol:     "USDT",
					Blockchain: "Ethereum",
					Address:    "0xdAC17F958D2ee523a2206206994597C13D831ec7",
				},
			},
		},
		{
			Exchange:    scrapers.BINANCE_EXCHANGE,
			Symbol:      "ETH",
			ForeignName: "ETH-USDT",
			UnderlyingPair: models.Pair{
				QuoteToken: models.Asset{
					Symbol:     "ETH",
					Blockchain: "Ethereum",
					Address:    "0x0000000000000000000000000000000000000000",
				},
				BaseToken: models.Asset{
					Symbol:     "USDT",
					Blockchain: "Ethereum",
					Address:    "0xdAC17F958D2ee523a2206206994597C13D831ec7",
				},
			},
		},
	}

	wg := sync.WaitGroup{}
	tradesblockChannel := make(chan map[string]models.TradesBlock)
	filtersChannel := make(chan []models.FilterPointExtended)
	triggerChannel := make(chan time.Time)

	// ----------------------------
	// Feeder mechanics
	// ----------------------------
	key := utils.Getenv("PRIVATE_KEY", "")
	key_password := utils.Getenv("PRIVATE_KEY_PASSWORD", "")
	deployedContract := utils.Getenv("DEPLOYED_CONTRACT", "0x11A29B3cC367910352b3edaF6FDAf044Ba4D8ECc")
	blockchainNode := utils.Getenv("BLOCKCHAIN_NODE", "https://rpc2.sepolia.org")
	backupNode := utils.Getenv("BACKUP_NODE", "https://rpc.sepolia.ethpandaops.io")

	conn, err := ethclient.Dial(blockchainNode)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	connBackup, err := ethclient.Dial(backupNode)
	if err != nil {
		log.Fatalf("Failed to connect to the backup Ethereum client: %v", err)
	}
	chainId, err := strconv.ParseInt(utils.Getenv("CHAIN_ID", "11155111"), 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse chainId: %v", err)
	}
	frequencySeconds, err := strconv.Atoi(utils.Getenv("FREQUENCY_SECONDS", "20"))
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
	// This is for testing purposes for now.
	triggerTick := time.NewTicker(time.Duration(frequencySeconds) * time.Second)
	go func() {
		for tick := range triggerTick.C {
			log.Warn("tick at: ", tick)
			triggerChannel <- tick
		}
	}()

	// Run Processor and subsequent routines.
	go Processor(exchangePairs, tradesblockChannel, filtersChannel, triggerChannel, &wg)

	// Outlook/Alternative: The triggerChannel can also be filled by the oracle updater by any other mechanism.
	// oracleUpdateExecutor(auth, contract, conn, chainId, filterChannel)
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

	timestamp := time.Now().Unix()
	for filterPoints := range filtersChannel {

		var keys []string
		var values []int64
		for _, fp := range filterPoints {
			log.Infof("%v -- filterPoint received: %v -- %v", time.Unix(timestamp, 0), fp.Value, fp.Time)
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

func Processor(exchangePairs []models.ExchangePair,
	tradesblockChannel chan map[string]models.TradesBlock,
	filtersChannel chan []models.FilterPointExtended,
	triggerChannel chan time.Time,
	wg *sync.WaitGroup,
) {

	log.Info("Start Processor......")
	// Collector starts collecting trades in the background.
	go Collector(exchangePairs, tradesblockChannel, triggerChannel, wg)

	// As soon as the trigger channel receives input a processing step is initiated.
	for tradesblocks := range tradesblockChannel {

		var filterPoints []models.FilterPointExtended

		for exchangepairIdentifier, tb := range tradesblocks {
			log.Info("length tradesblock: ", len(tb.Trades))
			latestPrice, timestamp, err := processing.LastPrice(tb.Trades, true)
			if err != nil {
				log.Error("GetLastPrice: ", err)
			}

			// Identify Pair from tradesblock (there should be a better way)
			var pair models.Pair
			if len(tb.Trades) > 0 {
				pair = models.Pair{QuoteToken: tb.Trades[0].QuoteToken, BaseToken: tb.Trades[0].BaseToken}
			}

			filterPoint := models.FilterPointExtended{
				Pair:   pair,
				Value:  latestPrice,
				Time:   timestamp,
				Source: strings.Split(exchangepairIdentifier, "-")[0],
			}
			filterPoints = append(filterPoints, filterPoint)
		}

		filtersChannel <- filterPoints
	}

}

// Collector starts a scraper for given @exchanges
func Collector(
	exchangePairs []models.ExchangePair,
	tradesblockChannel chan map[string]models.TradesBlock,
	triggerChannel chan time.Time,
	wg *sync.WaitGroup,
) {

	exchangepairMap := utils.MakeExchangepairMap(exchangePairs)
	tradesChannelIn := make(chan models.Trade)
	for exchange := range exchangepairMap {
		wg.Add(1)
		go scrapers.RunScraper(exchange, exchangepairMap[exchange], tradesChannelIn, wg)
	}

	// tradesblockMap maps an exchangpair identifier onto a TradesBlock.
	// This also means that each value consists of trades of only one exchangepair.
	tradesblockMap := make(map[string]models.TradesBlock)

	go func() {
		for {
			select {
			case trade := <-tradesChannelIn:
				exchangepair := models.Pair{QuoteToken: trade.QuoteToken, BaseToken: trade.BaseToken}
				exchangepairIdentifier := exchangepair.PairExchangeIdentifier(trade.Exchange.Name)
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
