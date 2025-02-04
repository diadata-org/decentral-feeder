package models

import (
	"time"
)

type SimulatedTrade struct {
	QuoteToken        Asset
	BaseToken         Asset
	Price             float64
	Volume            float64
	Slippage          float64
	Time              time.Time
	Exchange          Exchange
	PoolAddress       string
	TXHash            string
	EstimatedUSDPrice float64
}

// Struct for decentralized scraper pools.
type SimulatedTradesBlock struct {
	Pair      Pair
	Pool      Pool
	Trades    []SimulatedTrade
	StartTime time.Time
	EndTime   time.Time
	// A tradesblock is atomic if trades all are from the same pool.
	Atomic bool
	// Do we need this?
	ScraperID ScraperID
}

// GetLastTrade returns the latest trade from the slice @trades.
func GetLastSimulatedTrade(trades []SimulatedTrade) (lastTrade SimulatedTrade) {
	for _, trade := range trades {
		if trade.Time.After(lastTrade.Time) {
			lastTrade = trade
		}
	}
	return
}
