package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kwilteam/kwil-db/core/client"
	"github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	klog "github.com/kwilteam/kwil-db/core/log"
	ctypes "github.com/kwilteam/kwil-db/core/types/client"
)

const (
	chainID  = "kwil-chain-S9bvaqFI"
	provider = "http://127.0.0.1:8484"
	privKey  = "9167061a722d41dd5fb374c37bd6ed10ddd1b46c7d0016a5aaedae83c520fb00"
)

func main() {
	ctx := context.Background()
	signer := makeEthSigner(privKey)
	acctID := signer.Identity()

	opts := &ctypes.Options{
		Logger:  klog.NewStdOut(klog.InfoLevel),
		ChainID: chainID,
		Signer:  signer,
	}

	// Create the client and connect to the RPC provider.
	cl, err := client.NewClient(ctx, provider, opts)
	if err != nil {
		log.Fatal(err)
	}

	// Define the database name.
	dbName := "was_here"

	// Check if the database exists.
	databases, err := cl.ListDatabases(ctx, acctID)
	if err != nil {
		log.Fatal(err)
	}

	deployed := false
	for _, db := range databases {
		if db.Name == dbName {
			deployed = true
			break
		}
	}

	if deployed {
		// Drop the database.
		fmt.Printf("Dropping database %v...\n", dbName)
		txHash, err := cl.DropDatabase(ctx, dbName, ctypes.WithSyncBroadcast(true))
		if err != nil {
			log.Fatal(err)
		}

		// Wait for the transaction to be included in a block.
		err = waitForTx(cl, ctx, txHash)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Database %v dropped successfully.\n", dbName)
	} else {
		fmt.Printf("Database %v does not exist.\n", dbName)
	}
}

func makeEthSigner(keyHex string) auth.Signer {
	key, err := crypto.Secp256k1PrivateKeyFromHex(keyHex)
	if err != nil {
		panic(fmt.Sprintf("bad private key: %v", err))
	}
	return &auth.EthPersonalSigner{Key: *key}
}

func waitForTx(cl *client.Client, ctx context.Context, txHash []byte) error {
	res, err := cl.WaitTx(ctx, txHash, 250*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to wait for transaction: %v", err)
	}
	if res.TxResult.Code != 0 {
		return fmt.Errorf("transaction failed with code %d, log: %s", res.TxResult.Code, res.TxResult.Log)
	}
	return nil
}
