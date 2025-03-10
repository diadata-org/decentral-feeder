// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package luminametacontract

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

// LuminametacontractMetaData contains all meta data concerning the Luminametacontract contract.
var LuminametacontractMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOracleAddress\",\"type\":\"address\"}],\"name\":\"addOracle\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"key\",\"type\":\"string\"}],\"name\":\"getValue\",\"outputs\":[{\"internalType\":\"uint128\",\"name\":\"\",\"type\":\"uint128\"},{\"internalType\":\"uint128\",\"name\":\"\",\"type\":\"uint128\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"oracleToRemove\",\"type\":\"address\"}],\"name\":\"removeOracle\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newThreshold\",\"type\":\"uint256\"}],\"name\":\"setThreshold\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newTimeoutSeconds\",\"type\":\"uint256\"}],\"name\":\"setTimeoutSeconds\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// LuminametacontractABI is the input ABI used to generate the binding from.
// Deprecated: Use LuminametacontractMetaData.ABI instead.
var LuminametacontractABI = LuminametacontractMetaData.ABI

// Luminametacontract is an auto generated Go binding around an Ethereum contract.
type Luminametacontract struct {
	LuminametacontractCaller     // Read-only binding to the contract
	LuminametacontractTransactor // Write-only binding to the contract
	LuminametacontractFilterer   // Log filterer for contract events
}

// LuminametacontractCaller is an auto generated read-only Go binding around an Ethereum contract.
type LuminametacontractCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LuminametacontractTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LuminametacontractTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LuminametacontractFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type LuminametacontractFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LuminametacontractSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type LuminametacontractSession struct {
	Contract     *Luminametacontract // Generic contract binding to set the session for
	CallOpts     bind.CallOpts       // Call options to use throughout this session
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// LuminametacontractCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type LuminametacontractCallerSession struct {
	Contract *LuminametacontractCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts             // Call options to use throughout this session
}

// LuminametacontractTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type LuminametacontractTransactorSession struct {
	Contract     *LuminametacontractTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts             // Transaction auth options to use throughout this session
}

// LuminametacontractRaw is an auto generated low-level Go binding around an Ethereum contract.
type LuminametacontractRaw struct {
	Contract *Luminametacontract // Generic contract binding to access the raw methods on
}

// LuminametacontractCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type LuminametacontractCallerRaw struct {
	Contract *LuminametacontractCaller // Generic read-only contract binding to access the raw methods on
}

// LuminametacontractTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type LuminametacontractTransactorRaw struct {
	Contract *LuminametacontractTransactor // Generic write-only contract binding to access the raw methods on
}

// NewLuminametacontract creates a new instance of Luminametacontract, bound to a specific deployed contract.
func NewLuminametacontract(address common.Address, backend bind.ContractBackend) (*Luminametacontract, error) {
	contract, err := bindLuminametacontract(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Luminametacontract{LuminametacontractCaller: LuminametacontractCaller{contract: contract}, LuminametacontractTransactor: LuminametacontractTransactor{contract: contract}, LuminametacontractFilterer: LuminametacontractFilterer{contract: contract}}, nil
}

// NewLuminametacontractCaller creates a new read-only instance of Luminametacontract, bound to a specific deployed contract.
func NewLuminametacontractCaller(address common.Address, caller bind.ContractCaller) (*LuminametacontractCaller, error) {
	contract, err := bindLuminametacontract(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &LuminametacontractCaller{contract: contract}, nil
}

// NewLuminametacontractTransactor creates a new write-only instance of Luminametacontract, bound to a specific deployed contract.
func NewLuminametacontractTransactor(address common.Address, transactor bind.ContractTransactor) (*LuminametacontractTransactor, error) {
	contract, err := bindLuminametacontract(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &LuminametacontractTransactor{contract: contract}, nil
}

// NewLuminametacontractFilterer creates a new log filterer instance of Luminametacontract, bound to a specific deployed contract.
func NewLuminametacontractFilterer(address common.Address, filterer bind.ContractFilterer) (*LuminametacontractFilterer, error) {
	contract, err := bindLuminametacontract(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &LuminametacontractFilterer{contract: contract}, nil
}

// bindLuminametacontract binds a generic wrapper to an already deployed contract.
func bindLuminametacontract(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := LuminametacontractMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Luminametacontract *LuminametacontractRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Luminametacontract.Contract.LuminametacontractCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Luminametacontract *LuminametacontractRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Luminametacontract.Contract.LuminametacontractTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Luminametacontract *LuminametacontractRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Luminametacontract.Contract.LuminametacontractTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Luminametacontract *LuminametacontractCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Luminametacontract.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Luminametacontract *LuminametacontractTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Luminametacontract.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Luminametacontract *LuminametacontractTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Luminametacontract.Contract.contract.Transact(opts, method, params...)
}

// GetValue is a free data retrieval call binding the contract method 0x960384a0.
//
// Solidity: function getValue(string key) view returns(uint128, uint128)
func (_Luminametacontract *LuminametacontractCaller) GetValue(opts *bind.CallOpts, key string) (*big.Int, *big.Int, error) {
	var out []interface{}
	err := _Luminametacontract.contract.Call(opts, &out, "getValue", key)

	if err != nil {
		return *new(*big.Int), *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	out1 := *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)

	return out0, out1, err

}

// GetValue is a free data retrieval call binding the contract method 0x960384a0.
//
// Solidity: function getValue(string key) view returns(uint128, uint128)
func (_Luminametacontract *LuminametacontractSession) GetValue(key string) (*big.Int, *big.Int, error) {
	return _Luminametacontract.Contract.GetValue(&_Luminametacontract.CallOpts, key)
}

// GetValue is a free data retrieval call binding the contract method 0x960384a0.
//
// Solidity: function getValue(string key) view returns(uint128, uint128)
func (_Luminametacontract *LuminametacontractCallerSession) GetValue(key string) (*big.Int, *big.Int, error) {
	return _Luminametacontract.Contract.GetValue(&_Luminametacontract.CallOpts, key)
}

// AddOracle is a paid mutator transaction binding the contract method 0xdf5dd1a5.
//
// Solidity: function addOracle(address newOracleAddress) returns()
func (_Luminametacontract *LuminametacontractTransactor) AddOracle(opts *bind.TransactOpts, newOracleAddress common.Address) (*types.Transaction, error) {
	return _Luminametacontract.contract.Transact(opts, "addOracle", newOracleAddress)
}

// AddOracle is a paid mutator transaction binding the contract method 0xdf5dd1a5.
//
// Solidity: function addOracle(address newOracleAddress) returns()
func (_Luminametacontract *LuminametacontractSession) AddOracle(newOracleAddress common.Address) (*types.Transaction, error) {
	return _Luminametacontract.Contract.AddOracle(&_Luminametacontract.TransactOpts, newOracleAddress)
}

// AddOracle is a paid mutator transaction binding the contract method 0xdf5dd1a5.
//
// Solidity: function addOracle(address newOracleAddress) returns()
func (_Luminametacontract *LuminametacontractTransactorSession) AddOracle(newOracleAddress common.Address) (*types.Transaction, error) {
	return _Luminametacontract.Contract.AddOracle(&_Luminametacontract.TransactOpts, newOracleAddress)
}

// RemoveOracle is a paid mutator transaction binding the contract method 0xfdc85fc4.
//
// Solidity: function removeOracle(address oracleToRemove) returns()
func (_Luminametacontract *LuminametacontractTransactor) RemoveOracle(opts *bind.TransactOpts, oracleToRemove common.Address) (*types.Transaction, error) {
	return _Luminametacontract.contract.Transact(opts, "removeOracle", oracleToRemove)
}

// RemoveOracle is a paid mutator transaction binding the contract method 0xfdc85fc4.
//
// Solidity: function removeOracle(address oracleToRemove) returns()
func (_Luminametacontract *LuminametacontractSession) RemoveOracle(oracleToRemove common.Address) (*types.Transaction, error) {
	return _Luminametacontract.Contract.RemoveOracle(&_Luminametacontract.TransactOpts, oracleToRemove)
}

// RemoveOracle is a paid mutator transaction binding the contract method 0xfdc85fc4.
//
// Solidity: function removeOracle(address oracleToRemove) returns()
func (_Luminametacontract *LuminametacontractTransactorSession) RemoveOracle(oracleToRemove common.Address) (*types.Transaction, error) {
	return _Luminametacontract.Contract.RemoveOracle(&_Luminametacontract.TransactOpts, oracleToRemove)
}

// SetThreshold is a paid mutator transaction binding the contract method 0x960bfe04.
//
// Solidity: function setThreshold(uint256 newThreshold) returns()
func (_Luminametacontract *LuminametacontractTransactor) SetThreshold(opts *bind.TransactOpts, newThreshold *big.Int) (*types.Transaction, error) {
	return _Luminametacontract.contract.Transact(opts, "setThreshold", newThreshold)
}

// SetThreshold is a paid mutator transaction binding the contract method 0x960bfe04.
//
// Solidity: function setThreshold(uint256 newThreshold) returns()
func (_Luminametacontract *LuminametacontractSession) SetThreshold(newThreshold *big.Int) (*types.Transaction, error) {
	return _Luminametacontract.Contract.SetThreshold(&_Luminametacontract.TransactOpts, newThreshold)
}

// SetThreshold is a paid mutator transaction binding the contract method 0x960bfe04.
//
// Solidity: function setThreshold(uint256 newThreshold) returns()
func (_Luminametacontract *LuminametacontractTransactorSession) SetThreshold(newThreshold *big.Int) (*types.Transaction, error) {
	return _Luminametacontract.Contract.SetThreshold(&_Luminametacontract.TransactOpts, newThreshold)
}

// SetTimeoutSeconds is a paid mutator transaction binding the contract method 0x36f945cd.
//
// Solidity: function setTimeoutSeconds(uint256 newTimeoutSeconds) returns()
func (_Luminametacontract *LuminametacontractTransactor) SetTimeoutSeconds(opts *bind.TransactOpts, newTimeoutSeconds *big.Int) (*types.Transaction, error) {
	return _Luminametacontract.contract.Transact(opts, "setTimeoutSeconds", newTimeoutSeconds)
}

// SetTimeoutSeconds is a paid mutator transaction binding the contract method 0x36f945cd.
//
// Solidity: function setTimeoutSeconds(uint256 newTimeoutSeconds) returns()
func (_Luminametacontract *LuminametacontractSession) SetTimeoutSeconds(newTimeoutSeconds *big.Int) (*types.Transaction, error) {
	return _Luminametacontract.Contract.SetTimeoutSeconds(&_Luminametacontract.TransactOpts, newTimeoutSeconds)
}

// SetTimeoutSeconds is a paid mutator transaction binding the contract method 0x36f945cd.
//
// Solidity: function setTimeoutSeconds(uint256 newTimeoutSeconds) returns()
func (_Luminametacontract *LuminametacontractTransactorSession) SetTimeoutSeconds(newTimeoutSeconds *big.Int) (*types.Transaction, error) {
	return _Luminametacontract.Contract.SetTimeoutSeconds(&_Luminametacontract.TransactOpts, newTimeoutSeconds)
}
