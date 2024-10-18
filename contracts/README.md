# DIA Lumina oracle metacontract

## Overview
The DIA Lumina oracle consists of a two major components:

1. A collection of key/value smart contract that collect price data from each feeder.
2. The DIA metacontract that collates these prices from the key/value contracts and provides an automatically refreshed reading of the latest market price.

An exemplary Lumina data flow can be seen here.
Oracle feeders scrape trades from exchanges and submit these as last prices into their respective oracle contracts.
From there, a threshold of 5 in 1 hour is required for the meta contract to assume consensus on the median value of this set of trades.
![plot of the Lumina system](abstract_flow.png)

## Usage
The metacontract exposed a simple `Read` function called `getValue()`.
It has one parameter `pairKey`, which consists of the symbol of the queried asset and `/USD` to denominate its price in US Dollars.
For instance, the latest Bitcoin price is queried by calling `getValue("BTC/USD")`.

The function then returns two values:
1. The value of the asset in US Dollars, with 8 decimals.
2. The timestamp of the block with the latest calculation result.

The oracle has several safeguards against becoming stale or having not enough feeders providing updates.

## How it works

### Methodologies
The current version of the metacontract supports the median methodology.
Later iterations of the metacontract will allow selecting different methodologies for each price evaluation.

### Roles
Each metacontract has an `admin` address that can setup the metacontract for its specific usage.
The admin is able to add and remove feeders, set a threshold of required feeders, and set a timeout duration after which a feeder update is considered stale.

### Registering Feeders
The metacontract needs to learn about available feeder key/value storage smart contracts.
The admin can add as many feeders as they wish to the price evaluation using the `addOracle()` method.
The required parameter of that method is the address (on DIA chain) of the key/value smart contract belonging to that feeder.
After an oracle is added, its latest prices are immediately part of the price evaluation.

### Removing feeders
If needed, oracle feeders can also be removed by the admin.
The required parameter of that method is the address (on DIA chain) of the key/value smart contract belonging to that feeder.
Removal is immediate, so any call made to `getValue()` from the removal block onwards will not include data from the removed feeder any more.

### Acceptance Threshold
To make price discovery more robust, a threshold needs to be set by the admin.
This threshold corresponds to the number of feeders that need to have submitted a price to reach a valid consensus.

