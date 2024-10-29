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


# Node Deployment via Docker Compose

This guide provides instructions for deploying the node using Docker Compose. This setup allows you to run the node on your local machine or any infrastructure that supports Docker Compose.

## Requirements

- Ensure **Docker** and **Docker Compose** are installed on your machine.

## Setup

1. **Navigate to the Docker Compose Folder**
   - In this repository, locate the `docker-compose` folder, where you will find a file named `docker-compose.yaml`.

2. **Configure Environment Variables**
   - Create a `.env` file in the same directory as `docker-compose.yaml`. This file should contain the necessary variables, including:
     - `PRIVATE_KEY`
     - `PRIVATE_KEY_PASSWORD`
     - `DEPLOYED_CONTRACT`
     - *(Add any additional required variables here)*

## Deployment

3. **Run Docker Compose**
   - Open a terminal in the `docker-compose` folder and start the deployment by running:

     ```bash
     docker-compose up -d
     ```

## Verification

4. **Verify Logs**
   - Check if the container is running correctly by viewing the logs. Run the following command:

     ```bash
     docker-compose logs -f
     ```

   - **Expected Logs**: Look for logs similar to the example below, which indicate a successful startup:

    ```
        │ time="2024-10-29T13:39:35Z" level=info msg="Processor - Atomic filter value for market Binance:SUSHI-USDT with 20 trades: 0.7095307176575745."                                                                  │
        │ time="2024-10-29T13:39:35Z" level=info msg="Processor - Atomic filter value for market Simulation:UNI-USDC with 1 trades: 8.008539500390082."                                                                   │
        │ time="2024-10-29T13:39:35Z" level=info msg="Processor - Atomic filter value for market Crypto.com:USDT-USD with 5 trades: 0.99948."                                                                             │
        │ time="2024-10-29T13:39:35Z" level=info msg="Processor - filter median for MOVR: 9.864475653518195."                                                                                                             │
        │ time="2024-10-29T13:39:35Z" level=info msg="Processor - filter median for STORJ: 0.4672954012114179."                                                                                                           │
        │ time="2024-10-29T13:39:35Z" level=info msg="Processor - filter median for DIA: 0.9839597410694259."                                                                                                             │
        │ time="2024-10-29T13:39:35Z" level=info msg="Processor - filter median for WETH: 2626.9564003841315."   
    ```
## Error handling
If any issues arise, consult the log output for error messages and ensure all environment variables are correctly set in the `.env` file.
