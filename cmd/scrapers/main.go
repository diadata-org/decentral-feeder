package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	diaOracleV2MultiupdateService "github.com/diadata-org/diadata/pkg/dia/scraper/blockchain-scrapers/blockchains/ethereum/diaOracleV2MultiupdateService"
	"github.com/diadata-org/lumina-library/metrics"
	models "github.com/diadata-org/lumina-library/models"
	"github.com/diadata-org/lumina-library/onchain"
	"github.com/diadata-org/lumina-library/processor"
	"github.com/diadata-org/lumina-library/scrapers"
	utils "github.com/diadata-org/lumina-library/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const (
	CONFIG_RELOAD_SECONDS = 30
	// Separator for entries in the environment variables, i.e. Binance:BTC-USDT,KuCoin:BTC-USDT.
	ENV_SEPARATOR = ","
	// Separator for a pair ticker's assets, i.e. BTC-USDT.
	PAIR_TICKER_SEPARATOR = "-"
	// Separator for a pair on a given exchange, i.e. Binance:BTC-USDT.
	EXCHANGE_PAIR_SEPARATOR = ":"
	remoteConfigURL         = "https://raw.githubusercontent.com/diadata-org/decentral-feeder/master/config/exchange_pairs/pairs.json"
	localConfigPath         = "../../config/exchange_pairs/pairs.json"
)

var (
	// Comma separated list of exchangepairs. Pairs must be capitalized and symbols separated by hyphen.
	// It is the responsability of each exchange scraper to determine the correct format for the corresponding API calls.
	// Format should be as follows Binance:ETH-USDT,Binance:BTC-USDT
	// exchangePairsEnv = utils.Getenv("EXCHANGEPAIRS", "")
	// Comma separated list of pools on an exchange.
	// Format should be as follows PancakeswapV3:0xac0fe1c4126e4a9b644adfc1303827e3bb5dddf3:i
	// where 0<=i<=2 determines the order of the returned swaps.
	// 0: original pool order
	// 1: reversed pool order
	// 2: both directions
	poolsEnv = utils.Getenv("POOLS", "")

	exchangePairs     []models.ExchangePair
	pools             []models.Pool
	initialConfigHash string
)

type pairEntry struct {
	Pair          string `json:"Pair"`
	WatchDogDelay int    `json:"WatchDogDelay"`
}

type RawConfig struct {
	// e.g. [{"Binance": [{"Pair": "AAVE-USDT", "WatchDogDelay": 300}]}, {"OKEx": [{"Pair": "AAVE-USDT", "WatchDogDelay": 60}]}]
	ExchangePairs []map[string][]pairEntry `json:"ExchangePairs"`
}

func init() {
	configData, err := fetchRemoteConfig()
	if err != nil {
		log.Warnf("Failed to fetch remote config: %v. Falling back to local config.", err)
		configData, err = os.ReadFile(filepath.Clean(localConfigPath))
		if err != nil {
			log.Fatalf("Failed to read local config file: %v", err)
		}
	}

	var cfg RawConfig
	if err := json.Unmarshal(configData, &cfg); err != nil {
		log.Fatalf("Failed to parse config JSON: %v", err)
	}

	exchangePairs = buildPairsFromConfig(cfg)
	initialConfigHash = hashConfig(cfg)

	log.Infof("Total %d exchange pairs loaded", len(exchangePairs))

	// exchangePairs = models.ExchangePairsFromEnv(exchangePairsEnv, ENV_SEPARATOR, EXCHANGE_PAIR_SEPARATOR, PAIR_TICKER_SEPARATOR, getPath2Config())
	var poolErr error
	pools, poolErr = models.PoolsFromEnv(poolsEnv, ENV_SEPARATOR, EXCHANGE_PAIR_SEPARATOR)
	if poolErr != nil {
		log.Fatal("Read pools from ENV var: ", poolErr)
	}

}

func buildPairsFromConfig(cfg RawConfig) []models.ExchangePair {
	epMap := make(map[string][]string)
	wdMap := make(map[string]int)
	for _, exchObj := range cfg.ExchangePairs {
		for exchange, entries := range exchObj {
			for _, entry := range entries {
				p := strings.TrimSpace(entry.Pair)
				if p == "" || !strings.Contains(p, PAIR_TICKER_SEPARATOR) {
					log.Warnf("Invalid pair format: %s", p)
					continue
				}
				epMap[exchange] = append(epMap[exchange], p)
				wdMap[exchange+":"+p] = entry.WatchDogDelay
			}
		}
	}

	return models.ExchangePairsFromPairs(epMap, PAIR_TICKER_SEPARATOR, getPath2Config(), wdMap)
}

func fetchRemoteConfig() ([]byte, error) {
	resp, err := http.Get(remoteConfigURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, io.ErrUnexpectedEOF
	}
	return io.ReadAll(resp.Body)
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
	conn, err := utils.MakeEthClient(blockchainNode, backupNode)
	if err != nil {
		log.Fatalf("MakeEthClient: %v", err)
	}
	connBackup, err := utils.MakeEthClient(backupNode, blockchainNode)
	if err != nil {
		log.Fatalf("MakeEthClient: %v", err)
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

	// Use a ticker for triggering the processing.
	// This is for testing purposes for now. Could also be request based or other trigger types.
	triggerTick := time.NewTicker(time.Duration(frequencySeconds) * time.Second)
	go func() {
		for tick := range triggerTick.C {
			// log.Info("Trigger - tick at: ", tick)
			triggerChannel <- tick
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	// Run processor
	go processor.Processor(ctx, exchangePairs, pools, tradesblockChannel, filtersChannel, triggerChannel, failoverChannel, &wg)

	go watchConfigFileWithSeed(localConfigPath, time.Duration(CONFIG_RELOAD_SECONDS)*time.Second, initialConfigHash, func(newCfg RawConfig) {
		newPairs := buildPairsFromConfig(newCfg)
		log.Infof("Detected config change: %d pairs", len(newPairs))
		scrapers.UpdateExchangePairs(newPairs) // -> Collector hot update
		exchangePairs = newPairs
		cancel()
		close(tradesblockChannel)
		{
			time.Sleep(2 * time.Second)
			tradesblockChannel = make(chan map[string]models.TradesBlock)
			ctx, cancel = context.WithCancel(context.Background())
			go processor.Processor(ctx, exchangePairs, pools, tradesblockChannel, filtersChannel, triggerChannel, failoverChannel, &wg)
		}
	})

	// Move metrics setup here, right before the blocking call
	// Only setup metrics collection if metrics are enabled and metrics object exists
	if pushgatewayEnabled && m != nil {
		// Set the static contract label for Prometheus monitoring
		m.Contract.WithLabelValues(deployedContract).Set(1)

		for _, ep := range exchangePairs {
			// build label like Binance:AAVE-USDT
			label := ep.Exchange + ":" +
				ep.UnderlyingPair.QuoteToken.Symbol + "-" +
				ep.UnderlyingPair.BaseToken.Symbol

			m.ExchangePairs.WithLabelValues(label).Set(1)
		}

		// Push metrics to Pushgateway if enabled
		go metrics.PushMetricsToPushgateway(m, startTime, conn, privateKey, deployedContract)
	}

	// This should be the final line of main (blocking call)
	onchain.OracleUpdateExecutor(auth, contract, contractBackup, conn, connBackup, chainID, filtersChannel)
}

func watchConfigFileWithSeed(path string, interval time.Duration, seed string, onChange func(RawConfig)) {
	var lastHash string = seed
	for {
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			log.Warnf("Watcher: failed to load config: %v", err)
			time.Sleep(interval)
			continue
		}
		var cfg RawConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			log.Warnf("Watcher: failed to parse config: %v", err)
			time.Sleep(interval)
			continue
		}
		h := hashConfig(cfg)
		if h != lastHash {
			onChange(cfg)
			lastHash = h
		}
		time.Sleep(interval)
	}
}

// hashConfig for RawConfig create a hash of the config: flatten it into a list of (exchange, pair, watchdog), sort it, and hash it
func hashConfig(cfg RawConfig) string {
	type flat struct {
		Exchange string `json:"ex"`
		Pair     string `json:"pair"`
		WD       int    `json:"wd"`
	}

	flatList := make([]flat, 0, 64)
	for _, exchObj := range cfg.ExchangePairs {
		for ex, entries := range exchObj {
			for _, e := range entries {
				flatList = append(flatList, flat{
					Exchange: strings.TrimSpace(ex),
					Pair:     strings.TrimSpace(e.Pair),
					WD:       e.WatchDogDelay,
				})
			}
		}
	}

	// sort by Exchange, Pair, WD to make it order-independent
	sort.Slice(flatList, func(i, j int) bool {
		if flatList[i].Exchange != flatList[j].Exchange {
			return flatList[i].Exchange < flatList[j].Exchange
		}
		if flatList[i].Pair != flatList[j].Pair {
			return flatList[i].Pair < flatList[j].Pair
		}
		return flatList[i].WD < flatList[j].WD
	})

	b, _ := json.Marshal(flatList)
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:])
}

func getPath2Config() string {
	usr, _ := user.Current()
	dir := usr.HomeDir
	if dir == "/root" || dir == "/home" {
		return "/config/symbolIdentification/"
	}
	return os.Getenv("GOPATH") + "/src/github.com/diadata-org/decentral-feeder/config/symbolIdentification/"
}
