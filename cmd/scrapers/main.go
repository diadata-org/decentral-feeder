package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	diaOracleV3 "github.com/diadata-org/lumina-library/contracts/lumina/diaoraclev3"
	"github.com/diadata-org/lumina-library/metrics"
	models "github.com/diadata-org/lumina-library/models"
	"github.com/diadata-org/lumina-library/onchain"
	"github.com/diadata-org/lumina-library/processor"
	utils "github.com/diadata-org/lumina-library/utils"
	"github.com/ethereum/go-ethereum/ethclient"
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
	customFeeds        []models.CustomFeed
	branchMarketConfig string
	branchFeedConfig   string
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

	branchFeedConfig = utils.Getenv("BRANCH_FEED_CONFIG", "")
	customFeedsAux, err := models.FeedsFromConfigFile(branchFeedConfig)
	if err != nil {
		log.Fatal("FeedsFromConfigFile: ", err)
	}
	log.Info("feeds: ", customFeedsAux)

	// Check if all custom feeds can be realized.
	// Don't include feed if not all requested markets are available.(?)
	customFeeds = processor.FilterAdmissibleCustomFeeds(customFeedsAux, exchangePairs)
	log.Info("customFeeds: ", customFeeds)

}

func main() {

	chainID, err := strconv.ParseInt(utils.Getenv("CHAIN_ID", "10050"), 10, 64)
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
	metacontractPrecision, err := strconv.Atoi(utils.Getenv("METACONTRACT_PRECISION", "18"))
	if err != nil {
		log.Error("parse METACONTRACT_PRECISION: ", err)
		metacontractPrecision = 18
	}

	decimalPrecision, err := strconv.Atoi(utils.Getenv("DECIMAL_PRECISION", "18"))
	if err != nil {
		log.Error("parse DECIMAL_PRECISION: ", err)
		decimalPrecision = 18
	}

	batchSizeOracleUpdate, err := strconv.Atoi(utils.Getenv("BATCH_SIZE_ORACLE_UPDATE", "20"))
	if err != nil {
		log.Error("parse BATCH_SIZE_ORACLE_UPDATE: ", err)
		batchSizeOracleUpdate = 20
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
	// Metrics follow the active RPC once the switcher exists (set further below).
	var activeSwitcher atomic.Pointer[onchain.RPCSwitcher]
	metrics.StartMetricsWithClientProvider(
		func() *ethclient.Client {
			if sw := activeSwitcher.Load(); sw != nil {
				return sw.Endpoints().Conn
			}
			return conn
		},
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

	var contract *diaOracleV3.DIAOracleV3
	var contractBackup *diaOracleV3.DIAOracleV3
	err = onchain.DeployOrBindContract(deployedContract, conn, connBackup, auth, &contract, &contractBackup, uint8(decimalPrecision))
	if err != nil {
		log.Fatalf("Failed to Deploy or Bind primary and backup contract: %v", err)
	}

	// Create channels and set up blockchain connections
	wg := sync.WaitGroup{}
	tradesblockChannel := make(chan map[string]models.TradesBlock)
	filtersChannel := make(chan []models.FilterPoint)
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
		customFeeds,
		tradesblockChannel,
		filtersChannel,
		triggerChannel,
		failoverChannel,
		metacontractClient,
		metacontractAddress,
		metacontractPrecision,
		branchMarketConfig,
		branchFeedConfig,
		&wg,
	)

	// Optional: switch the RPC between Conduit mainnet and the Anvil backup based on the Liveness API.
	livenessAPIURL := utils.Getenv("LIVENESS_API_URL", "")
	anvilNode := utils.Getenv("ANVIL_NODE", "")
	if livenessAPIURL != "" && anvilNode != "" {
		switcher := onchain.NewRPCSwitcher(
			livenessAPIURL,
			blockchainNode,
			backupNode,
			anvilNode,
			deployedContract,
			chainID,
			privateKey,
			onchain.Endpoints{Conn: conn, ConnBackup: connBackup, Contract: contract, ContractBackup: contractBackup},
		)
		activeSwitcher.Store(switcher)
		go switcher.Run(context.Background())

		// This should be the final line of main (blocking call)
		onchain.OracleUpdateExecutorSwitchable(auth, switcher, chainID, decimalPrecision, batchSizeOracleUpdate, filtersChannel)
		return
	}
	if livenessAPIURL != "" || anvilNode != "" {
		log.Warn("LIVENESS_API_URL and ANVIL_NODE must both be set to enable RPC switching; running without it.")
	}

	// This should be the final line of main (blocking call)
	onchain.OracleUpdateExecutor(auth, contract, contractBackup, conn, connBackup, chainID, decimalPrecision, batchSizeOracleUpdate, filtersChannel)
}
