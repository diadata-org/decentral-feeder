package processor

import (
	"flag"
	"strconv"

	"github.com/diadata-org/decentral-feeder/pkg/models"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
	"github.com/sirupsen/logrus"
)

// For processing, all filters with timestamp older than time.Now()-toleranceSeconds are discarded.
var (
	toleranceSeconds int64
	log              *logrus.Logger
	filterType       = utils.Getenv("FILTER_TYPE", string(FILTER_LAST_PRICE))
	metaFilterType   = utils.Getenv("METAFILTER_TYPE", string(METAFILTER_MEDIAN))

	FILTER_LAST_PRICE = models.FilterType("LastPrice")
	METAFILTER_MEDIAN = models.MetafilterType("Median")
)

func init() {
	var err error
	flag.Parse()
	log = logrus.New()
	loglevel, err := logrus.ParseLevel(utils.Getenv("LOG_LEVEL_PROCESSOR", "info"))
	if err != nil {
		log.Errorf("Parse log level: %v.", err)
	}
	log.SetLevel(loglevel)

	toleranceSeconds, err = strconv.ParseInt(utils.Getenv("TOLERANCE_SECONDS", "20"), 10, 64)
	if err != nil {
		log.Errorf("Parse TOLERANCE_SECONDS environment variable: %v.", err)
	}

}
