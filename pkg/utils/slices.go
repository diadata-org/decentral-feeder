package utils

import "github.com/ethereum/go-ethereum/common"

func ContainsAddress(addresses []common.Address, address common.Address) (contains bool) {
	for _, a := range addresses {
		if address == a {
			contains = true
			return
		}
	}
	return
}
