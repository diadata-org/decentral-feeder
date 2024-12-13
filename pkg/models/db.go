package models

import (
	"context"
	"fmt"
	"strconv"

	"github.com/diadata-org/decentral-feeder/pkg/db"
	"github.com/kwilteam/kwil-db/core/client"
	ctypes "github.com/kwilteam/kwil-db/core/types/client"
	"github.com/kwilteam/kwil-db/core/types/transactions"
	log "github.com/sirupsen/logrus"
)

type DB struct {
	kwilClient *client.Client
	kwilDBID   string
}

type Datastore interface {
	SetTrade(trade Trade) error
	SetExchange(exchange Exchange) error
	GetExchange(name string) (Exchange, error)
	GetExchanges(ctx context.Context) error
}

func NewDataStore(chainIDKwil string, providerKwil string) (*DB, error) {
	kwilClient := db.GetKwilClient(chainIDKwil, providerKwil)
	return &DB{kwilClient: kwilClient}, nil
}

func (datastore *DB) DeployDatabase(dbName string, dbSchemaPath string) (bool, error) {
	dbID, deployed, err := db.DeployDatabase(datastore.kwilClient, dbName, dbSchemaPath)
	datastore.kwilDBID = dbID
	log.Info("dbID: ", dbID)
	return deployed, err
}

func (datastore *DB) DropDatabase(ctx context.Context, dbName string) (transactions.TxHash, error) {
	return datastore.kwilClient.DropDatabase(ctx, dbName, []ctypes.TxOpt{ctypes.WithSyncBroadcast(true)}...)
}

func (datastore *DB) SetExchange(ctx context.Context, exchange Exchange) error {
	const setAction = "set_exchange"
	txOpts := []ctypes.TxOpt{ctypes.WithSyncBroadcast(true)}

	input := [][]any{{exchange.Name, exchange.Centralized, exchange.Blockchain}}
	txHash, err := datastore.kwilClient.Execute(ctx, datastore.kwilDBID, setAction, input, txOpts...)
	if err != nil {
		return err
	}
	log.Info("txHash: ", txHash)
	return nil
}

func (datastore *DB) GetExchange(ctx context.Context, name string) (exchange Exchange, err error) {
	var callResp *ctypes.CallResult
	callResp, err = datastore.kwilClient.Call(ctx, datastore.kwilDBID, "get_exchange", []any{name})
	if err != nil {
		return
	}

	records := callResp.Records
	if records == nil {
		log.Warnf("No data records in the 'exchange' table with name %s.", name)
		return
	}
	maps := records.ExportString()
	for _, m := range maps {
		exchange.Name = name
		for key, value := range m {
			switch key {
			case "blockchain":
				exchange.Blockchain = value
			case "centralized":
				exchange.Centralized, err = strconv.ParseBool(value)
				if err != nil {
					log.Error("parse `centralized`: ", err)
				}
			}
		}
	}
	return
}

func (datastore *DB) GetExchanges(ctx context.Context) error {
	const getAllAction = "get_all_exchanges"
	fmt.Printf("Retrieving all entries from the 'exchange' table using action %q...\n", getAllAction)

	callResp, err := datastore.kwilClient.Call(ctx, datastore.kwilDBID, getAllAction, nil)
	if err != nil {
		return err
	}

	records := callResp.Records
	if records == nil {
		fmt.Println("No data records in the 'exchange' table.")
		return err
	}

	tab := records.ExportString()
	if len(tab) == 0 {
		fmt.Println("No data records in the 'exchange' table.")
	} else {
		fmt.Println("All entries in the 'exchange' table:")
		var headers []string
		for k := range tab[0] {
			headers = append(headers, k)
		}
		fmt.Printf("Column names: %v\n", headers)
		fmt.Println("Values:")
		for _, row := range tab {
			var rowVals []string
			for _, h := range headers {
				rowVals = append(rowVals, row[h])
			}
			fmt.Printf("%v\n", rowVals)
		}
	}
	return nil
}

func (datastore *DB) SetTrade(trade Trade) error {
	// TO DO
	// datastore.kwilClient.ExecuteAction()
	return nil
}
