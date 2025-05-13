package main

import (
	"math/big"
	"os"
	"os/user"
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
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
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

	// Comma separated list of exchangepairs. Pairs must be capitalized and symbols separated by hyphen.
	// It is the responsability of each exchange scraper to determine the correct format for the corresponding API calls.
	// Format should be as follows Binance:ETH-USDT,Binance:BTC-USDT
	exchangePairsEnv = utils.Getenv("EXCHANGEPAIRS", "Crypto.com:BTC-USDT,Crypto.com:BTC-USD")

	exchangePairs []models.ExchangePair
	pools         []models.Pool
)

func init() {
	exchangePairs = models.ExchangePairsFromEnv(exchangePairsEnv, ENV_SEPARATOR, EXCHANGE_PAIR_SEPARATOR, PAIR_TICKER_SEPARATOR, getPath2Config())
}

// GetImageVersion returns the Docker image version from environment variable
func GetImageVersion() string {
	// Get version from IMAGE_TAG environment variable
	version := os.Getenv("IMAGE_TAG")
	log.Infof("IMAGE_TAG: %s", version)

	if version == "" {
		version = "unknown" // fallback if not set
		log.Info("No version found, using 'unknown'")
	}

	log.Infof("Final image version: %s", version)
	return version
}

func main() {
	// get hostname of the container so that we can display it in monitoring dashboards
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to get hostname: %v", err)
	}

	// Check if metrics pushing to Pushgateway is enabled
	pushgatewayURL := os.Getenv("PUSHGATEWAY_URL")
	authUser := os.Getenv("PUSHGATEWAY_USER")
	authPassword := os.Getenv("PUSHGATEWAY_PASSWORD")
	pushgatewayEnabled := pushgatewayURL != "" && authUser != "" && authPassword != ""

	// Check if Prometheus HTTP server is enabled
	enablePrometheusServer := utils.Getenv("ENABLE_METRICS_SERVER", "false")
	prometheusServerEnabled := strings.ToLower(enablePrometheusServer) == "true"

	// Get the node operator ID from the environment variable (optional)
	nodeOperatorName := utils.Getenv("NODE_OPERATOR_NAME", "")

	// Create the job name for metrics (used for both modes)
	jobName := metrics.MakeJobName(hostname, nodeOperatorName)

	// Get chain ID for metrics
	chainIDStr := utils.Getenv("CHAIN_ID", "1050")
	chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse chain ID: %v", err)
	}

	// Get image version using our local function
	imageVersion := GetImageVersion()
	log.Infof("Image version: %s", imageVersion)

	// Set default pushgateway URL if enabled
	if pushgatewayEnabled {
		if pushgatewayURL == "" {
			pushgatewayURL = "https://pushgateway-auth.diadata.org"
		}
		log.Info("Metrics pushing enabled. Pushing to: ", pushgatewayURL)
	} else {
		log.Info("Metrics pushing to Pushgateway disabled")
	}

	// Create metrics object
	m := metrics.NewMetrics(
		prometheus.NewRegistry(),
		pushgatewayURL,
		jobName,
		authUser,
		authPassword,
		chainID,
		imageVersion,
	)

	// Start Prometheus HTTP server if enabled
	if prometheusServerEnabled {
		metricsPort := utils.Getenv("METRICS_PORT", "9090")
		go metrics.StartPrometheusServer(m, metricsPort)
		log.Info("Prometheus HTTP server enabled on port:", metricsPort)
	} else {
		log.Info("Prometheus HTTP server disabled")
	}

	// Record start time for uptime calculation
	startTime := time.Now()

	// Initialize feeder env variables
	deployedContract := utils.Getenv("DEPLOYED_CONTRACT", "")
	privateKeyHex := utils.Getenv("PRIVATE_KEY", "")
	blockchainNode := utils.Getenv("BLOCKCHAIN_NODE", "https://rpc.diadata.org")
	backupNode := utils.Getenv("BACKUP_NODE", "https://rpc.diadata.org")
	conn, err := ethclient.Dial(blockchainNode)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	connBackup, err := ethclient.Dial(backupNode)
	if err != nil {
		log.Fatalf("Failed to connect to the backup Ethereum client: %v", err)
	}

	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")

	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(chainID))
	if err != nil {
		log.Fatalf("Failed to create authorized transactor: %v", err)
	}

	var contract, contractBackup *diaOracleV2MultiupdateService.DiaOracleV2MultiupdateService
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

	// Use a ticker for triggering the processing.
	// This is for testing purposes for now. Could also be request based or other trigger types.
	triggerTick := time.NewTicker(time.Duration(frequencySeconds) * time.Second)
	go func() {
		for tick := range triggerTick.C {
			// log.Info("Trigger - tick at: ", tick)
			triggerChannel <- tick
		}
	}()

	// Run processor
	go processor.Processor(exchangePairs, pools, tradesblockChannel, filtersChannel, triggerChannel, failoverChannel, &wg)

	// Move metrics setup here, right before the blocking call
	// Only setup metrics collection if metrics are enabled and metrics object exists
	if pushgatewayEnabled && m != nil {
		// Set the static contract label for Prometheus monitoring
		m.Contract.WithLabelValues(deployedContract).Set(1)

		exchangePairsList := strings.Split(exchangePairsEnv, ",")
		for _, pair := range exchangePairsList {
			pair = strings.TrimSpace(pair) // Clean whitespace
			if pair != "" {
				m.ExchangePairs.WithLabelValues(pair).Set(1)
			}
		}

		// Push metrics to Pushgateway if enabled
		go metrics.PushMetricsToPushgateway(m, startTime, conn, privateKey, deployedContract)
	}

	// This should be the final line of main (blocking call)
	onchain.OracleUpdateExecutor(auth, contract, conn, chainID, filtersChannel)
}

func getPath2Config() string {
	usr, _ := user.Current()
	dir := usr.HomeDir
	if dir == "/root" || dir == "/home" {
		return "/config/symbolIdentification/"
	}
	return os.Getenv("GOPATH") + "/src/github.com/diadata-org/decentral-feeder/config/symbolIdentification/"
}
