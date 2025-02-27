## Table of Contents
 - [Resources](#resources)
 - [Overview](#overview)
 - [Detailed Description of the Building Blocks](#detailed-description-of-the-building-blocks)
   - [Scrapers](#scrapers)
   - [Collector](#collector)
   - [Processor](#processor)
   - [Feeder](#feeder)
   - [Monitoring](#monitoring)
 - [Smart Contract Documentation](contracts/README.md)
 - [Node Deployment Guide](#node-deployment-guide)
   - [Requirements](#requirements)
   - [Docker Compose Deployment](#docker-compose-deployment)
     - [Navigate to the Docker Compose Folder](#navigate-to-the-docker-compose-folder)
     - [Configure Environment Variables](#configure-environment-variables)
     - [Retrieve Deployed Contract](#retrieve-deployed-contract)
     - [Run Docker Compose](#run-docker-compose)
   - [Alternative Deployment Methods](#alternative-deployment-methods)
     - [Docker Run Deployment](#docker-run-deployment)
     - [Kubernetes Deployment](#kubernetes-deployment)
   - [Adding Exchange Pairs](#adding-exchange-pairs)
   - [Watchdog environment variables](#watchdog-environment-variables)
   - [Error Handling](#error-handling)
- [Migration guide to the new DIA testnet](#migration-guide-to-the-new-dia-testnet)
- [Conclusion](#conclusion)



## Resources

## Resources


| **Field**         | **Value**                                                                                      |
|--------------------|-----------------------------------------------------------------------------------------------|
| **Chain name**     | DIA Lasernet Testnet                                                                          |
| **Chain ID**       | 100640                                                                                        |
| **Block explorer** | [https://testnet-explorer.diadata.org](https://testnet-explorer.diadata.org)                  |
| **RPC URL**        | [https://testnet-rpc.diadata.org](https://testnet-rpc.diadata.org)                            |
| **Websocket**      | [wss://testnet-rpc.diadata.org](wss://testnet-rpc.diadata.org)                                |
| **Gas token**      | DIA on ETH Sepolia `0xa35a89390FcA5dB148859114DADe875280250Bd1`                               |
| **Faucet**         | [https://faucet.diadata.org](https://faucet.diadata.org)                                      |
| **Documentation**  | [https://docs.diadata.org](https://docs.diadata.org)                                          |

# Overview

This repository hosts a self-contained containerized application comprising three main components: scraper, collector, and processor. The scraper collects trade data from various centralized and decentralized exchanges. The collector and processor aggregate the data through a two-step process to produce a scalar value associated with an asset, which is subsequently published on-chain. In most cases, this value represents the asset's price in USD.


# Detailed Description of the Building Blocks
![Feeder Architecture v2](assets/Feeder_Architecture_v2.png)

In the following sections, we describe the function and usage of the building blocks (the three components mentioned earlier) that make up the system (see figure). The explanation proceeds from left to right


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
The processor is a 2-step aggregation procedure similar to mapReduce:
* Step 1: Aggregate trades from an atomic tradesblock. The type of aggregation can be selected through an environment variable (see Feeder/main). The only assumption on the aggregation implementation is that it returns a `float64`.
* Step 2: Aggregate filter values obtained in step 1. The selection of aggregation method and assumptions are identical to Step 1.
The obtained scalar value is sent to the Oracle feeder.

## Feeder
The feeder is feeding a simple key value oracle. It publishes the value obtained from the Processor. It is worth mentioning that the feeder can contain the trigger mechanism that initiates an iteration of the data flow diagram.

## Monitoring
For monitoring, we use Prometheus client libraries for Go to create, manage, and expose metrics. A metrics struct is defined to track uptime (using a Prometheus gauge) and store Pushgateway details like URL, job name, and authentication credentials. The uptime metric is initialized, registered, and updated periodically based on the application's runtime. Metrics are pushed to the Pushgateway every 30 seconds, with authentication and error handling in place.

## Smart Contract Documentation
For more details about the contracts, refer to the following documentation:

[Smart Contract Documentation](contracts/README.md)


# Node Deployment Guide

This document outlines the procedures for deploying the `diadata/decentralized-feeder:<VERSION>` containerized application. Replace `<VERSION>` with the desired version (e.g.`v0.0.5`) when deploying.

For the most recent Docker image tags, please refer to public docker hub:
[https://hub.docker.com/r/diadata/decentralized-feeder/tags](https://hub.docker.com/r/diadata/decentralized-feeder/tags)
## Requirements

- Ensure that Docker or Docker Compose is installed on your machine.

- Clone this repository to your local machine.

- The container has minimal resource requirements, making it suitable for most machines, including Windows, macOS, Linux, and Raspberry Pi, as long as Docker is installed.

- An ETH private key from MetaMask or any other Eth wallet. Alternatively to generate private key effortlesly eth-keys tool can be used for this [ethereum/eth-keys](https://github.com/ethereum/eth-keys)

- DIA tokens in your wallet (you can use faucet for this [https://faucet.diadata.org](https://faucet.diadata.org))   

## Docker Compose Deployment

###  Navigate to the Docker Compose Folder
   - Locate the `docker-compose` folder in this repository.
   - Inside, you will find a file named `docker-compose.yaml`.

###  Configure Environment Variables
   - Create a `.env` file in the same directory as `docker-compose.yaml`. This file should contain the following variables:
     - `NODE_OPERATOR_NAME`: A unique and descriptive name identifying the organization or entity running the node. This name is used for monitoring and should be chosen carefully to ensure it is both meaningful and recognizable (e.g., include your organization name or geographical region). Providing a clear name helps distinguish your node in dashboards and logs.
     - `CHAIN_ID`: set the chain ID value
     - `PRIVATE_KEY`: Your private key for the deployment.
     - `DEPLOYED_CONTRACT`: The contract address. Initially, leave this empty during the first deployment to retrieve the deployed contract.
     - `PUSHGATEWAY_USER`:  to allow decentralized-feeder authenticate towards the monitoring server. Reach out to the team to get hold of these credentials, info [at] diadata.org
     - `PUSHGATEWAY_PASSWORD`: to allow decentralized-feeder authenticate towards the monitoring server. Reach out to the team to get hold of these credentials,  info [at] diadata.org
     
     For additional environment variable configurations, refer to [Adding Exchange Pairs](#adding-exchange-pairs) and [Watchdog environment variables](#watchdog-environment-variables)


   - Example `.env` file:
     ```plaintext
     NODE_OPERATOR_NAME=
     CHAIN_ID=
     PRIVATE_KEY=
     DEPLOYED_CONTRACT=
     PUSHGATEWAY_USER=
     PUSHGATEWAY_PASSWORD=
     ```

   - Open a terminal in the `docker-compose` folder and start the deployment by running:
      ```bash
      docker-compose up
      ```

###  Retrieve Deployed Contract
   - Once the container is deployed with `DEPLOYED_CONTRACT` env variable empty the logs will display the deployed contract address in the following format:
     ```plaintext
     │ time="2024-11-25T11:30:08Z" level=info msg="Contract pending deploy: 0xxxxxxxxxxxxxxxxxxxxxxxxxx."
     ```
   - Copy the displayed contract address (e.g., `0xxxxxxxxxxxxxxxxxxxxxxxxxx`) and stop the container with `docker rm -f <container_name>`.

   - Update your `.env` file with `DEPLOYED_CONTRACT` variable mentioned above. Redeployed the container with  `docker-compose up -d`
     ```plaintext
     DEPLOYED_CONTRACT=0xxxxxxxxxxxxxxxxxxxxxxxxxx
     ```

   - Check if the container is running correctly by viewing the logs. Run the following command:
     ```bash
     docker-compose logs -f
     ```

   - Expected Logs: Look for logs similar to the example below, which indicate a successful startup:
     ```
     │ time="2024-10-29T13:39:35Z" level=info msg="Processor - Atomic filter value for market Binance:SUSHI-USDT with 20 trades: 0.7095307176575745."                                                                  │
     │ time="2024-10-29T13:39:35Z" level=info msg="Processor - Atomic filter value for market Simulation:UNI-USDC with 1 trades: 8.008539500390082."                                                                   │
     │ time="2024-10-29T13:39:35Z" level=info msg="Processor - Atomic filter value for market Crypto.com:USDT-USD with 5 trades: 0.99948."                                                                             │
     │ time="2024-10-29T13:39:35Z" level=info msg="Processor - filter median for MOVR: 9.864475653518195."                                                                                                             │
     │ time="2024-10-29T13:39:35Z" level=info msg="Processor - filter median for STORJ: 0.4672954012114179."                                                                                                           │
     │ time="2024-10-29T13:39:35Z" level=info msg="Processor - filter median for DIA: 0.9839597410694259."                                                                                                             │
     │ time="2024-10-29T13:39:35Z" level=info msg="Processor - filter median for WETH: 2626.9564003841315."   
     ```
    
   - You can optionally cleanup the deployment once you're done by running:

      ```
      docker rm -f <container_name>
      ```
   - Verify the container has been removed:
      ```
      docker ps -a
      ```




## Alternative Deployment Methods

###  Docker Run Deployment

This method is suitable for simple setups without orchestration.

#### Command
   - Deploy the feeder with `DEPLOYED_CONTRACT` initially empty:
     ```bash
     docker run -d \
       -e NODE_OPERATOR_NAME= \
       -e PRIVATE_KEY= \
       -e CHAIN_ID= \
       -e DEPLOYED_CONTRACT= \
       -e PUSHGATEWAY_USER= \
       -e PUSHGATEWAY_PASSWORD= \
       --name decentralized-feeder \
       diadata/decentralized-feeder:<VERSION>
     ```
   - Retrieve the logs to get the deployed contract address:
     ```bash
     docker logs <container_name>
     ```
   - Stop the container, update the `DEPLOYED_CONTRACT` value, and restart:
     ```bash
     docker stop <container_name>
     docker run -d \
       -e NODE_OPERATOR_NAME= \
       -e PRIVATE_KEY= \
       -e CHAIN_ID= \
       -e DEPLOYED_CONTRACT= \
       -e PUSHGATEWAY_USER= \
       -e PUSHGATEWAY_PASSWORD= \
       -e EXCHANGEPAIRS= \ 
       --name decentralized-feeder \
       diadata/decentralized-feeder:<VERSION>
     ```
   - Retrieve the logs to verify the container is running as expected
      ```bash
      docker logs <container_name>
      ```
   -  For additional environment variable configurations, refer to [Adding Exchange Pairs](#adding-exchange-pairs) and [Watchdog environment variables](#watchdog-environment-variables)


###  Kubernetes Deployment

Kubernetes is ideal for production environments requiring scalability and high availability.

#### Deployment YAML
   - Create a Kubernetes `Deployment` manifest. Replace `<VERSION>` with the desired version:
     ```yaml
     apiVersion: apps/v1
     kind: Deployment
     metadata:
       name: decentralized-feeder
       namespace: default
     spec:
       replicas: 1
       selector:
         matchLabels:
           app: decentralized-feeder
       template:
         metadata:
           labels:
             app: decentralized-feeder
         spec:
           containers:
           - name: feeder-container
             image: diadata/decentralized-feeder:<VERSION>
             env:
             - name: PRIVATE_KEY
               valueFrom:
                 secretKeyRef: {key: private_key_secret, name: private_key_secret}
             - name: NODE_OPERATOR_NAME
               value: ""
             - name: DEPLOYED_CONTRACT
               value: ""
             - name: CHAIN_ID
               value: ""
             - name: EXCHANGEPAIRS
               value: ""
             - name: PUSHGATEWAY_USER= 
               value: ""
             - name: PUSHGATEWAY_PASSWORD= 
               value: ""
             - containerPort: 8080

For additional environment variable configurations, refer to [Adding Exchange Pairs](#adding-exchange-pairs) and [Watchdog environment variables](#watchdog-environment-variables)

#### Steps to Deploy
   1. Deploy the feeder with `DEPLOYED_CONTRACT` set to an empty string (`""`) in the Kubernetes manifest.
      ```bash
      kubectl apply -f deployment.yaml
      ```
   2. Monitor the logs for the deployed contract address:
      ```bash
      kubectl logs <pod-name>
      ```
   3. Update the `DEPLOYED_CONTRACT` value in the manifest with the retrieved contract address.
   4. Apply the updated manifest:
      ```bash
      kubectl apply -f deployment.yaml
      ```



## Adding Exchange Pairs

To configure exchange pairs for the decentralized feeder, use the `EXCHANGEPAIRS` environment variable. This can be done regardless of the deployment method. The variable specifies pairs to scrape from various exchanges, formatted as a comma-separated list of `<Exchange>:<Asset-Pair>` (e.g., `Binance:BTC-USDT`).


#### Steps to Add Exchange Pairs

Locate the environment configuration file or section for your deployment method:
   - For Docker Compose: Use the `.env` file or add directly to the `docker-compose.yaml` file.
   - For Kubernetes: Update the kubernetes manifest file `manifest.yaml`
   - For Docker Run: Pass the variable directly using the `-e` flag.

### Define the `EXCHANGEPAIRS` variable with your desired pairs as a comma-separated list.

   - Example in docker-compose:
     ```plaintext
     EXCHANGEPAIRS=" 
     Binance:TON-USDT, Binance:TRX-USDT, Binance:UNI-USDT, Binance:USDC-USDT, Binance:WIF-USDT,
     CoinBase:AAVE-USD, CoinBase:ADA-USD, CoinBase:AERO-USD, CoinBase:APT-USD, CoinBase:ARB-USD,
     GateIO:ARB-USDT, GateIO:ATOM-USDT, GateIO:AVAX-USDT, GateIO:BNB-USDT, GateIO:BONK-USDT,
     Kraken:AAVE-USD, Kraken:ADA-USD, Kraken:ADA-USDT, Kraken:APT-USD, Kraken:ARB-USD,
     KuCoin:AAVE-USDT, KuCoin:ADA-USDT, KuCoin:AERO-USDT, KuCoin:APT-USDT, KuCoin:AR-USDT,
     Crypto.com:BONK-USD, Crypto.com:BTC-USDT, Crypto.com:BTC-USD, Crypto.com:CRV-USD
     "

   - Example in Kubernetes manifest:
       ```yaml
      spec:
        containers:
        - name: feeder-container
          image: diadata/decentralized-feeder:<VERSION>
          env:
          - name: PRIVATE_KEY
            value: "myprivatekey"
          - name: DEPLOYED_CONTRACT
            value: ""
          - name: EXCHANGEPAIRS
            value: "
            Binance:TON-USDT, Binance:TRX-USDT, Binance:UNI-USDT, Binance:USDC-USDT, Binance:WIF-USDT,
            CoinBase:AAVE-USD, CoinBase:ADA-USD, CoinBase:AERO-USD, CoinBase:APT-USD, CoinBase:ARB-USD,
            GateIO:ARB-USDT, GateIO:ATOM-USDT, GateIO:AVAX-USDT, GateIO:BNB-USDT, GateIO:BONK-USDT,
            Kraken:AAVE-USD, Kraken:ADA-USD, Kraken:ADA-USDT, Kraken:APT-USD, Kraken:ARB-USD,
            KuCoin:AAVE-USDT, KuCoin:ADA-USDT, KuCoin:AERO-USDT, KuCoin:APT-USDT, KuCoin:AR-USDT,
            Crypto.com:BONK-USD, Crypto.com:BTC-USDT, Crypto.com:BTC-USD, Crypto.com:CRV-USD
            "
          ports:
          - containerPort: 8080


   - Example in Docker Run:
     ```bash
     docker run -d \
      -e NODE_OPERATOR_NAME= \
      -e PRIVATE_KEY=your-private-key \
      -e DEPLOYED_CONTRACT=your-contrract \
      -e EXCHANGEPAIRS="Binance:TON-USDT, Binance:TRX-USDT, ....." \
      --name decentralized-feeder \
      diadata/decentralized-feeder:<VERSION>
     ```

 ### Verify the configuration:
   - Docker Compose: Check logs with:
     ```bash
     docker-compose logs -f
     ```
   - Kubernetes: Check pod logs:
     ```bash
     kubectl logs <pod-name>
     ```
   - Docker Run: View logs with:
     ```bash
     docker logs <container-name>
     ```
   - The output should look like:
     ```
      lasernet-feeder-1  | time="2024-11-26T12:22:47Z" level=info msg="Processor - Start......"
      lasernet-feeder-1  | time="2024-11-26T12:22:47Z" level=info msg="CoinBase - Started scraper."
      lasernet-feeder-1  | time="2024-11-26T12:22:47Z" level=info msg="Kraken - Started scraper."
      lasernet-feeder-1  | time="2024-11-26T12:22:47Z" level=info msg="GateIO - Started scraper."
      lasernet-feeder-1  | time="2024-11-26T12:22:47Z" level=info msg="KuCoin - Started scraper."
      lasernet-feeder-1  | time="2024-11-26T12:22:47Z" level=info msg="Crypto.com - Started scraper."
      lasernet-feeder-1  | time="2024-11-26T12:22:47Z" level=info msg="Binance - Started scraper at 2024-11-26 12:22:47.97428349 +0000 UTC m=+0.037715635."lasernet-feeder-1  | time="2024-11-26T12:23:08Z" level=info msg="Processor - Atomic filter value for market Binance:USDC-USDT with 89 trades: 0.9998099980000003."
      lasernet-feeder-1  | time="2024-11-26T12:23:08Z" level=info msg="Processor - Atomic filter value for market Binance:UNI-USDT with 47 trades: 10.817108170000003."
      lasernet-feeder-1  | time="2024-11-26T12:23:09Z" level=info msg="Processor - Atomic filter value for market Binance:TRX-USDT with 297 trades: 0.18920189200000007."
      lasernet-feeder-1  | time="2024-11-26T12:23:09Z" level=info msg="Processor - Atomic filter value for market CoinBase:APT-USD with 18 trades: 11.38."
      lasernet-feeder-1  | time="2024-11-26T12:23:09Z" level=info msg="Processor - Atomic filter value for market GateIO:BNB-USDT with 3 trades: 620.9062090000001."
      lasernet-feeder-1  | time="2024-11-26T12:23:09Z" level=info msg="Processor - Atomic filter value for market CoinBase:AERO-USD with 5 trades: 1.27824."
      ....
      ```

## Watchdog environment variables
The decentralized feeders contain two different types of watchdog variables that monitor the liveliness of WebSocket connections used for subscribing to trades in exchange pairs.
1. Exchange-wide watchdogs, such as `BINANCE_WATCHDOG` If no trades are recorded by a scraper for any pair on the given exchange within `BINANCE_WATCHDOG` seconds, the scraper is restarted and will resubscribe to all pairs specified in the feeder's configuration.
2. Pairwise watchdogs such as `BINANCE_WATCHDOG_BTC_USDT`:  If no trades are recorded by a scraper for a specific pair within `EXCHANGE_WATCHDOG_ASSET1_ASSET2` seconds, the scraper will unsubscribe and subsequently resubscribe to the corresponding pair. All other subscriptions of this scraper will remain untouched.
The first type of watchdog applies to cases where the scraper fails, for instance due to server-side issues, and require a restart.
The second type of watchdog applies to dropping websocket subscriptions. These ocurr in websocket connections and are often "silent", i.e. there is no error message that allows for a proper handling. 
An example of how watchdog variable could look like in the context of kubernetes manifest.
```
  - name: COINBASE_WATCHDOG
    value: "240"
  - name: CRYPTODOTOCOM_WATCHDOG
    value: "240"
  - name: GATEIO_WATCHDOG
    value: "240"
  - name: BINANCE_WATCHDOG_BTC_USDTs
    value: "300"
  - name: CRYPTODOTCOM_WATCHDOG_BTC_USDT
    value: "300"
  - name: KUCOIN_WATCHDOG_BTC_USDC
    value: "300"
```

## Error Handling
If any issues arise during deployment, follow these steps based on your deployment method:

 #### Check Logs:
   - Docker Compose: `docker-compose logs -f`
   - Docker Run: `docker logs <container_name>`
   - Kubernetes: `kubectl logs <pod-name>`

 #### Verify Environment Variables:
   - Ensure all required variables (`PRIVATE_KEY`, `DEPLOYED_CONTRACT`) are correctly set:
     - Docker Compose: Check `.env` file.
     - Docker Run: Verify `-e` flags.
     - Kubernetes: Check the Deployment manifest or ConfigMap.

 #### Restart Deployment:
   - Docker Compose: 
     ```bash
     docker-compose down && docker-compose up -d
     ```
   - Docker Run: 
     ```bash
     docker stop <container_name> && docker rm <container_name> && docker run -d ...
     ```
   - Kubernetes:
     ```bash
     kubectl delete pod <pod-name>
     ```

 #### Check Configuration:
   - Ensure the correct image version is used and manifests/files are properly configured.

 #### Update or Rebuild:
   - Ensure you're using the correct image version:
     ```bash
     docker pull diadata/decentralized-feeder:<VERSION>
     ```
   - Apply fixes and redeploy.


## Migration guide to the new DIA testnet

We've migrated DIA lasernet from Optimism to Arbitrum. This guide provides step-by-step instructions for data feeders to transition their DIA Lasernet node to the new DIA testnet.

### Prerequisites
- **Latest DIA Docker Image**: Use the most recent image version. Check the latest tags [here](https://hub.docker.com/r/diadata/decentralized-feeder/tags).
- **DIA Tokens**: Verify that you have DIA tokens in your wallet on the new testnet. You can obtain tokens via the [DIA Faucet](https://faucet.diadata.org) or by contacting the team.

### Steps
1. **Set the DEPLOYED_CONTRACT to an Empty String**:  
   In your deployment configuration (or `.env` file), update the variable as follows:  
   `DEPLOYED_CONTRACT=""`

2. **Set the CHAIN_ID to 100640 (new testnet id)**:  
   `CHAIN_ID="100640"`

3. **Deploy the Container**:
  When deployed with an empty `DEPLOYED_CONTRACT`, the logs will display a message like: 
  ``` 
  time="2024-11-25T11:30:08Z" level=info msg="Contract pending deploy: 0xxxxxxxxxxxxxxxxxxxxxxxxxx."
  ```

4. **Stop the Running Container**:
    Stop the container using your preferred method (e.g., docker rm -f <container_name>).

5. **Update Your Configuration File**:
    Open your .env file and update the DEPLOYED_CONTRACT variable with the copied address:
    `DEPLOYED_CONTRACT=0xxxxxxxxxxxxxxxxxxxxxxxxxx`

6. **Redeploy the Container**:
  Bring up the container again with the updated configuration (e.g., using docker-compose up -d).

7. **Verify the Deployment**:
Check the container logs to ensure everything is running correctly:
`docker-compose logs -f`

## Conclusion

The `diadata/decentralized-feeder:<VERSION>` image can be deployed using various methods to accommodate different use cases. For production environments, Kubernetes or Helm is recommended for scalability and flexibility. For simpler setups or local testing, Docker Compose or Docker Run is sufficient.

If you encounter any issues or need further assistance, feel free to reach out to the team @ info [at] diadata.org