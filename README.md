# Overview

This repository consists of a self-contained data collection, processing and publishing pipeline. More precisely, scrapers are collecting trades data from various centralized and decentralized exchanges.
Thus obtained trades are then processed in a 2-step aggregation procedure in order to come up with a scalar value related to an asset that is subsequently published on-chain. In most cases, this value will be an asset's USD price.

![alt text](https://github.com/diadata-org/decentral-feeder/blob/master/assets/Feeder_Architecture_Small.jpg?raw=true)

# Detailed Description of the Building Blocks
In the following, we describe function and usage of the constituting building blocks (see figure). We proceed from bottom to top.

## Scrapers
Each scraper is implemented in a dedicated file in the folder /pkg/scrapers with the main function signature `func NewExchangeScraper(pairs []models.ExchangePair, tradesChannel chan models.Trade, wg *sync.WaitGroup)`, 
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
2. Step: Aggregate filter values obtained in step 1. The selection of aggregation method and assumptions are identical to Step 1.
The obtained scalar value is sent to the Oracle feeder.

## Feeder
The feeder is feeding a simple key value oracle. It publishes the value obtained from the Processor. It is worth mentioning that the feeder can contain the trigger mechanism that initiates an iteration of the data flow diagram.


## Deployment Methods
1. Deploying via Docker Compose

You can deploy the node using Docker Compose, allowing you to run it on your machine or any infrastructure that supports Docker Compose.
Steps:
* In this repository, locate the docker-compose folder.
* Inside, you will find a file named docker-compose.yaml.

* Update Placeholder Values:
     * Before deploying, you need to update the following placeholder values in the docker-compose.yaml file:
        * PRIVATE_KEY
        * PRIVATE_KEY_PASSWORD
        * DEPLOYED_CONTRACT
        * UniswapV2_URI_REST=
        * UniswapV2_URI_WS=
        * HTTPS_PROXY (only needed if deployed to infra hosted in states)
    * Also, if you want to deploy a specific tag, update this version under 
        *image: us.icr.io/dia-registry/oracles/diadecentraloracleservice:v1.0.XXX

* After updating the placeholder values, you can deploy the node by running the following command in the terminal:
`docker-compose up -d`

2. Deploying via Helm on Kubernetes

You can also deploy the containers to a Kubernetes cluster using Helm manifest files.
Steps:
* Navigate to the Helm Charts Directory:
    * The Helm manifest files are located under the /helmcharts/oracles/conduit-test directory.

Deploy Using Helm:
* Use Helm to deploy the containers to your Kubernetes cluster. Ensure you have Helm installed and configured to interact with your Kubernetes cluster.
* You can deploy the Helm chart by running the following command:
    `helm upgrade -n dia-oracles-prod --set repository.tag="v1.0.XX" diaoracleservice-conduit-XX .`
