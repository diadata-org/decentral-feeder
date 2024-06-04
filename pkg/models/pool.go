package models

import (
	"time"

	"github.com/diadata-org/decentral-feeder/pkg/utils"
	"github.com/tkanos/gonfig"
)

// Pool is the container for liquidity pools on DEXes.
type Pool struct {
	Exchange     Exchange      `json:"Exchange"`
	Blockchain   Blockchain    `json:"Blockchain"`
	Address      string        `json:"Address"`
	Assetvolumes []AssetVolume `json:"Assetvolumes"`
	Time         time.Time     `json:"Time"`
}

type AssetVolume struct {
	Asset  Asset   `json:"Asset"`
	Volume float64 `json:"Volume"`
	Index  uint8   `json:"Index"`
}

func MakePoolMap(pools []Pool) map[string][]Pool {
	poolMap := make(map[string][]Pool)
	for _, pool := range pools {
		poolMap[pool.Exchange.Name] = append(poolMap[pool.Exchange.Name], pool)
	}
	return poolMap
}

func GetPoolsFromConfig(exchange string) ([]Pool, error) {
	path := utils.GetPath("pools/", exchange)
	type Pools struct {
		Pools []Pool
	}
	var p Pools
	err := gonfig.GetConf(path, &p)
	if err != nil {
		return []Pool{}, err
	}
	return p.Pools, nil
}
