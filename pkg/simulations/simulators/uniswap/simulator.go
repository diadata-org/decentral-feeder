package uniswap

import (
	"math"
	"math/big"

	coreEntities "github.com/daoleno/uniswap-sdk-core/entities"
	"github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/sirupsen/logrus"

	"github.com/daoleno/uniswapv3-sdk/examples/contract"
	"github.com/daoleno/uniswapv3-sdk/examples/helper"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
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

func (c *Simulator) Execute(t1 models.Asset, t2 models.Asset, amountIn string, fees *big.Int) (string, error) {

	token1 := coreEntities.NewToken(1, common.HexToAddress(t1.Address), uint(t1.Decimals), t1.Name, t1.Name)

	token2 := coreEntities.NewToken(1, common.HexToAddress(t2.Address), uint(t2.Decimals), t2.Name, t2.Name)

	return c.quoteTokens(amountIn, token2, token1, fees)

}

func (c *Simulator) quoteTokens(input string, token0 *coreEntities.Token, token1 *coreEntities.Token, fees *big.Int) (string, error) {
	quoterContract, err := contract.NewUniswapv3Quoter(common.HexToAddress(helper.ContractV3Quoter), c.Eth)
	if err != nil {
		c.log.Errorln("failed to create quoter contract")
		return "", err
	}

	amountIn := helper.FloatStringToBigInt(input, int(token0.Decimals()))
	sqrtPriceLimitX96 := big.NewInt(0)

	var out []interface{}

	rawCaller := &contract.Uniswapv3QuoterRaw{Contract: quoterContract}

	err = rawCaller.Call(&bind.CallOpts{}, &out, "quoteExactInputSingle", token0.Address, token1.Address,
		fees, amountIn, sqrtPriceLimitX96)
	if err != nil {
		c.log.Errorf("Failed to call quoteExactInputSingle for fee %s in pool %s-%s: %v", fees.String(), token0.Symbol(), token1.Symbol(), err)
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
