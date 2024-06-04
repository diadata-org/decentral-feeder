package models

import "time"

// FilterPoint contains the resulting value of a filter applied to an asset.
type FilterPoint struct {
	Asset      Asset
	Value      float64
	Name       string
	Time       time.Time
	FirstTrade Trade
	LastTrade  Trade
}

type FilterPointExtended struct {
	Pair   Pair
	Source string
	Value  float64
	Name   string
	Time   time.Time
}
