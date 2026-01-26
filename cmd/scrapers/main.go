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
	exchanges = utils.Getenv("CEX_LIST", "")
	// Comma separated list of pools on an exchange.
	// Format should be as follows PancakeswapV3:0xac0fe1c4126e4a9b644adfc1303827e3bb5dddf3:i
	// where 0<=i<=2 determines the order of the returned swaps.
	// 0: original pool order
	// 1: reversed pool order
	// 2: both directions
	dexEnv = utils.Getenv("DEX_LIST", "")

	exchangePairs      []models.ExchangePair
	pools              []models.Pool
	branchMarketConfig string
)

func init() {
	cexLists := strings.Split(exchanges, ENV_SEPARATOR)
	branchMarketConfig = utils.Getenv("BRANCH_MARKET_CONFIG", "")
	var epErr error
	exchangePairs, epErr = models.ExchangePairsFromConfigFiles(cexLists, branchMarketConfig)
	if epErr != nil {
		log.Error("Read exchange pairs from files: ", epErr)
	}

	dexLists := strings.Split(dexEnv, ENV_SEPARATOR)
	var err error
	pools, err = models.PoolsFromConfigFiles(dexLists, branchMarketConfig)
	if err != nil {
		log.Error("Read pools from Config files: ", err)
	}

	if len(exchangePairs) == 0 && len(pools) == 0 {
		log.Fatal("no exchangepairs and no pools available.")
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
	metacontractNode := utils.Getenv("METACONTRACT_NODE", "")
	conn, connBackup, metacontractClient, privateKey, auth := utils.SetupOnchain(
		blockchainNode,
		backupNode,
		metacontractNode,
		privateKeyHex,
		chainID,
	)
	metacontractAddress := utils.Getenv("METACONTRACT_ADDRESS", "")
	metacontractPrecision, err := strconv.Atoi(utils.Getenv("METACONTRACT_PRECISION", "8"))
	if err != nil {
		log.Error("parse METACONTRACT_PRECISION: ", err)
		metacontractPrecision = 8
	}

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
	go processor.Processor(
		exchangePairs,
		pools,
		tradesblockChannel,
		filtersChannel,
		triggerChannel,
		failoverChannel,
		metacontractClient,
		metacontractAddress,
		metacontractPrecision,
		branchMarketConfig,
		&wg,
	)

	// This should be the final line of main (blocking call)
	onchain.OracleUpdateExecutor(auth, contract, contractBackup, conn, connBackup, chainID, filtersChannel)
}
