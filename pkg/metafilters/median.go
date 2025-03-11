package metafilters

import (
	models "github.com/diadata-org/decentral-feeder/pkg/models"
	utils "github.com/diadata-org/decentral-feeder/pkg/utils"
)

const (
	medianFilterName = "median"
)

// Median returns the median value for all filter points that share the same quote asset.
// The input @filterPoints still consists of "atomic" filter points.
func Median(filterPoints []models.FilterPointPair) (medianizedFilterPoints []models.FilterPointPair) {
	filterAssetMap := models.GroupFiltersByAsset(filterPoints)

	for asset, filters := range filterAssetMap {
		filterValue := utils.Median(models.GetValuesFromFilterPoints(filters))
		var fp models.FilterPointPair
		fp.Value = filterValue
		fp.Pair.QuoteToken = asset
		fp.Name = medianFilterName
		fp.Time = models.GetLatestTimestampFromFilterPoints(filters)
		if len(filters) > 0 {
			fp.SourceType = filters[0].SourceType
		}
		medianizedFilterPoints = append(medianizedFilterPoints, fp)
	}

	return
}
