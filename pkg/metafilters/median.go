package metafilters

import (
	models "github.com/diadata-org/decentral-feeder/pkg/models"
	utils "github.com/diadata-org/decentral-feeder/pkg/utils"
)

const (
	medianFilterName = "median"
)

func Median(filterPoints []models.FilterPointExtended) (medianizedFilterPoints []models.FilterPointExtended) {
	filterAssetMap := models.GroupFilterByAsset(filterPoints)
	for asset, filters := range filterAssetMap {
		filterValue := utils.Median(models.GetValuesFromFilterPoints(filters))
		var fp models.FilterPointExtended
		fp.Value = filterValue
		fp.Pair.QuoteToken = asset
		fp.Name = medianFilterName
		medianizedFilterPoints = append(medianizedFilterPoints, fp)
	}
	return
}
