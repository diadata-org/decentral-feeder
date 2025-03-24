package models

import (
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	luminametacontract "github.com/diadata-org/decentral-feeder/pkg/contracts/lumina/metacontract"
	uniswap "github.com/diadata-org/decentral-feeder/pkg/contracts/uniswap/pair"
	"github.com/diadata-org/decentral-feeder/pkg/utils"
)

// Asset is the data type for all assets, ranging from fiat to crypto.
type Asset struct {
	Symbol     string `json:"Symbol"`
	Name       string `json:"Name"`
	Address    string `json:"Address"`
	Decimals   uint8  `json:"Decimals"`
	Blockchain string `json:"Blockchain"`
}

func (a *Asset) AssetIdentifier() string {
	return a.Blockchain + "-" + a.Address
}

// GetOnchainPrice returns the latest price of asset @a as published by DIA metacontract with @address.
func (a *Asset) GetOnchainPrice(metacontractAddress common.Address, precision int, client *ethclient.Client) (assetQuotation AssetQuotation, err error) {
	var caller *luminametacontract.LuminametacontractCaller

	caller, err = luminametacontract.NewLuminametacontractCaller(metacontractAddress, client)
	if err != nil {
		return
	}
	priceBig, timeUnixBig, err := caller.GetValue(&bind.CallOpts{}, a.Symbol+"/USD")
	if err != nil {
		return
	}
	assetQuotation.Time = time.Unix(timeUnixBig.Int64(), 0)
	assetQuotation.Price, _ = new(big.Float).Mul(new(big.Float).SetInt(priceBig), big.NewFloat(math.Pow10(-precision))).Float64()
	assetQuotation.Asset = *a
	assetQuotation.Source = "DIA_Lumina_" + metacontractAddress.Hex()

	return
}

func GetAsset(address common.Address, blockchain string, client *ethclient.Client) (asset Asset, err error) {
	var contract *uniswap.IERC20Caller
	contract, err = uniswap.NewIERC20Caller(address, client)
	if err != nil {
		log.Error("NewIERC20Caller: ", err)
		return
	}

	asset.Symbol, err = contract.Symbol(&bind.CallOpts{})
	if err != nil {
		log.Errorf("Get Symbol from on-chain for address %s: %v", address, err)
		return
	}
	asset.Name, err = contract.Name(&bind.CallOpts{})
	if err != nil {
		log.Warnf("Get Name from on-chain for address %s: %v", address, err)
	}
	asset.Decimals, err = contract.Decimals(&bind.CallOpts{})
	if err != nil {
		log.Errorf("Get Decimals from on-chain for address %s: %v", address, err)
		return
	}
	asset.Address = address.Hex()
	asset.Blockchain = blockchain

	return
}

// getBalance returns the balance of @a in pool with address @poolAddress.
func (a *Asset) GetBalance(poolAddress common.Address, client *ethclient.Client) (float64, error) {
	balanceBig, err := utils.GetBalanceOf(common.HexToAddress(a.Address), poolAddress, client)
	if err != nil {
		return 0, err
	}
	balance, _ := new(big.Float).Quo(big.NewFloat(0).SetInt(balanceBig), new(big.Float).SetFloat64(math.Pow10(int(a.Decimals)))).Float64()
	return balance, nil
}
