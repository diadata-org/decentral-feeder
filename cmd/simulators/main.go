package main

import (
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"runtime"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/onchain"
	simulationprocessor "github.com/diadata-org/decentral-feeder/pkg/simulations/simulationProcessor"
	utils "github.com/diadata-org/decentral-feeder/pkg/utils"
	diaOracleV2MultiupdateService "github.com/diadata-org/diadata/pkg/dia/scraper/blockchain-scrapers/blockchains/ethereum/diaOracleV2MultiupdateService"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
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
	dexpairs       *prometheus.GaugeVec
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
		dexpairs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "feeder",
				Name:      "dexpairs",
				Help:      "List of DEX pairs to be pushed as labels for each Feeder.",
			},
			[]string{"dex_pair"}, // Label to store each DEX pair
		),
		pushGatewayURL: pushGatewayURL,
		jobName:        jobName,
		authUser:       authUser,
		authPassword:   authPassword,
	}
	reg.MustRegister(m.uptime)
	reg.MustRegister(m.cpuUsage)
	reg.MustRegister(m.memoryUsage)
	reg.MustRegister(m.contract)
	reg.MustRegister(m.dexpairs)
	return m
}

func init() {

	// Extract dexpairs from env var.
	for _, p := range strings.Split(pairsEnv, ENV_SEPARATOR) {
		var pair models.ExchangePair
		parts := strings.Split(p, EXCHANGE_SEPARATOR)
		if len(parts) < 3 {
			log.Warnf("Invalid DEX pair format: %s", p)
			continue
		}
		pair.Exchange = strings.TrimSpace(parts[0])
		pair.UnderlyingPair.QuoteToken.Blockchain = parts[1]
		addresses := strings.Split(parts[2], PAIR_SEPARATOR)
		if len(addresses) < 2 {
			log.Warnf("Invalid address format in DEX pair: %s", p)
			continue
		}
		pair.UnderlyingPair.QuoteToken.Address = addresses[0]
		pair.UnderlyingPair.BaseToken.Address = addresses[1]
		pair.UnderlyingPair.BaseToken.Blockchain = parts[1]
		exchangePairs = append(exchangePairs, pair)
		log.Infof(
			"dex -- blockchain -- address0 -- address1: %s -- %s -- %s -- %s",
			pair.Exchange,
			parts[1],
			addresses[0],
			addresses[1],
		)
	}
	for _, ep := range exchangePairs {
		log.Infof("%s-%s", ep.UnderlyingPair.QuoteToken.Address, ep.UnderlyingPair.BaseToken.Address)
	}

}

func main() {

	// Get hostname of the container so that we can display it in monitoring dashboards
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to get hostname: %v", err)
	}
	// Get pushgatewayURL variable from Kubernetes env variables, if not, the default is https://pushgateway-auth.diadata.org
	pushgatewayURL := utils.Getenv("PUSHGATEWAY_URL", "https://pushgateway-auth.diadata.org")
	authUser := os.Getenv("PUSHGATEWAY_USER")
	authPassword := os.Getenv("PUSHGATEWAY_PASSWORD")

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg, pushgatewayURL, "df_"+hostname, authUser, authPassword)

	// Record start time for uptime calculation
	startTime := time.Now()

	// Get deployed contract and set the metric
	deployedContract := utils.Getenv("DEPLOYED_CONTRACT", "")
	m.contract.WithLabelValues(deployedContract).Set(1) // The value is arbitrary; the label holds the address

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

			// Push metrics to the Pushgateway
			pushCollector := push.New(m.pushGatewayURL, m.jobName).
				Collector(m.uptime).
				Collector(m.cpuUsage).
				Collector(m.memoryUsage).
				Collector(m.contract).
				Collector(m.dexpairs)

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
	blockchainNode := utils.Getenv("BLOCKCHAIN_NODE", "")
	backupNode := utils.Getenv("BACKUP_NODE", "https://testnet-rpc.diadata.org")
	conn, err := ethclient.Dial(blockchainNode)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	connBackup, err := ethclient.Dial(backupNode)
	if err != nil {
		log.Fatalf("Failed to connect to the backup Ethereum client: %v", err)
	}
	chainId, err := strconv.ParseInt(utils.Getenv("CHAIN_ID", "100640"), 10, 64)
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
