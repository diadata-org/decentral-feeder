package main

import (
	"flag"
	"math/big"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/onchain"
	"github.com/diadata-org/decentral-feeder/pkg/processor"
	scrapers "github.com/diadata-org/decentral-feeder/pkg/scrapers"
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
	// Separator for entries in the environment variables, i.e. Binance:BTC-USDT,KuCoin:BTC-USDT.
	ENV_SEPARATOR = ","
	// Separator for a pair ticker's assets, i.e. BTC-USDT.
	PAIR_TICKER_SEPARATOR = "-"
	// Separator for a pair on a given exchange, i.e. Binance:BTC-USDT.
	EXCHANGE_PAIR_SEPARATOR = ":"
)

var (
	env = flag.Bool("env", true, "Get pairs from env variable if set to true. Otherwise, pairs are read from config file.")

	// Comma separated list of exchanges. Only used in case pairs are read from config files.
	exchanges = utils.Getenv("EXCHANGES", "UniswapV2,Binance,Simulation")
	// Comma separated list of exchangepairs. Pairs must be capitalized and symbols separated by hyphen.
	// It is the responsability of each exchange scraper to determine the correct format for the corresponding API calls.
	// Format should be as follows Binance:ETH-USDT,Binance:BTC-USDT
	exchangePairsEnv = utils.Getenv("EXCHANGEPAIRS", "Crypto.com:BTC-USDT,Crypto.com:BTC-USD")
	exchangePairs    []models.ExchangePair
)

type metrics struct {
	uptime         prometheus.Gauge
	cpuUsage       prometheus.Gauge
	memoryUsage    prometheus.Gauge
	contract       *prometheus.GaugeVec
	exchangePairs  *prometheus.GaugeVec
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
		pushGatewayURL: pushGatewayURL,
		jobName:        jobName,
		authUser:       authUser,
		authPassword:   authPassword,
	}
	reg.MustRegister(m.uptime)
	reg.MustRegister(m.cpuUsage)
	reg.MustRegister(m.memoryUsage)
	reg.MustRegister(m.contract)
	return m
}

func init() {
	flag.Parse()

	if *env {
		exchangePairs = models.ExchangePairsFromEnv(exchangePairsEnv, ENV_SEPARATOR, EXCHANGE_PAIR_SEPARATOR, PAIR_TICKER_SEPARATOR)

	} else {
		// Collect all CEX pairs for subsequent scraping.
		// CEX pairs are mapped onto the underlying assets as well.
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
		}
	}
}

func main() {

	// get hostname of the container so that we can display it in monitoring dashboards
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

	// Record start time for uptime calculation
	startTime := time.Now()

	// Get deployed contract and set the metric
	deployedContract := utils.Getenv("DEPLOYED_CONTRACT", "")
	// Set the static contract label for Prometheus monitoring
	m.contract.WithLabelValues(deployedContract).Set(1) // The value is arbitrary; the label holds the address

	exchangePairsList := strings.Split(exchangePairsEnv, ",")
	for _, pair := range exchangePairsList {
		pair = strings.TrimSpace(pair) // Clean whitespace
		if pair != "" {
			m.exchangePairs.WithLabelValues(pair).Set(1)
		}
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

			// Push metrics to the Pushgateway
			pushCollector := push.New(m.pushGatewayURL, m.jobName).
				Collector(m.uptime).
				Collector(m.cpuUsage).
				Collector(m.memoryUsage).
				Collector(m.contract).
				Collector(m.exchangePairs)

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
	tradesblockChannel := make(chan map[string]models.TradesBlock)
	filtersChannel := make(chan []models.FilterPointPair)
	triggerChannel := make(chan time.Time)
	failoverChannel := make(chan string)

	// Feeder mechanics
	privateKeyHex := utils.Getenv("PRIVATE_KEY", "")
	blockchainNode := utils.Getenv("BLOCKCHAIN_NODE", "https://testnet-rpc.diadata.org")
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
	frequencySeconds, err := strconv.Atoi(utils.Getenv("FREQUENCY_SECONDS", "20"))
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
	go processor.Processor(exchangePairs, tradesblockChannel, filtersChannel, triggerChannel, failoverChannel, &wg)

	// Outlook/Alternative: The triggerChannel can also be filled by the oracle updater by any other mechanism.
	onchain.OracleUpdateExecutor(auth, contract, conn, chainId, filtersChannel)
}
