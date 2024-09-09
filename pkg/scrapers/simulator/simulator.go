package simulation

import (
	"context"
	"math"
	"math/big"

	coreEntities "github.com/daoleno/uniswap-sdk-core/entities"
	"github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/sirupsen/logrus"

	"github.com/daoleno/uniswapv3-sdk/examples/contract"
	"github.com/daoleno/uniswapv3-sdk/examples/helper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	SlippageAmount = 3000

	MainnetChainId = 1
)

var (
	SwapRouter = common.HexToAddress(helper.ContractV3SwapRouterV1)
)

type Simulator struct {
	Eth *ethclient.Client
	log *logrus.Logger
}

func New(client *ethclient.Client, log *logrus.Logger) *Simulator {
	c := Simulator{Eth: client, log: log}
	return &c

}

func (c *Simulator) Execute(t1 models.Asset, t2 models.Asset) (string, error) {

	token1 := coreEntities.NewToken(1, common.HexToAddress(t1.Address), uint(t1.Decimals), t1.Name, t1.Name)

	token2 := coreEntities.NewToken(1, common.HexToAddress(t2.Address), uint(t2.Decimals), t2.Name, t2.Name)

	return c.quoteTokens(context.TODO(), "1000", token2, token1)

}

func (c *Simulator) quoteTokens(ctx context.Context, input string, token0 *coreEntities.Token, token1 *coreEntities.Token) (string, error) {
	quoterContract, err := contract.NewUniswapv3Quoter(common.HexToAddress(helper.ContractV3Quoter), c.Eth)
	if err != nil {
		c.log.Errorln("failed to create quoter contract")
		return "", err
	}
	// 0.03% slippage
	fee := big.NewInt(SlippageAmount)

	amountIn := helper.FloatStringToBigInt(input, int(token0.Decimals()))
	sqrtPriceLimitX96 := big.NewInt(0)

	var out []interface{}

	rawCaller := &contract.Uniswapv3QuoterRaw{Contract: quoterContract}

	err = rawCaller.Call(nil, &out, "quoteExactInputSingle", token0.Address, token1.Address,
		fee, amountIn, sqrtPriceLimitX96)
	if err != nil {
		c.log.Errorln("failed to call quoteExactInputSingle")
		return "", err
	}

	c.log.Debugf("Quote: input: %s, output: %s", input, out[0].(*big.Int).String())

	return CurrencyToString(out[0].(*big.Int), int(token1.Decimals())), nil
}

func CurrencyToString(units *big.Int, decimals int) string {
	w := new(big.Float).SetInt(units)
	w = new(big.Float).Quo(w, big.NewFloat(math.Pow10(decimals)))
	return w.String()
}
