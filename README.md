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


# Node Deployment Guide

This document outlines the procedures for deploying the `diadata/decentralized-feeder:<VERSION>` containerized application. Replace `<VERSION>` with the desired version (e.g., `v0.0.4`, `v0.0.5`, etc.) when deploying.

## Requirements

- Ensure **Docker** and **Docker Compose** are installed on your machine.

---

## **Docker Compose Deployment**

### **1. Navigate to the Docker Compose Folder**
   - Locate the `docker-compose` folder in this repository.
   - Inside, you will find a file named `docker-compose.yaml`.

### **2. Configure Environment Variables**
   - Create a `.env` file in the same directory as `docker-compose.yaml`. This file should contain the following variables:
     - `PRIVATE_KEY`: Your private key for the deployment.
     - `DEPLOYED_CONTRACT`: The contract address. Initially, leave this empty during the first deployment to retrieve the deployed contract.

   - Example `.env` file:
     ```plaintext
     PRIVATE_KEY=myprivatekey
     DEPLOYED_CONTRACT=
     ```

### **3. Retrieve Deployed Contract**
   - Deploy the feeder with `DEPLOYED_CONTRACT` empty.
   - Upon the first deployment, the logs will display the deployed contract address in the following format:
     ```plaintext
     │ time="2024-11-25T11:30:08Z" level=info msg="Contract pending deploy: 0x708e54f09a8b0xxxxxxxxxxxxxxxx."
     ```
   - Copy the displayed contract address (e.g., `0x708e54f09a8b0xxxxxxxxxxxxxxxx`) and paste it into your `.env` file as the value for `DEPLOYED_CONTRACT`.

   - Update your `.env` file:
     ```plaintext
     PRIVATE_KEY=myprivatekey
     DEPLOYED_CONTRACT=0x708e54f09a8b0xxxxxxxxxxxxxxxx
     ```

### **4. Run Docker Compose**
   - Open a terminal in the `docker-compose` folder and start the deployment by running:
     ```bash
     docker-compose up -d
     ```

---

## **Verification**

### **4. Verify Logs**
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

---

## **Alternative Deployment Methods**

### **2. Docker Run Deployment**

This method is suitable for simple setups without orchestration.

#### **Command**
   - Deploy the feeder with `DEPLOYED_CONTRACT` initially empty:
     ```bash
     docker run -d \
       -e PRIVATE_KEY=myprivatekey \
       -e DEPLOYED_CONTRACT= \
       --name decentralized-feeder \
       diadata/decentralized-feeder:<VERSION>
     ```
   - Retrieve the logs to get the deployed contract address:
     ```bash
     docker logs decentralized-feeder
     ```
   - Stop the container, update the `DEPLOYED_CONTRACT` value, and restart:
     ```bash
     docker stop decentralized-feeder
     docker run -d \
       -e PRIVATE_KEY=myprivatekey \
       -e DEPLOYED_CONTRACT=0x708e54f09a8b0xxxxxxxxxxxxxxxx \
       --name decentralized-feeder \
       diadata/decentralized-feeder:<VERSION>
     ```

---

### **Error Handling**
   - If any issues arise during deployment, follow these steps:
     1. **Check Logs**: View the logs to identify error messages. Use:
        ```bash
        docker-compose logs -f
        ```
     2. **Verify Environment Variables**: Ensure all required environment variables (`PRIVATE_KEY`, `DEPLOYED_CONTRACT`) are correctly set in the `.env` file.
     3. **Restart Deployment**: If needed, restart the deployment by bringing down and restarting the container:
        ```bash
        docker-compose down
        docker-compose up -d
        ```

---

### **2. Kubernetes Deployment**

Kubernetes is ideal for production environments requiring scalability and high availability.

#### **Deployment YAML**
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
               value: "myprivatekey"
             - name: DEPLOYED_CONTRACT
               value: ""
             ports:
             - containerPort: 8080
     ```

#### **Steps to Deploy**
   1. Deploy the feeder with `DEPLOYED_CONTRACT` set to an empty string (`""`) in the Kubernetes manifest.
   2. Monitor the logs for the deployed contract address:
      ```bash
      kubectl logs <pod-name>
      ```
   3. Update the `DEPLOYED_CONTRACT` value in the manifest with the retrieved contract address.
   4. Apply the updated manifest:
      ```bash
      kubectl apply -f deployment.yaml
      ```

---



## **Conclusion**

The `diadata/decentralized-feeder:<VERSION>` image can be deployed using various methods to accommodate different use cases. For production environments, Kubernetes or Helm is recommended for scalability and flexibility. For simpler setups or local testing, Docker Compose or Docker Run is sufficient.

If you encounter any issues or need further assistance, feel free to reach out to the team.
