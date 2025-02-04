package simulators

import "testing"

var tokenInDecimal uint8 = 6

func Test1(t *testing.T) {

	tests := []struct {
		name    string
		decimal int8
		events  []SwapEvents
		want    float64
	}{
		// {
		// 	name:    "btc test",
		// 	decimal: 8,
		// 	want:    float64(64412.653222),
		// 	events: []SwapEvents{
		// 		{
		// 			Amount0In:  0,
		// 			Amount0Out: 155249,
		// 			Amount1In:  100000000,
		// 			Amount1Out: 0,
		// 		},
		// 	},
		// },
		// {
		// 	name:    "eth test",
		// 	decimal: 18,
		// 	want:    float64(3191.916321),
		// 	events: []SwapEvents{
		// 		{
		// 			Amount0In:  100000000,
		// 			Amount0Out: 0,
		// 			Amount1In:  0,
		// 			Amount1Out: 31329142104179192,
		// 		},
		// 	},
		// },

		// {
		// 	name:    "dia test",
		// 	decimal: 18,
		// 	want:    float64(3191.916321),
		// 	events: []SwapEvents{
		// 		{
		// 			Amount0In:  0,
		// 			Amount0Out: -2714460419641193305,
		// 			Amount1In:  1000000000,
		// 			Amount1Out: 0,
		// 		},
		// 	},
		// },
		// {
		// 	name:    "dia test2",
		// 	decimal: 18,
		// 	want:    float64(3191.916321),
		// 	events: []SwapEvents{
		// 		{
		// 			Amount0In:  1000000000,
		// 			Amount0Out: 0,
		// 			Amount1In:  0,
		// 			Amount1Out: 312959527487065911,
		// 		},
		// 		{
		// 			Amount0In:  0,
		// 			Amount0Out: 4802307714900534001,
		// 			Amount1In:  312959527487065911,
		// 			Amount1Out: 0,
		// 		},
		// 	},
		// },

		// {
		// 	name:    "uni test",
		// 	decimal: 18,
		// 	want:    float64(315.442822),
		// 	events: []SwapEvents{
		// 		{
		// 			Amount0In:  1000000000,
		// 			Amount0Out: 0,
		// 			Amount1In:  0,
		// 			Amount1Out: 294117507868533908,
		// 		},
		// 		{
		// 			Amount0In:  0,
		// 			Amount0Out: 3170146627780400005,
		// 			Amount1In:  294117507868533908,
		// 			Amount1Out: 0,
		// 		},
		// 	},
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := getSimulationSwapData(tt.events, tokenInDecimal, uint8(tt.decimal)); got != tt.want {
				t.Errorf("CalculatePrice() = %f, want %f", got, tt.want)
			}
		})
	}

}
