package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	cli "gopkg.in/urfave/cli.v1"

	"github.com/Fantom-foundation/go-opera/inter/validator"
	"github.com/Fantom-foundation/go-opera/valkeystore"
)

func addFakeValidatorKey(ctx *cli.Context, pubkey validator.PubKey, valKeystore valkeystore.RawKeystoreI) {
	// add fake validator key
	if ctx.GlobalIsSet(FakeNetFlag.Name) {
		key := getFakeValidatorKey(ctx)
		if key != nil && !valKeystore.Has(pubkey) {
			err := valKeystore.Add(pubkey, crypto.FromECDSA(key), "fakepassword")
			if err != nil {
				utils.Fatalf("Failed to add fake validator key: %v", err)
			}
			log.Info("Added fake validator key", "pubkey", pubkey.String())
		}
	}
}

func getValKeystoreDir(cfg node.Config) string {
	_, _, keydir, err := cfg.AccountConfig()
	if err != nil {
		utils.Fatalf("Failed to setup account config: %v", err)
	}
	return keydir
}

// makeValidatorPasswordList reads password lines from the file specified by the global --validator.password flag.
func makeValidatorPasswordList(ctx *cli.Context) []string {
	if path := ctx.GlobalString(validatorPasswordFlag.Name); path != "" {
		text, err := ioutil.ReadFile(path)
		if err != nil {
			utils.Fatalf("Failed to read password file: %v", err)
		}
		lines := strings.Split(string(text), "\n")
		// Sanitise DOS line endings.
		for i := range lines {
			lines[i] = strings.TrimRight(lines[i], "\r")
		}
		return lines
	}
	if ctx.GlobalIsSet(FakeNetFlag.Name) {
		return []string{"fakepassword"}
	}
	return nil
}

func unlockValidatorKey(ctx *cli.Context, pubKey validator.PubKey, valKeystore valkeystore.KeystoreI) error {
	var err error
	for trials := 0; trials < 3; trials++ {
		prompt := fmt.Sprintf("Unlocking validator key %s | Attempt %d/%d", pubKey.String(), trials+1, 3)
		password := getPassPhrase(prompt, false, 0, makeValidatorPasswordList(ctx))
		err = valKeystore.Unlock(pubKey, password)
		if err == nil {
			log.Info("Unlocked validator key", "pubkey", pubKey.String())
			return nil
		}
	}
	// All trials expended to unlock account, bail out
	return err
}
