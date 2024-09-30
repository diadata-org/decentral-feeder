package kwildb

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

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
	provider = "http://127.0.0.1:8484"
)

// DeployAndListTags function contains the main logic, exported for use in main.go
func DeployAndListTags() {
	privKey := os.Getenv("PRIV_KEY")
	if privKey == "" {
		log.Fatal("Private key not found in environment variables")
	}
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

	// List all entries in the tags table
	listTags(cl, ctx, dbID)
}

func listTags(cl *client.Client, ctx context.Context, dbID string) {
	const getAllAction = "get_all"
	fmt.Printf("Retrieving all entries from the 'tags' table using action %q...\n", getAllAction)

	callResp, err := cl.Call(ctx, dbID, getAllAction, nil)
	if err != nil {
		log.Fatal(err)
	}

	records := callResp.Records
	if records == nil {
		fmt.Println("No data records in the 'tags' table.")
		return
	}

	tab := records.ExportString()
	if len(tab) == 0 {
		fmt.Println("No data records in the 'tags' table.")
	} else {
		fmt.Println("All entries in the 'tags' table:")
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
}

func checkTx(cl *client.Client, ctx context.Context, txHash []byte, action string) {
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

action get_all() public view {
    SELECT * FROM tags;
}
`
