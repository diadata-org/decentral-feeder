package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/onchain"
	simulationprocessor "github.com/diadata-org/decentral-feeder/pkg/simulations/simulationProcessor"
	utils "github.com/diadata-org/decentral-feeder/pkg/utils"
	diaOracleV2MultiupdateService "github.com/diadata-org/diadata/pkg/dia/scraper/blockchain-scrapers/blockchains/ethereum/diaOracleV2MultiupdateService"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/shirou/gopsutil/cpu"
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

type metrics struct {
	uptime         prometheus.Gauge
	cpuUsage       prometheus.Gauge
	memoryUsage    prometheus.Gauge
	contract       *prometheus.GaugeVec
	exchangePairs  *prometheus.GaugeVec
	gasBalance     prometheus.Gauge
	lastUpdateTime prometheus.Gauge
	pushGatewayURL string
	jobName        string
	authUser       string
	authPassword   string
}

func NewMetrics(reg prometheus.Registerer, pushGatewayURL, jobName, authUser, authPassword string) *metrics {
	m := &metrics{
		uptime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "feeder",
			Name:      "uptime_hours",
			Help:      "Feeder Uptime in hours.",
		}),
		cpuUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "feeder",
			Name:      "cpu_usage_percent",
			Help:      "Feeder CPU usage in percent.",
		}),
		memoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "feeder",
			Name:      "memory_usage_megabytes",
			Help:      "Feeder Memory usage in megabytes.",
		}),
		contract: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "feeder",
				Name:      "contract_info",
				Help:      "Feeder contract information.",
			},
			[]string{"contract"}, // Label to store the contract address
		),
		exchangePairs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "feeder",
				Name:      "exchange_pairs",
				Help:      "List of exchange pairs to be pushed as labels for each Feeder.",
			},
			[]string{"exchange_pair"}, // Label to store each exchange pair
		),
		gasBalance: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "feeder",
			Name:      "gas_balance",
			Help:      "Gas wallet balance in DIA.",
		}),
		lastUpdateTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "feeder",
			Name:      "last_update_time",
			Help:      "Last update time in UTC timestamp.'",
		}),
		pushGatewayURL: pushGatewayURL,
		jobName:        jobName,
		authUser:       authUser,
		authPassword:   authPassword,
	}
	reg.MustRegister(m.uptime)
	reg.MustRegister(m.cpuUsage)
	reg.MustRegister(m.memoryUsage)
	reg.MustRegister(m.contract)
	reg.MustRegister(m.exchangePairs)
	reg.MustRegister(m.gasBalance)
	reg.MustRegister(m.lastUpdateTime)
	return m
}

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
	// get pushgatewayURL variable from kubernetes env variables, if not, the default is https://pushgateway-auth.diadata.org
	pushgatewayURL := utils.Getenv("PUSHGATEWAY_URL", "https://pushgateway-auth.diadata.org")
	authUser := os.Getenv("PUSHGATEWAY_USER")
	authPassword := os.Getenv("PUSHGATEWAY_PASSWORD")

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg, pushgatewayURL, "df_"+hostname, authUser, authPassword)

	//Record start time for uptime calculation
	startTime := time.Now()

	// Get deployed contract and set the metric
	deployedContract := utils.Getenv("DEPLOYED_CONTRACT", "")
	//Set the static contract label for Prometheus monitoring
	m.contract.WithLabelValues(deployedContract).Set(1) // The value is arbitrary; the label holds the address

	// Add this code to expose exchangePairs for monitoring
	if len(exchangePairs) > 0 {
		for _, pair := range exchangePairs {
			pairLabel := fmt.Sprintf("%s:%s:%s-%s",
				pair.Exchange,
				pair.UnderlyingPair.QuoteToken.Blockchain,
				pair.UnderlyingPair.QuoteToken.Address,
				pair.UnderlyingPair.BaseToken.Address)

			m.exchangePairs.WithLabelValues(pairLabel).Set(1)
			log.Infof("Added exchange pair to metrics: %s", pairLabel)
		}
	} else {
		log.Info("No exchange pairs to monitor; DEX_PAIRS environment variable is empty or improperly formatted")
	}

	// Periodically update and push metrics to pushgateway
	go func() {
		for {
			uptime := time.Since(startTime).Hours()
			m.uptime.Set(uptime)

			// Update memory usage
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			memoryUsageMB := float64(memStats.Alloc) / 1024 / 1024 // Convert bytes to megabytes
			m.memoryUsage.Set(memoryUsageMB)

			// Update CPU usage using gopsutil
			percent, _ := cpu.Percent(0, false)
			if len(percent) > 0 {
				m.cpuUsage.Set(percent[0])
			}

			// Get the gas wallet balance
			conn, err := ethclient.Dial(utils.Getenv("BLOCKCHAIN_NODE", "https://rpc.diadata.org"))
			if err != nil {
				log.Errorf("Failed to connect to the Ethereum client: %v", err)
				continue
			}
			privateKeyHex := utils.Getenv("PRIVATE_KEY", "")
			privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
			privateKey, err := crypto.HexToECDSA(privateKeyHex)
			if err != nil {
				log.Fatalf("Failed to load private key: %v", err)
			}
			gasBalance, err := getAddressBalance(conn, privateKey)
			if err != nil {
				log.Errorf("Failed to fetch address balance: %v", err)
			}
			m.gasBalance.Set(gasBalance)

			// Get the latest event timestamp
			lastUpdateTime, err := getLatestEventTimestamp(conn, deployedContract)
			if err != nil {
				log.Errorf("Error fetching latest event timestamp: %v", err)
			}
			m.lastUpdateTime.Set(lastUpdateTime)

			// Push metrics to the Pushgateway
			pushCollector := push.New(m.pushGatewayURL, m.jobName).
				Collector(m.uptime).
				Collector(m.cpuUsage).
				Collector(m.memoryUsage).
				Collector(m.contract).
				Collector(m.exchangePairs).
				Collector(m.gasBalance).
				Collector(m.lastUpdateTime)

			if err := pushCollector.
				BasicAuth(m.authUser, m.authPassword).
				Push(); err != nil {
				log.Errorf("Could not push metrics to Pushgateway: %v", err)
			} else {
				log.Printf("Metrics pushed successfully to Pushgateway")
			}

			time.Sleep(30 * time.Second) // update metrics every 30 seconds
		}
	}()

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
	chainId, err := strconv.ParseInt(utils.Getenv("CHAIN_ID", "1050"), 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse chainId: %v", err)
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

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(chainId))
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

	// Outlook/Alternative: The triggerChannel can also be filled by the oracle updater by any other mechanism.
	onchain.OracleUpdateExecutor(auth, contract, conn, chainId, filtersChannel)
}

func getAddressBalance(client *ethclient.Client, privateKey *ecdsa.PrivateKey) (float64, error) {
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return math.NaN(), fmt.Errorf("Failed to cast public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	balance, err := client.BalanceAt(context.Background(), address, nil)
	if err != nil {
		return math.NaN(), fmt.Errorf("Failed to get balance: %w", err)
	}

	balanceInDIA, _ := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18)).Float64()
	return balanceInDIA, nil
}

func getLatestEventTimestamp(client *ethclient.Client, contractAddress string) (float64, error) {
	// Get the latest block number
	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return math.NaN(), fmt.Errorf("failed to fetch latest block header: %v", err)
	}
	latestBlock := header.Number.Int64()

	// Calculate the start block for the query
	startBlock := latestBlock - 1000
	if startBlock < 0 {
		startBlock = 0 // Ensure the start block is not negative
	}

	// Define filter query for the last 'blockRange' blocks
	query := ethereum.FilterQuery{
		Addresses: []common.Address{common.HexToAddress(contractAddress)},
		FromBlock: big.NewInt(startBlock),
		ToBlock:   big.NewInt(latestBlock),
	}

	// Fetch logs for the specified block range
	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		return math.NaN(), fmt.Errorf("failed to fetch logs: %v", err)
	}

	// Check if logs are empty
	if len(logs) == 0 {
		return math.NaN(), fmt.Errorf("no events found in the last 1000 blocks")
	}

	// Get the latest timestamp from the last log
	lastLog := logs[len(logs)-1]
	blockHeader, err := client.HeaderByHash(context.Background(), lastLog.BlockHash)
	if err != nil {
		return math.NaN(), fmt.Errorf("failed to fetch block header for log: %v", err)
	}

	return float64(blockHeader.Time), nil
}
