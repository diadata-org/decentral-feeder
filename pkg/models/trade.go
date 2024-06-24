package models

import "time"

type Trade struct {
	QuoteToken     Asset
	BaseToken      Asset
	Price          float64
	Volume         float64
	Time           time.Time
	Exchange       Exchange
	PoolAddress    string
	ForeignTradeID string
	// Depending on the connection to the processing layer we might not need it here.
	EstimatedUSDPrice float64
}

// Struct for decentralized scraper pools.
// TO DO: Revisit fields.
type TradesBlock struct {
	// Add field for Asset? So far, we only consider atomic tradesblocks.
	// Asset Asset
	Pair      Pair
	Trades    []Trade
	StartTime time.Time
	EndTime   time.Time
	// Do we need this?
	ScraperID ScraperID
}

// ScraperID is the container identifying a scraper node.
type ScraperID struct {
	// ID could for instance be evm address.
	ID string
	// Human readable name of the entity that is running the scraper.
	Name             string
	RegistrationTime time.Time
}
