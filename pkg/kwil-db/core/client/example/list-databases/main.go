package main

import (
	"context"
	"fmt"
	"log"

	"github.com/kwilteam/kwil-db/core/client"
	"github.com/kwilteam/kwil-db/core/crypto"
	"github.com/kwilteam/kwil-db/core/crypto/auth"
	klog "github.com/kwilteam/kwil-db/core/log"
	ctypes "github.com/kwilteam/kwil-db/core/types/client"
)

const (
	chainID  = "kwil-chain-nSRNXdbH"
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

	// List databases owned by the account.
	databases, err := cl.ListDatabases(ctx, acctID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d database(s):\n", len(databases))
	for _, db := range databases {
		fmt.Println(" -", db.Name)
	}
}

func makeEthSigner(keyHex string) auth.Signer {
	key, err := crypto.Secp256k1PrivateKeyFromHex(keyHex)
	if err != nil {
		panic(fmt.Sprintf("bad private key: %v", err))
	}
	return &auth.EthPersonalSigner{Key: *key}
}
