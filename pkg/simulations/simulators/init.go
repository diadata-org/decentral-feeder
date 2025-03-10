package simulators

import (
	models "github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
	"github.com/sirupsen/logrus"
)

const (
	UNISWAP_SIMULATION = "UniswapSimulation"
)

var (
	Exchanges = make(map[string]models.Exchange)
	log       *logrus.Logger
)

func init() {

	Exchanges[UNISWAP_SIMULATION] = models.Exchange{Name: UNISWAP_SIMULATION, Centralized: false, Simulation: true, Blockchain: utils.ETHEREUM}

	log = logrus.New()
	loglevel, err := logrus.ParseLevel(utils.Getenv("LOG_LEVEL_SCRAPERS", "info"))
	if err != nil {
		log.Errorf("Parse log level: %v.", err)
	}
	log.SetLevel(loglevel)

}
