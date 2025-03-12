<div style="text-align: center;">
    <img src="./assets/DIA_logo.png" alt="Dia logo" width="200" height="auto" style="padding: 20px;">
</div>

# decentral-feeder
![Docker Image Version](https://img.shields.io/docker/v/diadata/decentralized-feeder?sort=semver)
![Build Status](https://img.shields.io/github/actions/workflow/status/diadata-org/decentral-feeder/master-pipeline.yml?branch=master)
![GitHub contributors](https://img.shields.io/github/contributors/diadata-org/decentral-feeder)
![GitHub commit activity](https://img.shields.io/github/commit-activity/y/diadata-org/decentral-feeder)
![GitHub stars](https://img.shields.io/github/stars/diadata-org/decentral-feeder?style=social)
[![Twitter Follow](https://img.shields.io/twitter/follow/DIAdata_org?style=social)](https://twitter.com/your-twitter-handle)

The node setup instructions are available in our [Wiki]() page!

This repository hosts a self-contained containerized application for running a data feeder in the Lumina oracle network. It comprises of three main components: scraper, collector, and processor. The scraper collects trade data from various CEXs and DEXs. The collector and processor aggregate the data through a two-step process to produce a scalar value associated with an asset, which is subsequently published on-chain. In most cases, this value represents the asset's price in USD.

<div style="text-align: center;">
    <img src="assets/Feeder_Architecture_v2.png" alt="Feeder Architecture v2" width="800" height="auto">
    <p style="font-style: italic;">Decentralized feeders components</p>
</div>

## Documentation
For node deployment instructions, you can visit our [Wiki](https://github.com/diadata-org/decentral-feeder/wiki) page. 

To learn about DIA's oracle stacks, you can visit our documentation [here](https://docs.diadata.org/). 

## Issues
To report bugs or suggest enhancements, you can create a [Github Issue](https://docs.github.com/en/issues/tracking-your-work-with-issues/using-issues/creating-an-issue) in the repository.

## Contribution Guidelines
Coming soon...

## Community
You can find our team on the following channels:
- [Discord](https://discord.com/invite/RjHBcZ9mEH)
- [Telegram](https://t.me/diadata_org)
- [X](https://x.com/DIAdata_org)