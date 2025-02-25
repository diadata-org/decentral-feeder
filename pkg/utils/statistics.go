package utils

import (
	"math"
	"sort"
)

func Median(samples []float64) (median float64) {
	var length = len(samples)
	if length > 0 {
		sort.Float64s(samples)
		if length%2 == 0 {
			median = (samples[length/2-1] + samples[length/2]) / 2
		} else {
			median = samples[(length+1)/2-1]
		}
	}
	return
}

func Average(samples []float64) (average float64) {
	var length = len(samples)
	if length > 0 {
		for _, s := range samples {
			average += s
		}
		average /= float64(length)
	}
	return
}

// RemoveOutliers removes outliers from @samples which have an average distance to the rest larger
// than @threshold. Distance is normalized by the 2-norm. The removal process is iteratively and
// returns as soon as no distance is larger than threshold or len(@cleanedSamples)=2.
// @indices is the slice of indices which were originally in samples, so @samples[@indices]=@cleanedSamples.
func RemoveOutliers(samples []float64, threshold float64) (cleanedSamples []float64, indices []int) {
	// Keep track of old indices.
	var indexMap = make(map[int]int)
	for i := range samples {
		indexMap[i] = i
	}

	cleanedSamples = samples
	m, ind := max(AvgDistances(cleanedSamples))

	for m > threshold && len(cleanedSamples) > 2 {
		cleanedSamples = removeIndex(cleanedSamples, ind)
		indexMap = pushIndexMap(indexMap, ind)
		m, ind = max(AvgDistances(cleanedSamples))
	}

	for _, value := range indexMap {
		indices = append(indices, value)
	}
	sort.Ints(indices)

	return
}

// AvgDistances returns a slice. The i-th entry of @distSlice is the average distance
// from all other entries normalized by the 2-norm of @samples.
func AvgDistances(samples []float64) (distSlice []float64) {
	l := len(samples)
	N := Norm(samples)

	for i := 0; i < l; i++ {
		var s float64
		for j := 0; j < l; j++ {
			s += math.Abs(samples[i] - samples[j])
		}
		distSlice = append(distSlice, s/N)
	}

	return
}

func Norm(samples []float64) float64 {
	var norm float64
	for _, s := range samples {
		norm += math.Abs(s) * math.Abs(s)
	}
	return math.Sqrt(norm)
}

// pushIndexMap is a helper for @flattenSlice that updates the index memory map.
func pushIndexMap(indexMap map[int]int, index int) map[int]int {
	for i := 0; i < len(indexMap); i++ {
		if i >= index && i < len(indexMap)-1 {
			indexMap[i] = indexMap[i+1]
		}
	}
	delete(indexMap, len(indexMap)-1)
	return indexMap
}

// removeIndex removes entry at @index from @samples.
func removeIndex(samples []float64, index int) []float64 {
	return append(samples[:index], samples[index+1:]...)
}

// max returns the maximal value of the non-negative slice @samples.
func max(samples []float64) (maximum float64, index int) {
	for i, s := range samples {
		if s > maximum {
			maximum = s
			index = i
		}
	}
	return
}
