package simulation

import (
	coreEntities "github.com/daoleno/uniswap-sdk-core/entities"
	"github.com/daoleno/uniswapv3-sdk/examples/helper"
	"github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/nnn-gif/curvesimulator/simulator"
	"github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type CurveSimulator struct {
	Eth *ethclient.Client
	log *logrus.Logger
	sim *simulator.CurveSimulation
}

func NewCurveSimulator(client *ethclient.Client, log *logrus.Logger) *CurveSimulator {
	sim, err := simulator.New()
	if err != nil {
		log.Fatalf("Error initializing CurveSimulation: %v", err)
	}
	c := CurveSimulator{Eth: client, log: log, sim: sim}
	defer sim.Close()

	return &c

}

func (c *CurveSimulator) Execute(t1 models.Asset, t2 models.Asset) (string, error) {
	c.log.Debugf("curve simulating asset %s to asset %s,", t1.Address, t2.Address)

	token1 := coreEntities.NewToken(1, common.HexToAddress(t1.Address), uint(t1.Decimals), t1.Name, t1.Name)

	token2 := coreEntities.NewToken(1, common.HexToAddress(t2.Address), uint(t2.Decimals), t2.Name, t2.Name)

	amountIn := helper.FloatStringToBigInt("1000", int(token2.Decimals()))

	finalop, err := c.sim.Simulate(t1.Address, t2.Address, amountIn)
	if err != nil {
		return "", err

	}

	return CurrencyToString(finalop, int(token1.Decimals())), err

}
