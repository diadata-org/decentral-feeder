package models

import (
	"strings"

	"github.com/diadata-org/decentral-feeder/pkg/utils"
	"github.com/tkanos/gonfig"
)

// ExchangePair is the container for a pair as used by exchanges.
// Across exchanges, these pairs cannot be uniquely mapped on asset pairs.
type ExchangePair struct {
	Symbol         string `json:"Symbol"`
	ForeignName    string `json:"ForeignName"`
	Exchange       string `json:"Exchange"`
	UnderlyingPair Pair   `json:"UnderlyingPair"`
}

// Pair is a pair of dia assets.
type Pair struct {
	QuoteToken Asset `json:"QuoteToken"`
	BaseToken  Asset `json:"BaseToken"`
}

func (p *Pair) ExchangePairIdentifier(exchange string) string {
	return exchange + "-" + p.Identifier()
}

func (p *Pair) Identifier() string {
	return p.QuoteToken.Blockchain + "-" + p.QuoteToken.Address + "-" + p.BaseToken.Blockchain + "-" + p.BaseToken.Address
}

// ExchangePairsFromEnv parses the string @exchangePairsEnv consisting of pairs on exchanges
// and returns full asset information for the corresponding exchangepairs.
func ExchangePairsFromEnv(
	exchangePairsEnv string,
	envSeparator string,
	exchangePairSeparator string,
	pairTickerSeparator string,
) (exchangePairs []ExchangePair) {

	// epMap maps an exchange on the underlying pair symbol tickers.
	epMap := make(map[string][]string)
	for _, ep := range strings.Split(exchangePairsEnv, envSeparator) {
		exchange := strings.TrimSpace(strings.Split(ep, exchangePairSeparator)[0])
		pairSymbol := strings.TrimSpace(strings.Split(ep, exchangePairSeparator)[1])
		epMap[exchange] = append(epMap[exchange], pairSymbol)
	}

	// Assign assets to pair symbols.
	for exchange := range epMap {
		symbolIdentificationMap, err := GetSymbolIdentificationMap(exchange)
		if err != nil {
			log.Fatal("GetSymbolIdentificationMap: ", err)
		}
		for _, pairSymbol := range epMap[exchange] {
			symbols := strings.Split(pairSymbol, pairTickerSeparator)
			var ep ExchangePair
			ep.Exchange = exchange
			ep.ForeignName = pairSymbol
			ep.Symbol = symbols[0]
			ep.UnderlyingPair.QuoteToken = symbolIdentificationMap[ExchangeSymbolIdentifier(symbols[0], exchange)]
			ep.UnderlyingPair.BaseToken = symbolIdentificationMap[ExchangeSymbolIdentifier(symbols[1], exchange)]
			exchangePairs = append(exchangePairs, ep)
		}
	}
	return
}

// MakeExchangepairMap returns a map in which exchangepairs are grouped by exchange string key.
func MakeExchangepairMap(exchangePairs []ExchangePair) map[string][]ExchangePair {
	exchangepairMap := make(map[string][]ExchangePair)
	for _, ep := range exchangePairs {
		log.Infof("exchangepair in exchangepairMap: %s:%s-%s:%s ... ", ep.UnderlyingPair.QuoteToken.Blockchain, ep.UnderlyingPair.QuoteToken.Address, ep.UnderlyingPair.BaseToken.Blockchain, ep.UnderlyingPair.BaseToken.Address)
		log.Infof("... and corresponding exchange: %s", ep.Exchange)
		exchangepairMap[ep.Exchange] = append(exchangepairMap[ep.Exchange], ep)
	}
	return exchangepairMap
}

// MakeTickerPairMap returns a map that maps a pair ticker onto the underlying pair with full asset information.
func MakeTickerPairMap(exchangePairs []ExchangePair) map[string]Pair {
	tickerPairMap := make(map[string]Pair)
	for _, ep := range exchangePairs {
		symbols := strings.Split(ep.ForeignName, "-")
		if len(symbols) < 2 {
			continue
		}
		tickerPairMap[symbols[0]+symbols[1]] = ep.UnderlyingPair
	}
	return tickerPairMap
}

func GetPairsFromConfig(exchange string) ([]ExchangePair, error) {
	path := utils.GetPath("pairs/", exchange)
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
		exchangePairs []ExchangePair
	)
	err := gonfig.GetConf(path, &p)
	if err != nil {
		return []ExchangePair{}, err
	}

	symbolIdentificationMap, err := GetSymbolIdentificationMap(exchange)
	if err != nil {
		return exchangePairs, err
	}

	for _, exchangepairsymbol := range p.Pairs {
		var ep ExchangePair
		ep.Exchange = exchange
		ep.ForeignName = exchangepairsymbol.ForeignName
		ep.Symbol = exchangepairsymbol.QuoteSymbol

		ep.UnderlyingPair.QuoteToken = symbolIdentificationMap[ExchangeSymbolIdentifier(ep.Symbol, ep.Exchange)]
		ep.UnderlyingPair.BaseToken = symbolIdentificationMap[ExchangeSymbolIdentifier(exchangepairsymbol.BaseSymbol, ep.Exchange)]
		exchangePairs = append(exchangePairs, ep)
	}
	return exchangePairs, nil
}

// GetSymbolIdentificationMap returns a map which maps an asset's symbol ticker on @exchange onto the underlying asset.
func GetSymbolIdentificationMap(exchange string) (map[string]Asset, error) {
	identificationMap := make(map[string]Asset)
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
	path := utils.GetPath("symbolIdentification/", exchange)
	err := gonfig.GetConf(path, &identifiedAssets)
	if err != nil {
		return identificationMap, err
	}

	for _, t := range identifiedAssets.Tokens {
		identificationMap[ExchangeSymbolIdentifier(t.Symbol, t.Exchange)] = Asset{
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
