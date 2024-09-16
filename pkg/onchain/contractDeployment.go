package onchain

import (
	"time"

	diaOracleV2MultiupdateService "github.com/diadata-org/diadata/pkg/dia/scraper/blockchain-scrapers/blockchains/ethereum/diaOracleV2MultiupdateService"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func DeployOrBindContract(
	deployedContract string,
	conn *ethclient.Client,
	connBackup *ethclient.Client,
	auth *bind.TransactOpts,
	contract **diaOracleV2MultiupdateService.DiaOracleV2MultiupdateService,
	contractBackup **diaOracleV2MultiupdateService.DiaOracleV2MultiupdateService) error {
	var err error
	if deployedContract != "" {
		// bind primary and backup
		*contract, err = diaOracleV2MultiupdateService.NewDiaOracleV2MultiupdateService(common.HexToAddress(deployedContract), conn)
		if err != nil {
			return err
		}
		*contractBackup, err = diaOracleV2MultiupdateService.NewDiaOracleV2MultiupdateService(common.HexToAddress(deployedContract), connBackup)
		if err != nil {
			return err
		}
	} else {
		// deploy contract
		var addr common.Address
		var tx *types.Transaction
		addr, tx, *contract, err = diaOracleV2MultiupdateService.DeployDiaOracleV2MultiupdateService(auth, conn)
		if err != nil {
			log.Fatalf("could not deploy contract: %v", err)
			return err
		}
		log.Infof("Contract pending deploy: 0x%x.", addr)
		log.Infof("Transaction waiting to be mined: 0x%x.", tx.Hash())
		// bind backup
		*contractBackup, err = diaOracleV2MultiupdateService.NewDiaOracleV2MultiupdateService(addr, connBackup)
		if err != nil {
			return err
		}
		time.Sleep(180000 * time.Millisecond)
	}
	return nil
}
