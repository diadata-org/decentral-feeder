package processor

import (
	"strconv"

	"github.com/diadata-org/decentral-feeder/pkg/utils"
	log "github.com/sirupsen/logrus"
)

// For processing, all filters with timestamp older than time.Now()-toleranceSeconds are discarded.
var toleranceSeconds int64

func init() {
	var err error
	toleranceSeconds, err = strconv.ParseInt(utils.Getenv("TOLERANCE_SECONDS", "20"), 10, 64)
	if err != nil {
		log.Error("Parse TOLERANCE_SECONDS environment variable: ", err)
	}
}
