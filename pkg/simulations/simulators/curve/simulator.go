package simulator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"

	"github.com/nnn-gif/curvesimulator/curveregistry"
	"github.com/nnn-gif/curvesimulator/route"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	DefaultCurveRegistryAddr = common.HexToAddress("0xF98B45FA17DE75FB1aD0e7aFD971b0ca00e379fC")
	DefaultRPCURL            = "https://eth-mainnet.g.alchemy.com/v2/ylUVvdvrP5OR_liUKykoj8jI9DyKKkL7"
)

type CurveSimulation struct {
	rpcURL          string
	registryAddress common.Address

	client           *ethclient.Client
	registryContract *curveregistry.Curveregistry

	ampDict map[string]float64
	tvlDict map[string]float64

	constants *route.CurveConstants
}

func New() (*CurveSimulation, error) {
	client, err := ethclient.Dial(DefaultRPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	registryContract, err := curveregistry.NewCurveregistry(DefaultCurveRegistryAddr, client)
	if err != nil {
		// Make sure to close the client to prevent resource leaks
		client.Close()
		return nil, fmt.Errorf("failed to bind registry contract: %w", err)
	}

	cs := &CurveSimulation{
		rpcURL:           DefaultRPCURL,
		registryAddress:  DefaultCurveRegistryAddr,
		client:           client,
		registryContract: registryContract,
		ampDict: map[string]float64{
			"0x1111111111111111111111111111111111111111": 100,
			"0x2222222222222222222222222222222222222222": 50,
		},
		tvlDict: map[string]float64{
			"poolA": 50000,
			"poolB": 1500000,
		},
		constants: &route.CurveConstants{
			ZERO_ADDRESS: "0x0000000000000000000000000000000000000000",
			NATIVE_TOKEN: route.NativeToken{
				Address:        "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE",
				WrappedAddress: "0xC02aaa39b223Fe8D0A0e5C4F27eAD9083C756Cc2",
			},
		},
	}

	return cs, nil
}

func (cs *CurveSimulation) Close() {
	if cs.client != nil {
		cs.client.Close()
	}
}

func (cs *CurveSimulation) Simulate(startCoin, endCoin string, amountIn *big.Int) (*big.Int, error) {

	endCoin = common.HexToAddress(endCoin).Hex()
	startCoin = common.HexToAddress(startCoin).Hex()

	allPools, err := cs.fetchPools(startCoin, endCoin)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pools: %w", err)
	}

	graphInput := route.RouteGraphInput{
		Constants:                cs.constants,
		ChainID:                  1,
		IsLiteChain:              false,
		AllPools:                 allPools,
		AmplificationCoefficient: cs.ampDict,
		PoolTvlDict:              cs.tvlDict,
	}

	graph := route.CreateRouteGraph(graphInput)

	// edgesMap := graph["0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"]
	// if edgesMap == nil {
	// 	fmt.Println("No edges from stETH??")
	// } else {
	// 	for outCoin, steps := range edgesMap {
	// 		fmt.Printf("Edge  -> %s\n", outCoin)
	// 		for _, s := range steps {
	// 			fmt.Printf("  Step: poolId=%s, swapParams=%v\n", s.PoolID, s.SwapParams)
	// 		}
	// 	}
	// }

	routes := route.FindAllRoutes(graph, startCoin, endCoin, 2, 20)
	if len(routes) == 0 {
		return nil, errors.New("no routes found")
	}

	inputAmount := 1000.0
	gasPriceGwei := 30.0
	ethPriceUsd := 1800.0

	bestRoute, _ := route.PickBestRoute(routes, inputAmount, gasPriceGwei, ethPriceUsd)
	if len(bestRoute) == 0 {
		return nil, errors.New("failed to pick a best route")
	}

	finalAmount, err := route.SimulateFullRouteWithGetDy(cs.client, bestRoute, amountIn)
	if err != nil {
		return nil, fmt.Errorf("failed to simulate route: %w", err)
	}

	return finalAmount, nil
}

func (cs *CurveSimulation) fetchPools(startCoin, endCoin string) ([]route.IPoolData, error) {
	pools, err := cs.registryContract.FindPoolsForCoins(
		&bind.CallOpts{Context: context.Background()},
		common.HexToAddress(startCoin),
		common.HexToAddress(endCoin),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find pools for coins: %w", err)
	}

	poolCount := len(pools)
	if poolCount == 0 {
		return nil, fmt.Errorf("no pools found for %s -> %s", startCoin, endCoin)
	}

	var allPools []route.IPoolData
	for i, poolAddr := range pools {
		coinAddrs, err := cs.registryContract.GetCoins(
			&bind.CallOpts{Context: context.Background()},
			poolAddr,
		)
		if err != nil {
			log.Printf("Error fetching coins for pool %s: %v\n", poolAddr.Hex(), err)
			continue
		}

		var wrappedCoins []string
		for _, caddr := range coinAddrs {
			if caddr == (common.Address{}) {
				continue
			}
			wrappedCoins = append(wrappedCoins, caddr.Hex())
		}

		if len(wrappedCoins) == 0 {
			continue
		}

		poolData := route.IPoolData{
			PoolID:               fmt.Sprintf("Pool-%d", i),
			SwapAddress:          poolAddr.Hex(),
			WrappedCoinAddresses: wrappedCoins,
		}
		allPools = append(allPools, poolData)
	}

	return allPools, nil
}