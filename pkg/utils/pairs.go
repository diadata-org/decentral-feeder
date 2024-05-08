package utils

import "github.com/diadata-org/diaprotocol/pkg/models"

// MakeExchangepairMap returns a map in which exchangepairs are grouped by exchange string key.
func MakeExchangepairMap(exchangePairs []models.ExchangePair) map[string][]models.ExchangePair {
	exchangepairMap := make(map[string][]models.ExchangePair)
	for _, ep := range exchangePairs {
		exchangepairMap[ep.Exchange] = append(exchangepairMap[ep.Exchange], ep)
	}
	return exchangepairMap
}
