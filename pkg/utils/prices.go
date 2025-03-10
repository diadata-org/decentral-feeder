package utils

import "encoding/json"

func GetPriceFromDiaAPI(address string, blockchain string) (float64, error) {
	baseString := "https://api.diadata.org/v1/assetQuotation/" + blockchain + "/" + address
	response, _, err := GetRequest(baseString)
	if err != nil {
		return 0, err
	}
	type assetQuotation struct {
		Price  float64 `json:"Price"`
		Volume float64 `json:"VolumeYesterdayUSD"`
	}
	var aq assetQuotation
	err = json.Unmarshal(response, &aq)
	if err != nil {
		return 0, err
	}
	return aq.Price, nil
}
