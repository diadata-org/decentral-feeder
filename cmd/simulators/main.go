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
	simulationprocessor "github.com/diadata-org/lumina-library/simulations/simulationProcessor"
	utils "github.com/diadata-org/lumina-library/utils"
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

// GetImageVersion returns the Docker image version from environment variable
func GetImageVersion() string {
	// Get version from IMAGE_TAG environment variable
	version := os.Getenv("IMAGE_TAG")

	if version == "" {
		version = "unknown" // fallback if not set
		log.Info("No version found, using 'unknown'")
	}
	return version
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
	tradesblockChannel := make(chan map[string]models.SimulatedTradesBlock)
	filtersChannel := make(chan []models.FilterPointPair)
	triggerChannel := make(chan time.Time)

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
			triggerChannel <- tick
		}
	}()

	// Run Processor and subsequent routines.
	go simulationprocessor.Processor(
		exchangePairs,
		tradesblockChannel,
		filtersChannel,
		triggerChannel,
		metacontractClient,
		metacontractAddress,
		metacontractPrecision,
		&wg,
	)

	// Update the oracle. Use backup node if it fails.
	onchain.OracleUpdateExecutor(auth, contract, contractBackup, conn, connBackup, chainID, filtersChannel)

}
