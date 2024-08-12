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
	Value  float64
	Name   string
	Time   time.Time
	Source string
}

// GroupFilterByAsset returns @fpMap which maps an asset on all extended filter points contained in @filterPoints.
func GroupFilterByAsset(filterPoints []FilterPointExtended) (fpMap map[Asset][]FilterPointExtended) {
	fpMap = make(map[Asset][]FilterPointExtended)
	for _, fp := range filterPoints {
		fpMap[fp.Pair.QuoteToken] = append(fpMap[fp.Pair.QuoteToken], fp)
	}
	return
}

// GetValuesFromFilterPoints returns a slice containing just the values from @filterPoints.
func GetValuesFromFilterPoints(filterPoints []FilterPointExtended) (filterValues []float64) {
	for _, fp := range filterPoints {
		filterValues = append(filterValues, fp.Value)
	}
	return
}

// GetLatestTimestampFromFilterPoints returns the latest timstamp among all @filterPoints.
func GetLatestTimestampFromFilterPoints(filterPoints []FilterPointExtended) (timestamp time.Time) {
	for _, fp := range filterPoints {
		if fp.Time.After(timestamp) {
			timestamp = fp.Time
		}
	}
	return
}

// RemoveOldFilters removes all filter points from @filterPoints whith timestamp more than
// @toleranceSeconds before @timestamp.
func RemoveOldFilters(filterPoints []FilterPointExtended, toleranceSeconds int64, timestamp time.Time) (cleanedFilterPoints []FilterPointExtended, removedFilters int) {
	for _, fp := range filterPoints {
		if fp.Time.After(timestamp.Add(-time.Duration(toleranceSeconds) * time.Second)) {
			cleanedFilterPoints = append(cleanedFilterPoints, fp)
		} else {
			removedFilters++
		}
	}
	return
}
