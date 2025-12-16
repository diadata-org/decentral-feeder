package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v56/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const PAIR_SEPARATOR = "-"

var EXCHANGE = "CoinBase"

// Structs for Exchange.json and SymbolIdentification
type ExchangePair struct {
	Pair          string `json:"Pair"`
	WatchDogDelay int    `json:"WatchDogDelay,omitempty"`
	// ... other fields omitted
}

type ExchangePairConfig struct {
	Exchangepairs []ExchangePair `json:"ExchangePairs"`
}

type Pool struct {
	Address       string `json:"Address"`
	Blockchain    string `json:"Blockchain"`
	WatchDogDelay int    `json:"WatchDogDelay,omitempty"`
}

type PoolConfig struct {
	Pools []Pool `json:"Pools"`
}

type Asset struct {
	Symbol     string `json:"Symbol"`
	Address    string `json:"Address"`
	Blockchain string `json:"Blockchain"`
}

type SymbolAsset struct {
	Symbol     string `json:"Symbol"`
	Exchange   string `json:"Exchange"`
	Address    string `json:"Address"`
	Blockchain string `json:"Blockchain"`
}
type SymbolIdentification struct {
	Tokens []SymbolAsset `json:"Tokens"`
}

func main() {
	prMode := flag.Bool("pr", false, "If set, creates a PR with the updated file")
	outPath := flag.String("out", "", "If set, writes all updated config files to this local folder")
	factor := flag.Float64("factor", 2.0, "Multiplicative factor for the watchdog time")
	ref := flag.String("ref", "master", "Ref/branch for the fetch & PR base (default master)")
	flag.Parse()

	owner, repo, dir := "diadata-org", "decentral-feeder", "config/exchangePairs"

	client := getGithubClient()
	exchanges, err := FetchFilenamesWithoutExtensionFromGithub(client, owner, repo, dir, *ref)
	if err != nil {
		log.Fatal("FetchFilenames: ", err)
	}
	log.Infof("Exchanges: %v", exchanges)

	// For PR: prep branch, etc
	var branchName string
	if *prMode {
		branchName = fmt.Sprintf("update-watchdogs-%d", time.Now().Unix())
		refObj, _, err := client.Git.GetRef(context.Background(), owner, repo, "refs/heads/"+*ref)
		if err != nil {
			log.Fatalf("Error getting ref: %v", err)
		}
		// Create branch
		newRef := &github.Reference{
			Ref: github.String("refs/heads/" + branchName),
			Object: &github.GitObject{
				SHA: refObj.Object.SHA,
			},
		}
		_, _, err = client.Git.CreateRef(context.Background(), owner, repo, newRef)
		if err != nil {
			log.Fatalf("Error creating branch: %v", err)
		}
	}

	for _, exchange := range exchanges {
		log.Infof("===== Processing %s =====", exchange)
		pairsPath := fmt.Sprintf("config/exchangePairs/%s.json", exchange)
		symidPath := fmt.Sprintf("config/symbolIdentification/%s.json", exchange)

		symbolFile, _, err := fetchGithubFile(client, owner, repo, symidPath)
		if err != nil {
			log.Warnf("Symbol identification file missing for %s, skipping: %v", exchange, err)
			continue
		}
		var symID SymbolIdentification
		if err := json.Unmarshal(symbolFile, &symID); err != nil {
			log.Warnf("Could not decode symbolIdentification for %s: %v", exchange, err)
			continue
		}
		symIDMap := makeIdentificationMap(symID)

		pairsFile, sha, err := fetchGithubFile(client, owner, repo, pairsPath)
		if err != nil {
			log.Warnf("Error fetching %s: %v", pairsPath, err)
			continue
		}
		var pairs ExchangePairConfig
		if err := json.Unmarshal(pairsFile, &pairs); err != nil {
			log.Warnf("Could not decode %s: %v", pairsPath, err)
			continue
		}

		log.Infof("Processing pairs for %s", exchange)
		for i, pair := range pairs.Exchangepairs {
			symbols := strings.Split(pair.Pair, PAIR_SEPARATOR)
			if len(symbols) != 2 {
				continue
			}
			quoteAsset := symIDMap[symbols[0]]
			baseAsset := symIDMap[symbols[1]]
			watchdog := computeWatchdogTime(quoteAsset, baseAsset, exchange, *factor)
			log.Infof("Pair %s-%s old vs. new watchdog: %d -- %d", quoteAsset.Symbol, baseAsset.Symbol, pair.WatchDogDelay, watchdog)
			pairs.Exchangepairs[i].WatchDogDelay = watchdog
			time.Sleep(500 * time.Millisecond)
		}

		// Marshal updated JSON (pretty-printed)
		updated, err := json.MarshalIndent(pairs, "", "  ")
		if err != nil {
			log.Errorf("Error marshaling updated JSON for %s: %v", exchange, err)
			continue
		}

		if *prMode {
			log.Infof("Updating %s in branch %s", pairsPath, branchName)
			opts := &github.RepositoryContentFileOptions{
				Message: github.String(fmt.Sprintf("Update watchdogs in %s.json @%d", exchange, time.Now().Unix())),
				Content: updated,
				SHA:     github.String(sha),
				Branch:  github.String(branchName),
			}
			_, _, err = client.Repositories.UpdateFile(context.Background(),
				owner, repo, pairsPath, opts)
			if err != nil {
				log.Errorf("Error updating file: %v", err)
			}
		} else if *outPath != "" {
			// Write each updated file as <outPath>/<exchange>.json
			outFile := filepath.Join(*outPath, exchange+".json")
			log.Infof("Writing updated file to %s", outFile)
			if err := os.WriteFile(outFile, updated, 0644); err != nil {
				log.Errorf("Error writing output file: %v", err)
			}
		} else {
			log.Infof("Updated %s config file:\n%s", exchange, string(updated))
		}
	}

	// ---- POOLS ----
	poolsDir := "config/pools"
	poolFiles, err := FetchFilenamesWithoutExtensionFromGithub(client, owner, repo, poolsDir, *ref)
	if err != nil {
		log.Fatal("FetchFilenames pools: ", err)
	}
	log.Infof("Pools: %v", poolFiles)

	for _, poolFile := range poolFiles {
		log.Infof("===== Processing pool config %s =====", poolFile)
		poolsPath := fmt.Sprintf("%s/%s.json", poolsDir, poolFile)

		poolsFile, sha, err := fetchGithubFile(client, owner, repo, poolsPath)
		if err != nil {
			log.Warnf("Error fetching %s: %v", poolsPath, err)
			continue
		}
		var poolsConfig PoolConfig
		if err := json.Unmarshal(poolsFile, &poolsConfig); err != nil {
			log.Warnf("Could not decode %s: %v", poolsPath, err)
			continue
		}

		for i, pool := range poolsConfig.Pools {
			watchdog := computeWatchdogTimeForPool(pool, *factor)
			log.Infof("Pool %s:%s old vs new watchdog: %d -- %d", pool.Blockchain, pool.Address, pool.WatchDogDelay, watchdog)
			poolsConfig.Pools[i].WatchDogDelay = watchdog
			time.Sleep(500 * time.Millisecond)
		}

		updated, err := json.MarshalIndent(poolsConfig, "", "  ")
		if err != nil {
			log.Errorf("Error marshaling updated pool config JSON for %s: %v", poolFile, err)
			continue
		}

		if *prMode {
			// update in PR
			log.Infof("Updating %s in branch %s", poolsPath, branchName)
			opts := &github.RepositoryContentFileOptions{
				Message: github.String(fmt.Sprintf("Update watchdogs in pools/%s.json @%d", poolFile, time.Now().Unix())),
				Content: updated,
				SHA:     github.String(sha),
				Branch:  github.String(branchName),
			}
			_, _, err = client.Repositories.UpdateFile(context.Background(),
				owner, repo, poolsPath, opts)
			if err != nil {
				log.Errorf("Error updating pool file: %v", err)
			}
		} else if *outPath != "" {
			outFile := filepath.Join(*outPath, "pools-"+poolFile+".json")
			log.Infof("Writing updated pool file to %s", outFile)
			if err := os.WriteFile(outFile, updated, 0644); err != nil {
				log.Errorf("Error writing output file: %v", err)
			}
		} else {
			log.Infof("Updated %s pool config file:\n%s", poolFile, string(updated))
		}
	}

	if *prMode {
		newPR := &github.NewPullRequest{
			Title:               github.String("Update exchangePairs watchdogs via script"),
			Head:                github.String(branchName),
			Base:                github.String(*ref),
			Body:                github.String("This PR updates watchdog delay times in all exchangePairs config files based on latest 24h trade frequency."),
			MaintainerCanModify: github.Bool(true),
		}
		pr, _, err := client.PullRequests.Create(context.Background(), owner, repo, newPR)
		if err != nil {
			log.Errorf("Error creating PR: %v", err)
		}
		log.Infof("Pull request created: %s", pr.GetHTMLURL())
	}
}

// fetchGithubFile returns file content of a github file using an API client.
func fetchGithubFile(client *github.Client, owner, repo, path string) (content []byte, sha string, err error) {
	fileContent, _, _, err := client.Repositories.GetContents(context.Background(), owner, repo, path, nil)
	if err != nil {
		return nil, "", err
	}
	if fileContent == nil || fileContent.Content == nil {
		return nil, "", fmt.Errorf("failed to get file content")
	}
	decoded, err := base64.StdEncoding.DecodeString(*fileContent.Content)
	return decoded, *fileContent.SHA, err
}

// getNumTrades returns the number of trades for pair @quote-base on @exchange in the last 24h.
func getNumTrades(quote Asset, base Asset, exchange string) (int, error) {
	url := fmt.Sprintf("https://api.diadata.org/v1/feedStats/%s/%s", quote.Blockchain, quote.Address)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var data struct {
		ExchangeVolumes []struct {
			Exchange    string `json:"Exchange"`
			PairVolumes []struct {
				Pair struct {
					QuoteToken Asset `json:"QuoteToken"`
					BaseToken  Asset `json:"BaseToken"`
				} `json:"Pair"`
				TradesCount int `json:"TradesCount"`
			} `json:"PairVolumes"`
		} `json:"ExchangeVolumes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}
	for _, ex := range data.ExchangeVolumes {
		if strings.EqualFold(ex.Exchange, exchange) {
			for _, pv := range ex.PairVolumes {
				log.Info("pcv: ", pv.Pair.QuoteToken.Symbol+pv.Pair.BaseToken.Symbol)
				log.Info("numTrades: ", pv.TradesCount)
				if pv.Pair.QuoteToken.Symbol == quote.Symbol && pv.Pair.BaseToken.Symbol == base.Symbol {
					return pv.TradesCount, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("trades count not found for %s-%s on %s", quote.Symbol, base.Symbol, exchange)
}

// computeWatchdogTime returns the watchdog time for pair @quote-base based on the number
// of trades in the last 24h.
func computeWatchdogTime(quote, base Asset, exchange string, factor float64) int {
	numTrades, err := getNumTrades(quote, base, exchange)
	if err != nil || numTrades == 0 {
		return 600
	}
	secondsPerTrade := float64(60*60*24) / float64(numTrades)
	time := int(factor * secondsPerTrade)
	if time < 600 {
		return 600
	}
	return time
}

func getNumTradesByPool(poolAddress string, blockchain string) (int, error) {
	url := fmt.Sprintf("https://api.diadata.org/v1/feedStats/%s/%s", blockchain, poolAddress)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	// Try first format
	var data struct {
		Trades24h int `json:"Trades24h"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err == nil && data.Trades24h > 0 {
		return data.Trades24h, nil
	}
	// Try fallback format
	var fallback struct {
		PoolVolumes []struct {
			Pool        string `json:"Pool"`
			TradesCount int    `json:"TradesCount"`
		} `json:"PoolVolumes"`
	}
	if err := json.Unmarshal(bodyBytes, &fallback); err == nil {
		for _, pv := range fallback.PoolVolumes {
			if strings.EqualFold(pv.Pool, poolAddress) {
				return pv.TradesCount, nil
			}
		}
	}
	return 0, fmt.Errorf("trades count not found for pool %s on %s", poolAddress, blockchain)
}

// ComputeWatchdogTime for pools
func computeWatchdogTimeForPool(pool Pool, factor float64) int {
	numTrades, err := getNumTradesByPool(pool.Address, pool.Blockchain)
	if err != nil || numTrades == 0 {
		return 600
	}
	secondsPerTrade := float64(60*60*24) / float64(numTrades)
	time := int(factor * secondsPerTrade)
	if time < 600 {
		return 600
	}
	return time
}

// getGithubClient returns a client for github API requests.
func getGithubClient() *github.Client {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Error("GITHUB_TOKEN not set in your environment")
		os.Exit(1)
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc)
}

func makeIdentificationMap(symID SymbolIdentification) map[string]Asset {
	m := make(map[string]Asset)
	for _, a := range symID.Tokens {
		m[a.Symbol] = Asset{Symbol: a.Symbol, Address: a.Address, Blockchain: a.Blockchain}
	}
	return m
}

func FetchFilenamesWithoutExtensionFromGithub(
	client *github.Client,
	owner, repo, dir, ref string,
) ([]string, error) {
	opts := &github.RepositoryContentGetOptions{Ref: ref}
	_, dirContent, _, err := client.Repositories.GetContents(context.Background(), owner, repo, dir, opts)
	if err != nil {
		return nil, err
	}
	// fileContent is nil if dir is a directory
	var filenames []string
	for _, file := range dirContent {
		if file.GetType() == "file" {
			name := file.GetName()
			base := strings.TrimSuffix(name, filepath.Ext(name))
			if base != "" {
				filenames = append(filenames, base)
			}
		}
	}
	return filenames, nil
}
