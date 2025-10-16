package main

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	diaOracleV2MultiupdateService "github.com/diadata-org/diadata/pkg/dia/scraper/blockchain-scrapers/blockchains/ethereum/diaOracleV2MultiupdateService"
	"github.com/diadata-org/lumina-library/metrics"
	models "github.com/diadata-org/lumina-library/models"
	"github.com/diadata-org/lumina-library/onchain"
	"github.com/diadata-org/lumina-library/processor"
	utils "github.com/diadata-org/lumina-library/utils"
	log "github.com/sirupsen/logrus"
)

const (
	// Separator for entries in the environment variables, i.e. Binance:BTC-USDT,KuCoin:BTC-USDT.
	ENV_SEPARATOR = ","
	// Separator for a pair on a given exchange, i.e. Binance:BTC-USDT.
	EXCHANGE_PAIR_SEPARATOR = ":"
)

var (

	// Comma separated list of exchangepairs. Pairs must be capitalized and symbols separated by hyphen.
	// It is the responsability of each exchange scraper to determine the correct format for the corresponding API calls.
	// Format should be as follows Binance:ETH-USDT,Binance:BTC-USDT
	exchanges = utils.Getenv("EXCHANGES", "")
	// Comma separated list of pools on an exchange.
	// Format should be as follows PancakeswapV3:0xac0fe1c4126e4a9b644adfc1303827e3bb5dddf3:i
	// where 0<=i<=2 determines the order of the returned swaps.
	// 0: original pool order
	// 1: reversed pool order
	// 2: both directions
	poolsEnv = utils.Getenv("POOLS", "")

	exchangePairs []models.ExchangePair
	pools         []models.Pool
)

func init() {
	exchangeLists := strings.Split(exchanges, ENV_SEPARATOR)
	var epErr error
	exchangePairs, epErr = models.ExchangePairsFromConfigFiles(exchangeLists)
	if epErr != nil {
		log.Fatal("Read exchange pairs from files: ", epErr)
	}
	var err error
	pools, err = models.PoolsFromEnv(poolsEnv, ENV_SEPARATOR, EXCHANGE_PAIR_SEPARATOR)
	if err != nil {
		log.Fatal("Read pools from ENV var: ", err)
	}

}

func main() {

	chainID, err := strconv.ParseInt(utils.Getenv("CHAIN_ID", "100640"), 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse chain ID: %v", err)
	}

	// Initialize env variables for on-chain setup.
	deployedContract := utils.Getenv("DEPLOYED_CONTRACT", "")
	privateKeyHex := utils.Getenv("PRIVATE_KEY", "")
	blockchainNode := utils.Getenv("BLOCKCHAIN_NODE", "")
	backupNode := utils.Getenv("BACKUP_NODE", "")
	conn, connBackup, privateKey, auth := utils.SetupOnchain(blockchainNode, backupNode, privateKeyHex, chainID)

	// Initialize env variables for metrics server.
	pushgatewayURL := os.Getenv("PUSHGATEWAY_URL")
	authUser := os.Getenv("PUSHGATEWAY_USER")
	authPassword := os.Getenv("PUSHGATEWAY_PASSWORD")
	enablePrometheusServer := utils.Getenv("ENABLE_METRICS_SERVER", "false")
	nodeOperatorName := utils.Getenv("NODE_OPERATOR_NAME", "")
	metricsPort := utils.Getenv("METRICS_PORT", "9090")
	imageVersion := os.Getenv("IMAGE_TAG")
	exchangePairString := models.MakeExchangePairString(exchangePairs)
	metrics.StartMetrics(
		conn,
		privateKey,
		deployedContract,
		pushgatewayURL,
		authUser,
		authPassword,
		enablePrometheusServer,
		nodeOperatorName,
		metricsPort,
		imageVersion,
		chainID,
		exchangePairString,
	)

	var contract *diaOracleV2MultiupdateService.DiaOracleV2MultiupdateService
	var contractBackup *diaOracleV2MultiupdateService.DiaOracleV2MultiupdateService
	err = onchain.DeployOrBindContract(deployedContract, conn, connBackup, auth, &contract, &contractBackup)
	if err != nil {
		log.Fatalf("Failed to Deploy or Bind primary and backup contract: %v", err)
	}

	// Create channels and set up blockchain connections
	wg := sync.WaitGroup{}
	tradesblockChannel := make(chan map[string]models.TradesBlock)
	filtersChannel := make(chan []models.FilterPointPair)
	triggerChannel := make(chan time.Time)
	failoverChannel := make(chan string)

	// Frequency for the trigger ticker initiating the computation of filter values.
	frequencySeconds, err := strconv.Atoi(utils.Getenv("FREQUENCY_SECONDS", "20"))
	if err != nil {
		log.Fatalf("Failed to parse frequencySeconds: %v", err)
	}

	// Use a ticker for triggering the processing. Could also be request based or other trigger types.
	triggerTick := time.NewTicker(time.Duration(frequencySeconds) * time.Second)
	go func() {
		for tick := range triggerTick.C {
			log.Debug("Trigger - tick at: ", tick)
			triggerChannel <- tick
		}
	}()

	// Run processor
	go processor.Processor(exchangePairs, pools, tradesblockChannel, filtersChannel, triggerChannel, failoverChannel, &wg)

	// This should be the final line of main (blocking call)
	onchain.OracleUpdateExecutor(auth, contract, contractBackup, conn, connBackup, chainID, filtersChannel)
}
