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
	env = flag.Bool("env", true, "Get pairs and pools from env variable if set to true. Otherwise, pairs are read from config file.")

	// Comma separated list of exchanges. Only used in case pairs are read from config files.
	exchanges = utils.Getenv("EXCHANGES", "UniswapV2,Binance")
	// Comma separated list of exchangepairs. Pairs must be capitalized and symbols separated by hyphen.
	// It is the responsability of each exchange scraper to determine the correct format for the corresponding API calls.
	// Format should be as follows Binance:ETH-USDT,Binance:BTC-USDT
	exchangePairsEnv = utils.Getenv("EXCHANGEPAIRS", "")
	// Comma separated list of pools.
	// The binary digit in the third position controls the order of the trades in the pool:
	// TO DO: For 0 the original order is taken into consideration, while for 1 the order of all trades in the pool is reversed.
	// Format should be as follows: UniswapV2:0x0d4a11d5EEaaC28EC3F61d100daF4d40471f1852:0,UniswapV2:0xc5be99A02C6857f9Eac67BbCE58DF5572498F40c:0
	poolsEnv = utils.Getenv("POOLS", "")

	exchangePairs []models.ExchangePair
	pools         []models.Pool
)

type metrics struct {
	uptime         prometheus.Gauge
	cpuUsage       prometheus.Gauge
	memoryUsage    prometheus.Gauge
	pushGatewayURL string
	jobName        string
}

func NewMetrics(reg prometheus.Registerer, pushGatewayURL, jobName string) *metrics {
	m := &metrics{
		uptime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "feeder",
			Name:      "uptime_seconds",
			Help:      "Uptime of the application in seconds.",
		}),
		cpuUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "feeder",
			Name:      "cpu_usage_percent",
			Help:      "CPU usage of the application in percent.",
		}),
		memoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "feeder",
			Name:      "memory_usage_megabytes",
			Help:      "Memory usage of the application in megabytes.",
		}),
		pushGatewayURL: pushGatewayURL,
		jobName:        jobName,
	}
	reg.MustRegister(m.uptime)
	reg.MustRegister(m.cpuUsage)
	reg.MustRegister(m.memoryUsage)
	return m
}

func init() {
	flag.Parse()

	if *env {
		exchangePairs = models.ExchangePairsFromEnv(exchangePairsEnv, ENV_SEPARATOR, EXCHANGE_PAIR_SEPARATOR, PAIR_TICKER_SEPARATOR)

		// Extract pools from env var.
		if poolsEnv != "" {
			for _, p := range strings.Split(poolsEnv, ENV_SEPARATOR) {
				var pool models.Pool
				pool.Exchange = scrapers.Exchanges[strings.Split(p, EXCHANGE_PAIR_SEPARATOR)[0]]
				pool.Address = strings.Split(p, EXCHANGE_PAIR_SEPARATOR)[1]
				pool.Blockchain = models.Blockchain{Name: pool.Exchange.Blockchain}
				pools = append(pools, pool)
			}
		}

	} else {
		// Collect all CEX pairs and DEX pools for subsequent scraping.
		// CEX pairs are mapped onto the underlying assets as well.
		// It's the responsability of the corresponding DEX scraper to fetch pool asset information.
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

			p, err := models.GetPoolsFromConfig(exchange)
			if err != nil {
				log.Fatalf("GetPoolsFromConfig for %s: %v", exchange, err)
			}
			pools = append(pools, p...)
		}
	}
}

func main() {
	// get hostname of the container so that we can display it in monitoring dashboards
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to get hostname: %v", err)
	}
	// get pushgatewayURL variable from kubernetes env variables, if not, the default is https://pushgateway.diadata.org
	pushgatewayURL := utils.Getenv("PUSHGATEWAY_URL", "https://pushgateway.diadata.org")

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg, pushgatewayURL, "df_"+hostname)

	// Record start time for uptime calculation
	startTime := time.Now()

	// Update metrics periodically
	go func() {
		for {
			uptime := time.Since(startTime).Seconds()
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
			if err := push.New(m.pushGatewayURL, m.jobName).
				Collector(m.uptime).
				Collector(m.cpuUsage).
				Collector(m.memoryUsage).
				Push(); err != nil {
				log.Errorf("Could not push metrics to Pushgateway: %v", err)
			}

			time.Sleep(10 * time.Second) // update metrics every 10 seconds
		}
	}()

	wg := sync.WaitGroup{}
	tradesblockChannel := make(chan map[string]models.TradesBlock)
	filtersChannel := make(chan []models.FilterPointExtended)
	triggerChannel := make(chan time.Time)
	failoverChannel := make(chan string)

	// Feeder mechanics
	key := utils.Getenv("PRIVATE_KEY", "")
	key_password := utils.Getenv("PRIVATE_KEY_PASSWORD", "")
	deployedContract := utils.Getenv("DEPLOYED_CONTRACT", "")
	blockchainNode := utils.Getenv("BLOCKCHAIN_NODE", "")
	backupNode := utils.Getenv("BACKUP_NODE", "")

	conn, err := ethclient.Dial(blockchainNode)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	connBackup, err := ethclient.Dial(backupNode)
	if err != nil {
		log.Fatalf("Failed to connect to the backup Ethereum client: %v", err)
	}
	chainId, err := strconv.ParseInt(utils.Getenv("CHAIN_ID", ""), 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse chainId: %v", err)
	}

	// Frequency for the trigger ticker initiating the computation of filter values.
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
	// This is for testing purposes for now. Could also be request based or other trigger types.
	triggerTick := time.NewTicker(time.Duration(frequencySeconds) * time.Second)
	go func() {
		for tick := range triggerTick.C {
			// log.Info("Trigger - tick at: ", tick)
			triggerChannel <- tick
		}
	}()

	// Run Processor and subsequent routines.
	go processor.Processor(exchangePairs, pools, tradesblockChannel, filtersChannel, triggerChannel, failoverChannel, &wg)

	// Outlook/Alternative: The triggerChannel can also be filled by the oracle updater by any other mechanism.
	onchain.OracleUpdateExecutor(auth, contract, conn, chainId, filtersChannel)
}
