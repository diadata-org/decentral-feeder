# Overview

This repository consists of a self-contained data collection, processing and publishing pipeline. More precisely, scrapers are collecting trades data from various centralized and decentralized exchanges.
Thus obtained trades are then processed in a 2-step aggregation procedure in order to come up with a scalar value related to an asset that is subsequently published on-chain. In most cases, this value will be an asset's USD price.

![embed]https://github.com/diadata-org/decentral-feeder/assets/Feeder_Architecture_Small.pdf[/embed]

# Detailed Description of the Building Blocks
In the following, we describe function and usage of the constituting building blocks (see figure). We proceed from bottom to top.

## Scrapers
Each scraper is implemented in a dedicated file in the folder /pkg/scraper with the main function signature `func NewExchangeScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, wg *sync.WaitGroup)`, 
resp. `pools` instead of `pairs` for decentralized exchanges.\
Its function is to continuously fetch trades data from a given exchange and send them to the channel `tradesChannel`.\
The expected input for a scraper is a set of pair tickers such as `BTC-USDT`. Tickers are always capitalized and symbols separated by a hyphen. It's the role of the scraper to format the pair ticker such that it can subscribe to
 the corresponding (websocket) stream. \
For centralized exchanges, a json file in /config/symbolIdentification is needed that assigns blockchain and address to each ticker symbol the scraper is handling.

## Collector
The collector gathers trades from all running scrapers. As soon as it receives a signal through a trigger channel it bundles trades in *atomic tradesblocks*. An atomic tradesblock is a set of trades restricted to one market on one exchange, for instance `BTC-USDT` trades on Binance exchange. These tradesblocks are sent to the `Processor`.

## Processor
The processor is a 2-step aggregation procedure similar to mapReduce.\
1. Step: Aggregate trades from an atomic tradesblock. The type of aggregation can be selected through an environment variable (see Feeder/main). The only assumption on the aggregation implementation is that it returns a `float64`.
2. Step: Aggregate filter values obtained in step 1. More precisely, all filter values obtained from atomic blocks with the same quote token are aggregated. 
