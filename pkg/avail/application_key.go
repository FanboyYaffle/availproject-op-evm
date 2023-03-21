package avail

import (
	"errors"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
)

const (
	// ApplicationKey is the App Key that distincts Avail Settlement Layer
	// data in Avail.
	ApplicationKey = "avail-settlement"

	// CallCreateApplicationKey is the RPC API call for creating new AppID on Avail.
	CallCreateApplicationKey = "DataAvailability.create_application_key"
)

var (
	// DefaultAppID is the Avail application ID.
	DefaultAppID = types.NewUCompactFromUInt(0)

	ErrAppIDNotFound = errors.New("AppID not found")
)

func EnsureApplicationKeyExists(client Client, applicationKey string, signingKeyPair signature.KeyringPair) (types.UCompact, error) {
	appID, err := QueryAppID(client, applicationKey)
	if errors.Is(err, ErrAppIDNotFound) {
		appID, err = CreateApplicationKey(client, applicationKey, signingKeyPair)
		if err != nil {
			return types.NewUCompactFromUInt(0), err
		}
	} else if err != nil {
		fmt.Printf("error while querying appID: %#v\n", err)
		return types.NewUCompactFromUInt(0), err
	}

	return appID, nil
}

func QueryAppID(client Client, applicationKey string) (types.UCompact, error) {
	api := client.instance()

	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return types.NewUCompactFromUInt(0), err
	}

	encodedAppKey, err := codec.Encode([]byte(applicationKey))
	if err != nil {
		return types.NewUCompactFromUInt(0), err
	}

	key, err := types.CreateStorageKey(meta, "DataAvailability", "AppKeys", encodedAppKey)
	if err != nil {
		return types.NewUCompactFromUInt(0), err
	}

	type AppKeyInfo struct {
		AccountID types.AccountID
		AppID     types.UCompact
	}

	var aki AppKeyInfo
	ok, err := api.RPC.State.GetStorageLatest(key, &aki)
	if err != nil {
		fmt.Printf("!!! failed to get the latest storage for appID\n")
		return types.NewUCompactFromUInt(0), err
	}

	if ok {
		return aki.AppID, nil
	} else {
		fmt.Printf("!!!! couldn't decode AppKeyInfo")
		return types.NewUCompactFromUInt(0), ErrAppIDNotFound
	}
}

func CreateApplicationKey(client Client, applicationKey string, signingKeyPair signature.KeyringPair) (types.UCompact, error) {
	api := client.instance()

	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return types.NewUCompactFromUInt(0), err
	}

	call, err := types.NewCall(meta, CallCreateApplicationKey, []byte(applicationKey))
	if err != nil {
		return types.NewUCompactFromUInt(0), err
	}

	ext := types.NewExtrinsic(call)

	rv, err := api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return types.NewUCompactFromUInt(0), err
	}

	key, err := types.CreateStorageKey(meta, "System", "Account", signingKeyPair.PublicKey)
	if err != nil {
		return types.NewUCompactFromUInt(0), err
	}

	var accountInfo types.AccountInfo
	ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil || !ok {
		return types.NewUCompactFromUInt(0), fmt.Errorf("couldn't fetch latest account storage info: %w", err)
	}

	genesisHash, err := api.RPC.Chain.GetBlockHash(0)
	if err != nil {
		return types.NewUCompactFromUInt(0), err
	}

	nonce := uint64(accountInfo.Nonce)
	o := types.SignatureOptions{
		// This transaction is Immortal (https://wiki.polkadot.network/docs/build-protocol-info#transaction-mortality)
		// Hence BlockHash: Genesis Hash.
		BlockHash:          genesisHash,
		Era:                types.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              types.NewUCompactFromUInt(nonce),
		SpecVersion:        rv.SpecVersion,
		Tip:                types.NewUCompactFromUInt(100),
		AppID:              DefaultAppID,
		TransactionVersion: rv.TransactionVersion,
	}

	err = ext.Sign(signingKeyPair, o)
	if err != nil {
		return types.NewUCompactFromUInt(0), err
	}

	sub, err := api.RPC.Author.SubmitAndWatchExtrinsic(ext)
	if err != nil {
		return types.NewUCompactFromUInt(0), err
	}

	defer sub.Unsubscribe()

	for {
		select {
		case status := <-sub.Chan():
			if status.IsInBlock {
				return QueryAppID(client, applicationKey)
			}

			if status.IsDropped || status.IsInvalid {
				return types.NewUCompactFromUInt(0), fmt.Errorf("unexpected extrinsic status from Avail: %#v", status)
			}

		case err = <-sub.Err():
			return types.NewUCompactFromUInt(0), fmt.Errorf("error while waiting for application key creation status: %w", err)
		}
	}
}
