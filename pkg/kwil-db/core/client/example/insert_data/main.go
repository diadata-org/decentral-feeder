package main

import (
	"context"
	"fmt"
	"log"
	"time"

	// Removed "slices" package for compatibility with older Go versions
	"github.com/kwilteam/kwil-db/core/client"
	"github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	klog "github.com/kwilteam/kwil-db/core/log"
	ctypes "github.com/kwilteam/kwil-db/core/types/client"
	"github.com/kwilteam/kwil-db/core/types/transactions"
	"github.com/kwilteam/kwil-db/core/utils"
	"github.com/kwilteam/kwil-db/parse"
)

const (
	chainID  = "kwil-chain-nSRNXdbH"
	provider = "http://localhost:8484/"
	privKey  = "9167061a722d41dd5fb374c37bd6ed10ddd1b46c7d0016a5aaedae83c520fb00" // Replace with your actual private key
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

	// Database name
	dbName := "was_here"
	dbID := utils.GenerateDBID(dbName, acctID)

	// Check if the database is already deployed
	databases, err := cl.ListDatabases(ctx, acctID)
	if err != nil {
		log.Fatal(err)
	}

	// Use a loop to check if the database is deployed
	deployed := false
	for _, db := range databases {
		if db.Name == dbName {
			deployed = true
			break
		}
	}

	if !deployed {
		fmt.Printf("Deploying database: %v...\n", dbName)

		// Parse the Kuneiform schema
		schema, err := parse.Parse([]byte(testKf))
		if err != nil {
			log.Fatal(err)
		}

		// Deploy the database
		txHash, err := cl.DeployDatabase(ctx, schema)
		if err != nil {
			log.Fatal(err)
		}

		// Set a timeout context for waiting on the transaction
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		checkTx(cl, ctxWithTimeout, txHash, "deploy database")
	} else {
		fmt.Printf("Database %v is already deployed.\n", dbName)
	}

	// Insert data into the database using the "tag" action
	const tagAction = "tag"
	data := "test message 2024-09-30 02"
	fmt.Printf("Inserting data into database %v using action %q...\n", dbName, tagAction)
	txHash, err := cl.Execute(ctx, dbID, tagAction, [][]any{{data}}, ctypes.WithSyncBroadcast(true))
	if err != nil {
		log.Fatal(err)
	}

	// Use a new timeout context for the insert transaction
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	checkTx(cl, ctxWithTimeout, txHash, "insert data")
}

func checkTx(cl *client.Client, ctx context.Context, txHash []byte, action string) {
	// Wait for the transaction to be included in a block, checking every 250ms
	res, err := cl.WaitTx(ctx, txHash, 250*time.Millisecond)
	if err != nil {
		log.Fatalf("Failed to wait for transaction %x: %v", txHash, err)
	}
	if res.TxResult.Code == transactions.CodeOk.Uint32() {
		fmt.Printf("Success: %q in transaction %x\n", action, txHash)
	} else {
		log.Fatalf("Fail: %q in transaction %x, Result code %d, log: %q",
			action, txHash, res.TxResult.Code, res.TxResult.Log)
	}
}

func makeEthSigner(keyHex string) auth.Signer {
	key, err := crypto.Secp256k1PrivateKeyFromHex(keyHex)
	if err != nil {
		panic(fmt.Sprintf("bad private key: %v", err))
	}
	return &auth.EthPersonalSigner{Key: *key}
}

var testKf = `database was_here;

table tags {
    id uuid primary key,
    ident text not null,
    val int default(42),
    msg text not null
}

action tag($msg) public {
    INSERT INTO tags (id, ident, msg) VALUES (
        uuid_generate_v5('69c7f28c-b681-4d89-b4d9-8c8211065585'::uuid, @txid),
        @caller,
        $msg);
}
`
