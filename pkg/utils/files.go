package utils

import (
	"os"

	"github.com/diadata-org/diaprotocol/pkg/models"
	"github.com/tkanos/gonfig"
)

func GetPoolsFromConfig(exchange string) ([]models.Pool, error) {
	path := os.Getenv("GOPATH") + "/src/github.com/diadata-org/decentral-feeder/config/Pools/" + exchange + ".json"
	type Pools struct {
		Pools []models.Pool
	}
	var p Pools
	err := gonfig.GetConf(path, &p)
	if err != nil {
		return []models.Pool{}, err
	}
	return p.Pools, nil
}

func GetPairsFromConfig(exchange string) ([]models.ExchangePair, error) {
	path := os.Getenv("GOPATH") + "/src/github.com/diadata-org/decentral-feeder/config/pairs/" + exchange + ".json"
	type exchangepairsymbols struct {
		ForeignName string
		QuoteSymbol string
		BaseSymbol  string
	}
	type ExchangePairSymbols struct {
		Pairs []exchangepairsymbols
	}
	var (
		p             ExchangePairSymbols
		exchangePairs []models.ExchangePair
	)
	err := gonfig.GetConf(path, &p)
	if err != nil {
		return []models.ExchangePair{}, err
	}

	symbolIdentificationMap, err := GetSymbolIdentificationMap(exchange)
	if err != nil {
		return exchangePairs, err
	}

	for _, exchangepairsymbol := range p.Pairs {
		var ep models.ExchangePair
		ep.Exchange = exchange
		ep.ForeignName = exchangepairsymbol.ForeignName
		ep.Symbol = exchangepairsymbol.QuoteSymbol

		ep.UnderlyingPair.QuoteToken = symbolIdentificationMap[ExchangeSymbolIdentifier(ep.Symbol, ep.Exchange)]
		ep.UnderlyingPair.BaseToken = symbolIdentificationMap[ExchangeSymbolIdentifier(exchangepairsymbol.BaseSymbol, ep.Exchange)]
		exchangePairs = append(exchangePairs, ep)
	}
	return exchangePairs, nil
}

func GetSymbolIdentificationMap(exchange string) (map[string]models.Asset, error) {
	identificationMap := make(map[string]models.Asset)
	type IdentifiedAsset struct {
		Exchange   string
		Symbol     string
		Blockchain string
		Address    string
		Decimals   uint8
	}
	type IdentifiedAssets struct {
		Tokens []IdentifiedAsset
	}
	var identifiedAssets IdentifiedAssets
	path := os.Getenv("GOPATH") + "/src/github.com/diadata-org/decentral-feeder/config/symbolIdentification/" + exchange + ".json"
	err := gonfig.GetConf(path, &identifiedAssets)
	if err != nil {
		return identificationMap, err
	}

	for _, t := range identifiedAssets.Tokens {
		identificationMap[ExchangeSymbolIdentifier(t.Symbol, t.Exchange)] = models.Asset{
			Symbol:     t.Symbol,
			Blockchain: t.Blockchain,
			Address:    t.Address,
			Decimals:   t.Decimals,
		}
	}
	return identificationMap, nil
}

func ExchangeSymbolIdentifier(symbol string, exchange string) string {
	return symbol + "_" + exchange
}
