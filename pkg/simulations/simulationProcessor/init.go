package simulationprocessor

import (
	"flag"
	"strconv"

	"github.com/diadata-org/decentral-feeder/pkg/utils"
	"github.com/sirupsen/logrus"
)

var (
	toleranceSeconds int64
	log              *logrus.Logger

	filterType     = utils.Getenv("FILTER_TYPE_SIMULATION", "LastPrice")
	metaFilterType = utils.Getenv("METAFILTER_TYPE_SIMULATION", "Median")
)

func init() {
	var err error
	flag.Parse()
	log = logrus.New()
	loglevel, err := logrus.ParseLevel(utils.Getenv("LOG_LEVEL_SIMLUATION_PROCESSOR", "info"))
	if err != nil {
		log.Errorf("Parse log level: %v.", err)
	}
	log.SetLevel(loglevel)

	toleranceSeconds, err = strconv.ParseInt(utils.Getenv("SIMULATION_TOLERANCE_SECONDS", "120"), 10, 64)
	if err != nil {
		log.Errorf("Parse SIMLUATION_TOLERANCE_SECONDS environment variable: %v.", err)
	}

}
