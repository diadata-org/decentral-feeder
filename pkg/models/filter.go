package models

import (
	"time"
)

type FilterType string

type MetafilterType string

// FilterPoint contains the resulting value of a filter applied to an asset.
type FilterPoint struct {
	Asset      Asset
	Value      float64
	Name       string
	Time       time.Time
	Source     Exchange
	SourceType SourceType
}

type FilterPointPair struct {
	Pool       Pool
	Pair       Pair
	Value      float64
	Name       string
	Time       time.Time
	Source     Exchange
	SourceType SourceType
}

// GroupFilterByAsset returns @fpMap which maps an asset on all filter points contained in @filterPoints.
func GroupFiltersByAsset(filterPoints []FilterPointPair) (fpMap map[Asset][]FilterPointPair) {
	fpMap = make(map[Asset][]FilterPointPair)
	for _, fp := range filterPoints {
		fpMap[fp.Pair.QuoteToken] = append(fpMap[fp.Pair.QuoteToken], fp)
	}
	return
}

// GetValuesFromFilterPoints returns a slice containing just the values from @filterPoints.
func GetValuesFromFilterPoints(filterPoints []FilterPointPair) (filterValues []float64) {
	for _, fp := range filterPoints {
		filterValues = append(filterValues, fp.Value)
	}
	return
}

// GetLatestTimestampFromFilterPoints returns the latest timstamp among all @filterPoints.
func GetLatestTimestampFromFilterPoints(filterPoints []FilterPointPair) (timestamp time.Time) {
	for _, fp := range filterPoints {
		if fp.Time.After(timestamp) {
			timestamp = fp.Time
		}
	}
	return
}

// RemoveOldFilters removes all filter points from @filterPoints whith timestamp more than
// @toleranceSeconds before @timestamp.
func RemoveOldFilters(filterPoints []FilterPointPair, toleranceSeconds int64, timestamp time.Time) (cleanedFilterPoints []FilterPointPair, removedFilters int) {
	for _, fp := range filterPoints {
		if fp.Time.After(timestamp.Add(-time.Duration(toleranceSeconds) * time.Second)) {
			cleanedFilterPoints = append(cleanedFilterPoints, fp)
		} else {
			removedFilters++
		}
	}
	return
}
