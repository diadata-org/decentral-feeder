package simulation

import (
	"github.com/diadata-org/decentral-feeder/pkg/models"
)

type Simulator interface {
	Execute(t1 models.Asset, t2 models.Asset) (string, error)
}
