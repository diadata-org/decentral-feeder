package metafilters

import (
	"reflect"
	"testing"

	"github.com/diadata-org/decentral-feeder/pkg/models"
	utils "github.com/diadata-org/decentral-feeder/pkg/utils"
)

// TO DO: Write more test cases.
var (
	ETH  = models.Asset{Address: "0x0000000000000000000000000000000000000000", Blockchain: utils.ETHEREUM}
	BTC  = models.Asset{Address: "0x0000000000000000000000000000000000000000", Blockchain: utils.BITCOIN}
	USDC = models.Asset{Address: "", Blockchain: utils.ETHEREUM}
)

func TestMedian(t *testing.T) {
	cases := []struct {
		filterPoints           []models.FilterPointPair
		medianizedFilterPoints []models.FilterPointPair
	}{
		{
			[]models.FilterPointPair{
				{
					Pair:  models.Pair{QuoteToken: ETH, BaseToken: USDC},
					Value: 3388.34,
				},
				{
					Pair:  models.Pair{QuoteToken: ETH, BaseToken: USDC},
					Value: 3381.11,
				},
				{
					Pair:  models.Pair{QuoteToken: ETH, BaseToken: USDC},
					Value: 3179.78,
				},
			},
			[]models.FilterPointPair{
				{
					Pair:  models.Pair{QuoteToken: ETH},
					Value: 3381.11,
					Name:  "median",
				},
			},
		},

		{
			[]models.FilterPointPair{
				{
					Pair:  models.Pair{QuoteToken: ETH, BaseToken: USDC},
					Value: 3143.3,
				},
				{
					Pair:  models.Pair{QuoteToken: ETH, BaseToken: USDC},
					Value: 3281.11,
				},
				{
					Pair:  models.Pair{QuoteToken: BTC, BaseToken: USDC},
					Value: 62344.9,
				},
				{
					Pair:  models.Pair{QuoteToken: ETH, BaseToken: USDC},
					Value: 3179.78,
				},
			},
			[]models.FilterPointPair{
				{
					Pair:  models.Pair{QuoteToken: ETH},
					Value: 3179.78,
					Name:  "median",
				},
				{
					Pair:  models.Pair{QuoteToken: BTC},
					Value: 62344.9,
					Name:  "median",
				},
			},
		},

		{
			[]models.FilterPointPair{
				{
					Pair:  models.Pair{QuoteToken: ETH, BaseToken: USDC},
					Value: 3143.3,
				},
				{
					Pair:  models.Pair{QuoteToken: ETH, BaseToken: USDC},
					Value: 3281.11,
				},
				{
					Pair:  models.Pair{QuoteToken: ETH, BaseToken: USDC},
					Value: 3179.78,
				},
			},
			[]models.FilterPointPair{
				{
					Pair:  models.Pair{QuoteToken: ETH},
					Value: 3179.78,
					Name:  "median",
				},
			},
		},
	}

	for i, c := range cases {
		medianizedFilterPoints := Median(c.filterPoints)

		// Make maps from slices in order to deep compare.
		if !reflect.DeepEqual(models.GroupFiltersByAsset(medianizedFilterPoints), models.GroupFiltersByAsset(c.medianizedFilterPoints)) {
			t.Errorf("Median was incorrect, got: %v, expected: %v for set:%d", medianizedFilterPoints, c.medianizedFilterPoints, i)
		}

	}

}
