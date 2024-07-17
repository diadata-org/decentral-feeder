package models

import (
	"testing"
	"time"
)

func testGetLastTrade(t *testing.T) {
	cases := []struct {
		trades    []Trade
		timestamp time.Time
		lastTrade Trade
	}{
		{
			trades: []Trade{
				{
					Time: time.Unix(),
				},
				{},
			},
		},
		{},
	}

	for i, c := range cases {
		lastTrade := GetLastTrade(c.trades, c.timestamp)
	}
}
