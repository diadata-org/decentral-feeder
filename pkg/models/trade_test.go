package models

import (
	"testing"
	"time"
)

func testGetLastTrade(t *testing.T) {
	cases := []struct {
		trades    []Trade
		lastTrade Trade
	}{
		{
			trades: []Trade{
				{
					Time: time.Unix(1721209858, 0),
				},
				{
					Time: time.Unix(1657961611, 0),
				},
				{
					Time: time.Unix(1689497611, 0),
				},
			},
			lastTrade: Trade{Time: time.Unix(1721209858, 0)},
		},
		{
			trades: []Trade{
				{
					Time: time.Unix(0, 0),
				},
			},
			lastTrade: Trade{Time: time.Unix(0, 0)},
		},
	}

	for i, c := range cases {
		lastTrade := GetLastTrade(c.trades)
		if lastTrade != c.lastTrade {
			t.Errorf("Trade was incorrect, got: %v, expected: %v for set:%d", lastTrade, c.lastTrade, i)
		}
	}
}
