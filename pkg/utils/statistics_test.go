package utils

import (
	"math"
	"testing"
)

// TO DO: Write more test cases.

func TestMedian(t *testing.T) {
	cases := []struct {
		samples []float64
		median  float64
	}{
		{
			[]float64{21.2415, 3.4421, 24.1490, 71.1216, 19.47, 11.3313, 24.77809, 13.3166, 22.9814},
			21.2415,
		},
		{
			[]float64{3.31, 2.33, 9.01, 3.24, 1.53, 1.14},
			2.785,
		},
		{
			[]float64{1},
			1.0,
		},
		{
			[]float64{},
			0.0,
		},
	}

	for i, c := range cases {
		median := Median(c.samples)
		if math.Abs(float64(median-c.median)) > 1e-4 {
			t.Errorf("Median was incorrect, got: %f, expected: %f for set:%d", median, c.median, i)
		}
	}

}
