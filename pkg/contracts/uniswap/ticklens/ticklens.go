// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package ticklens

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// ITickLensPopulatedTick is an auto generated low-level Go binding around an user-defined struct.
type ITickLensPopulatedTick struct {
	Tick           *big.Int
	LiquidityNet   *big.Int
	LiquidityGross *big.Int
}

// TicklensMetaData contains all meta data concerning the Ticklens contract.
var TicklensMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"pool\",\"type\":\"address\"},{\"internalType\":\"int16\",\"name\":\"tickBitmapIndex\",\"type\":\"int16\"}],\"name\":\"getPopulatedTicksInWord\",\"outputs\":[{\"components\":[{\"internalType\":\"int24\",\"name\":\"tick\",\"type\":\"int24\"},{\"internalType\":\"int128\",\"name\":\"liquidityNet\",\"type\":\"int128\"},{\"internalType\":\"uint128\",\"name\":\"liquidityGross\",\"type\":\"uint128\"}],\"internalType\":\"structITickLens.PopulatedTick[]\",\"name\":\"populatedTicks\",\"type\":\"tuple[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// TicklensABI is the input ABI used to generate the binding from.
// Deprecated: Use TicklensMetaData.ABI instead.
var TicklensABI = TicklensMetaData.ABI

// Ticklens is an auto generated Go binding around an Ethereum contract.
type Ticklens struct {
	TicklensCaller     // Read-only binding to the contract
	TicklensTransactor // Write-only binding to the contract
	TicklensFilterer   // Log filterer for contract events
}

// TicklensCaller is an auto generated read-only Go binding around an Ethereum contract.
type TicklensCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TicklensTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TicklensTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TicklensFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type TicklensFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TicklensSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TicklensSession struct {
	Contract     *Ticklens         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TicklensCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TicklensCallerSession struct {
	Contract *TicklensCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// TicklensTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TicklensTransactorSession struct {
	Contract     *TicklensTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// TicklensRaw is an auto generated low-level Go binding around an Ethereum contract.
type TicklensRaw struct {
	Contract *Ticklens // Generic contract binding to access the raw methods on
}

// TicklensCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TicklensCallerRaw struct {
	Contract *TicklensCaller // Generic read-only contract binding to access the raw methods on
}

// TicklensTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TicklensTransactorRaw struct {
	Contract *TicklensTransactor // Generic write-only contract binding to access the raw methods on
}

// NewTicklens creates a new instance of Ticklens, bound to a specific deployed contract.
func NewTicklens(address common.Address, backend bind.ContractBackend) (*Ticklens, error) {
	contract, err := bindTicklens(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Ticklens{TicklensCaller: TicklensCaller{contract: contract}, TicklensTransactor: TicklensTransactor{contract: contract}, TicklensFilterer: TicklensFilterer{contract: contract}}, nil
}

// NewTicklensCaller creates a new read-only instance of Ticklens, bound to a specific deployed contract.
func NewTicklensCaller(address common.Address, caller bind.ContractCaller) (*TicklensCaller, error) {
	contract, err := bindTicklens(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &TicklensCaller{contract: contract}, nil
}

// NewTicklensTransactor creates a new write-only instance of Ticklens, bound to a specific deployed contract.
func NewTicklensTransactor(address common.Address, transactor bind.ContractTransactor) (*TicklensTransactor, error) {
	contract, err := bindTicklens(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &TicklensTransactor{contract: contract}, nil
}

// NewTicklensFilterer creates a new log filterer instance of Ticklens, bound to a specific deployed contract.
func NewTicklensFilterer(address common.Address, filterer bind.ContractFilterer) (*TicklensFilterer, error) {
	contract, err := bindTicklens(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &TicklensFilterer{contract: contract}, nil
}

// bindTicklens binds a generic wrapper to an already deployed contract.
func bindTicklens(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := TicklensMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Ticklens *TicklensRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Ticklens.Contract.TicklensCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Ticklens *TicklensRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Ticklens.Contract.TicklensTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Ticklens *TicklensRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Ticklens.Contract.TicklensTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Ticklens *TicklensCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Ticklens.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Ticklens *TicklensTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Ticklens.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Ticklens *TicklensTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Ticklens.Contract.contract.Transact(opts, method, params...)
}

// GetPopulatedTicksInWord is a free data retrieval call binding the contract method 0x351fb478.
//
// Solidity: function getPopulatedTicksInWord(address pool, int16 tickBitmapIndex) view returns((int24,int128,uint128)[] populatedTicks)
func (_Ticklens *TicklensCaller) GetPopulatedTicksInWord(opts *bind.CallOpts, pool common.Address, tickBitmapIndex int16) ([]ITickLensPopulatedTick, error) {
	var out []interface{}
	err := _Ticklens.contract.Call(opts, &out, "getPopulatedTicksInWord", pool, tickBitmapIndex)

	if err != nil {
		return *new([]ITickLensPopulatedTick), err
	}

	out0 := *abi.ConvertType(out[0], new([]ITickLensPopulatedTick)).(*[]ITickLensPopulatedTick)

	return out0, err

}

// GetPopulatedTicksInWord is a free data retrieval call binding the contract method 0x351fb478.
//
// Solidity: function getPopulatedTicksInWord(address pool, int16 tickBitmapIndex) view returns((int24,int128,uint128)[] populatedTicks)
func (_Ticklens *TicklensSession) GetPopulatedTicksInWord(pool common.Address, tickBitmapIndex int16) ([]ITickLensPopulatedTick, error) {
	return _Ticklens.Contract.GetPopulatedTicksInWord(&_Ticklens.CallOpts, pool, tickBitmapIndex)
}

// GetPopulatedTicksInWord is a free data retrieval call binding the contract method 0x351fb478.
//
// Solidity: function getPopulatedTicksInWord(address pool, int16 tickBitmapIndex) view returns((int24,int128,uint128)[] populatedTicks)
func (_Ticklens *TicklensCallerSession) GetPopulatedTicksInWord(pool common.Address, tickBitmapIndex int16) ([]ITickLensPopulatedTick, error) {
	return _Ticklens.Contract.GetPopulatedTicksInWord(&_Ticklens.CallOpts, pool, tickBitmapIndex)
}
