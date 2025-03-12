package route

import (
	"math"
	"math/big"
	"sort"
	"strings"

	"github.com/nnn-gif/curvesimulator/pool"
	"github.com/nnn-gif/curvesimulator/pool2"
	"github.com/nnn-gif/curvesimulator/pool3"
	"github.com/nnn-gif/curvesimulator/pool4"
	"github.com/nnn-gif/curvesimulator/pool5"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type NativeToken struct {
	Address        string
	WrappedAddress string
}
type IPoolData struct {
	PoolID                  string
	SwapAddress             string
	TokenAddress            string
	WrappedCoinAddresses    []string
	UnderlyingCoinAddresses []string
	IsLending               bool
	IsCrypto                bool
	IsMeta                  bool
	IsLLAMMA                bool
	IsFake                  bool
	IsFactory               bool
	IsNG                    bool
}

type RouteStep struct {
	PoolID            string
	SwapAddress       string
	InputCoinAddress  string
	OutputCoinAddress string
	SwapParams        [5]int

	PoolAddress     string
	BasePool        string
	BaseToken       string
	SecondBasePool  string
	SecondBaseToken string

	TVL float64
}
type RouteGraph = map[string]map[string][]RouteStep

type Route = []RouteStep

type CurveConstants struct {
	ZERO_ADDRESS string
	NATIVE_TOKEN NativeToken
}

type RouteGraphInput struct {
	Constants                *CurveConstants
	ChainID                  int
	IsLiteChain              bool
	AllPools                 []IPoolData
	AmplificationCoefficient map[string]float64
	PoolTvlDict              map[string]float64
}

type CurvePool struct {
	Address common.Address

	Name     string
	Tokens   []string
	Balances map[string]*big.Int
	Fee      *big.Float
}

func CreateRouteGraph(input RouteGraphInput) RouteGraph {
	routeGraph := make(RouteGraph)
	const GRAPH_MAX_EDGES = 3

	// start := time.Now()

	sortEdges := func(inCoin, outCoin string) {
		edges := routeGraph[inCoin][outCoin]
		sort.Slice(edges, func(i, j int) bool {
			return edges[i].TVL > edges[j].TVL
		})
		if len(edges) > GRAPH_MAX_EDGES {
			edges = edges[:GRAPH_MAX_EDGES]
		}
		routeGraph[inCoin][outCoin] = edges
	}

	addEdge := func(inCoin, outCoin string, step RouteStep) {
		if routeGraph[inCoin] == nil {
			routeGraph[inCoin] = make(map[string][]RouteStep)
		}
		routeGraph[inCoin][outCoin] = append(routeGraph[inCoin][outCoin], step)
		sortEdges(inCoin, outCoin)
	}

	for _, pool := range input.AllPools {
		poolID := pool.PoolID
		swapAddr := strings.ToLower(pool.SwapAddress)
		wrappedCoins := lowerCaseAll(pool.WrappedCoinAddresses)
		nCoins := len(wrappedCoins)

		var poolType int
		switch {
		case pool.IsLLAMMA:
			poolType = 4
		case pool.IsCrypto:
			if nCoins == 2 {
				poolType = 2
			} else if nCoins == 3 {
				poolType = 3 // triCrypto
			} else {
				poolType = 2 // fallback
			}
		default:
			poolType = 1 // stable
		}
		if pool.IsNG {
			poolType *= 10
		}

		tvl := input.PoolTvlDict[poolID]
		amp := input.AmplificationCoefficient[pool.SwapAddress]
		if !pool.IsCrypto {
			tvl *= amp
		}
		if input.IsLiteChain && tvl < 100.0 {
			// skip tiny pools
			continue
		}

		if !pool.IsFake {
			for i := 0; i < nCoins; i++ {
				for j := 0; j < nCoins; j++ {
					if i == j {
						continue
					}
					step := RouteStep{
						PoolID:            poolID,
						SwapAddress:       swapAddr,
						InputCoinAddress:  wrappedCoins[i],
						OutputCoinAddress: wrappedCoins[j],
						SwapParams:        [5]int{i, j, 1, poolType, nCoins}, // 1 => normal exchange
						TVL:               tvl,
					}
					addEdge(wrappedCoins[i], wrappedCoins[j], step)
				}
			}
		}

	}

	// elapsed := time.Since(start)
	// fmt.Printf("Processed %d pools in %v, routeGraph size: %d tokens.\n",
	// 	len(input.AllPools), elapsed, len(routeGraph))

	return routeGraph
}

func FindAllRoutes(
	graph RouteGraph,
	startCoin, endCoin string,
	maxDepth, maxPaths int,
) []Route {
	var results []Route

	queue := []Route{
		{{
			PoolID:            "DUMMY",
			InputCoinAddress:  startCoin,
			OutputCoinAddress: startCoin,
		}},
	}

	for len(queue) > 0 && len(results) < maxPaths {

		current := queue[0]
		queue = queue[1:]

		lastCoin := current[len(current)-1].OutputCoinAddress
		if lastCoin == endCoin {
			routeCopy := make(Route, len(current)-1)
			copy(routeCopy, current[1:])
			results = append(results, routeCopy)
			continue
		}

		if len(current)-1 >= maxDepth {
			continue
		}

		edgesMap := graph[lastCoin]
		if edgesMap == nil {
			continue
		}

		for _, steps := range edgesMap {

			for _, step := range steps {
				newRoute := make(Route, len(current))
				copy(newRoute, current)
				newRoute = append(newRoute, step)
				queue = append(queue, newRoute)
			}
		}
	}

	return results
}

func visitedCoin(route Route, coin string) bool {
	for _, step := range route {
		if step.OutputCoinAddress == coin {
			return true
		}
	}
	return false
}

func simulateRoute(route Route, amountIn float64) float64 {
	current := amountIn
	for _, step := range route {

		slippage := 0.002
		ratio := 1.0 - slippage*(1.0/(1.0+step.TVL/1e6))
		current = current * ratio
	}
	return current
}

func PickBestRoute(routes []Route, amountIn float64, gasPriceGwei float64, ethPriceUsd float64) (Route, float64) {
	bestValue := -math.MaxFloat64
	var bestRoute Route
	var bestOut float64

	// type RouteStep struct {
	// 	PoolID            string
	// 	SwapAddress       string
	// 	InputCoinAddress  string
	// 	OutputCoinAddress string
	// 	SwapParams        [5]int

	// 	PoolAddress     string
	// 	BasePool        string
	// 	BaseToken       string
	// 	SecondBasePool  string
	// 	SecondBaseToken string

	// 	TVL float64
	// }

	for _, r := range routes {
		out := simulateRoute(r, amountIn)
		outUsd := out * 1.0

		if r[0].SwapAddress == "0x118629329731ce1ff01f9401212c53939f8a9eeb" || r[0].SwapAddress == "0xbd917239ec067a6af82aabb6b45dad3d09f4efd8" || r[0].SwapAddress == "0xBD917239EC067A6aF82AABb6B45DAd3d09F4EFD8" {
			continue
		}

		steps := float64(len(r))
		gasUsed := steps * 100000
		costEth := gasUsed * (gasPriceGwei * 1e-9)
		costUsd := costEth * ethPriceUsd

		netUsd := outUsd - costUsd
		if netUsd > bestValue {
			bestValue = netUsd
			bestRoute = r
			bestOut = out
		}
	}

	return bestRoute, bestOut
}

func lowerCaseAll(list []string) []string {
	result := make([]string, len(list))
	for i, v := range list {
		result[i] = string(common.HexToAddress(v).Hex())
	}
	return result
}

func SimulateFullRouteWithGetDy(client *ethclient.Client, route []RouteStep, amountIn *big.Int) (*big.Int, error) {
	current := new(big.Int).Set(amountIn)

	for _, step := range route {
		poolAddr := common.HexToAddress(step.SwapAddress)

		// In real code, you'd do something like:

		poolCaller1, err := pool.NewPool(poolAddr, client)
		if err != nil {
			return nil, err
		}

		poolCaller2, err := pool2.NewPool2(poolAddr, client)
		if err != nil {
			return nil, err
		}

		poolCaller3, err := pool3.NewPool3(poolAddr, client)
		if err != nil {
			return nil, err
		}

		poolCaller4, err := pool4.NewPool4(poolAddr, client)
		if err != nil {
			return nil, err
		}
		poolCaller5, err := pool5.NewPool5(poolAddr, client)
		if err != nil {
			return nil, err
		}

		i, j := step.SwapParams[1], step.SwapParams[0]

		var out *big.Int

		out, err = poolCaller1.GetDy(&bind.CallOpts{}, big.NewInt(int64(i)), big.NewInt(int64(j)), current)
		if err != nil {

			out, err = poolCaller2.GetDy(&bind.CallOpts{}, big.NewInt(int64(i)), big.NewInt(int64(j)), current)
			if err != nil {

				out, err = poolCaller3.GetDy(&bind.CallOpts{}, big.NewInt(int64(i)), big.NewInt(int64(j)), current)
				if err != nil {

					out, err = poolCaller4.GetDy(&bind.CallOpts{}, big.NewInt(int64(i)), big.NewInt(int64(j)), current)
					if err != nil {

						out, err = poolCaller5.GetDy(&bind.CallOpts{}, big.NewInt(int64(i)), big.NewInt(int64(j)), current)

						if err != nil {
							return nil, err

						}

					}
				}
			}

		}
		// default:
		// 	out, err = poolCaller3.GetDy(&bind.CallOpts{}, big.NewInt(int64(i)), big.NewInt(int64(j)), current)
		// 	if err != nil {
		// 		return nil, err
		// 	}

		// For demonstration, we just do a mock:
		// out := new(big.Int).Div(current, big.NewInt(2)) // pretend we always get half for demonstration
		// fmt.Printf("Step %d => pool %s => in: %s => out: %s\n", idx, step.SwapAddress, current, out)
		current.Set(out)
	}

	return current, nil
}