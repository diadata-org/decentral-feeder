package main

import (
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	diaOracleV2MultiupdateService "github.com/diadata-org/diadata/pkg/dia/scraper/blockchain-scrapers/blockchains/ethereum/diaOracleV2MultiupdateService"
	"github.com/diadata-org/lumina-library/metrics"
	models "github.com/diadata-org/lumina-library/models"
	"github.com/diadata-org/lumina-library/onchain"
	simulationprocessor "github.com/diadata-org/lumina-library/simulations/simulationProcessor"
	utils "github.com/diadata-org/lumina-library/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const (
	// Separator for entries in the environment variables,
	// i.e. UniswapSimulation:Blockchain:Address1Out-Address1In,UniswapSimulation:Blockchain:Address2Out-Address2In.
	ENV_SEPARATOR = ","
	// Separator for a pair's addresses.
	PAIR_SEPARATOR = "-"
	// Separator for a pair on a given exchange, i.e. UniswapSimulation:Blockchain:AddressOut-AddressIn.
	EXCHANGE_SEPARATOR = ":"
)

var (
	// Comma separated list of DEX pairs.
	// Format should be as follows: Exchange:Blockchain:AddressTokenOut-AddressTokenIn
	pairsEnv      = utils.Getenv("DEX_PAIRS", "")
	exchangePairs []models.ExchangePair
)

func init() {
	// Extract exchangePairs from the DEX_PAIRS environment variable.
	for _, p := range strings.Split(pairsEnv, ENV_SEPARATOR) {
		var exchangePair models.ExchangePair
		parts := strings.Split(p, EXCHANGE_SEPARATOR)
		if len(parts) < 3 {
			log.Warnf("Invalid DEX pair format: %s", p)
			continue
		}
		exchangePair.Exchange = strings.TrimSpace(parts[0])
		exchangePair.UnderlyingPair.QuoteToken.Blockchain = parts[1]
		addresses := strings.Split(parts[2], PAIR_SEPARATOR)
		if len(addresses) < 2 {
			log.Warnf("Invalid address format in DEX pair: %s", p)
			continue
		}
		exchangePair.UnderlyingPair.QuoteToken.Address = addresses[0]
		exchangePair.UnderlyingPair.BaseToken.Address = addresses[1]
		exchangePair.UnderlyingPair.BaseToken.Blockchain = parts[1]
		exchangePairs = append(exchangePairs, exchangePair)
		log.Infof(
			"exchange -- blockchain -- address0 -- address1: %s -- %s -- %s -- %s",
			exchangePair.Exchange,
			parts[1],
			addresses[0],
			addresses[1],
		)
	}
}

func main() {
	//get hostname of the container so that we can display it in monitoring dashboards
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to get hostname: %v", err)
	}

	// Get image version from environment variable
	imageVersion := utils.Getenv("IMAGE_VERSION", "unknown")

	// Change variable names for consistency
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

	// Get deployed contract and set the metric
	deployedContract := utils.Getenv("DEPLOYED_CONTRACT", "")
	// Set the static contract label for Prometheus monitoring
	m.Contract.WithLabelValues(deployedContract).Set(1)

	// Add this code to expose exchangePairs for monitoring
	if len(exchangePairs) > 0 {
		for _, pair := range exchangePairs {
			pairLabel := fmt.Sprintf("%s:%s:%s-%s",
				pair.Exchange,
				pair.UnderlyingPair.QuoteToken.Blockchain,
				pair.UnderlyingPair.QuoteToken.Address,
				pair.UnderlyingPair.BaseToken.Address)

			m.ExchangePairs.WithLabelValues(pairLabel).Set(1)
			log.Infof("Added exchange pair to metrics: %s", pairLabel)
		}
	} else {
		log.Info("No exchange pairs to monitor; DEX_PAIRS environment variable is empty or improperly formatted")
	}

	wg := sync.WaitGroup{}
	tradesblockChannel := make(chan map[string]models.SimulatedTradesBlock)
	filtersChannel := make(chan []models.FilterPointPair)
	triggerChannel := make(chan time.Time)

	// Feeder mechanics
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

	// Frequency for the trigger ticker initiating the computation of filter values.
	frequencySeconds, err := strconv.Atoi(utils.Getenv("FREQUENCY_SECONDS", "120"))
	if err != nil {
		log.Fatalf("Failed to parse frequencySeconds: %v", err)
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

	// Use a ticker for triggering the processing.
	// This is for testing purposes for now. Could also be request based or other trigger types.
	triggerTick := time.NewTicker(time.Duration(frequencySeconds) * time.Second)
	go func() {
		for tick := range triggerTick.C {
			// log.Info("Trigger - tick at: ", tick)
			triggerChannel <- tick
		}
	}()

	// Run Processor and subsequent routines.
	go simulationprocessor.Processor(exchangePairs, tradesblockChannel, filtersChannel, triggerChannel, &wg)

	// Periodically update and push metrics to pushgateway
	if pushgatewayEnabled {
		go metrics.PushMetricsToPushgateway(m, startTime, conn, privateKey, deployedContract)
	}

	// Outlook/Alternative: The triggerChannel can also be filled by the oracle updater by any other mechanism.
	onchain.OracleUpdateExecutor(auth, contract, conn, chainID, filtersChannel)

}
