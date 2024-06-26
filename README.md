# Overview

This repository consists of a self-contained data collection, processing and publishing pipeline. More precisely, scrapers are collecting trades data from various centralized and decentralized exchanges.
Thus obtained trades are then processed in a 2-step aggregation procedure in order to come up with an asset price that is subsequently published on-chain.

# Detailed Description of the Building Blocks
In the following, we describe function and usage of the constituting building blocks (see figure). We proceed from bottom to top.

## Scrapers
Each scraper is implemented in a dedicated file in the folder /pkg/scraper. Its function is to continuously fetch trades data from a given exchange and send them to the channel `tradesChannel` obtained from the function `RunScraper`.
The expected input for a scraper is a set of pair tickers such as `BTC-USDT`. Tickers are always capitalized and symbols separated by a hyphen. It's the role of the scraper to format the pair ticker such that it can subscribe to
 the corresponding (websocket) stream.
