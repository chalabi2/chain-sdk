// Package v1beta4 provides the v1beta4 composite client for the Akash blockchain.
// It exposes the AEP-87 module versions — deployment/v1beta5 and market/v2beta1 —
// and reuses the v1beta3 transaction and node machinery, which is not coupled to
// module versions.
package v1beta4

import (
	"context"

	tmrpc "github.com/cometbft/cometbft/rpc/core/types"

	evdtypes "cosmossdk.io/x/evidence/types"
	feegranttypes "cosmossdk.io/x/feegrant"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdkclient "github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	v1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	slashtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	staketypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	atypes "pkg.akt.dev/go/node/audit/v1"
	btypes "pkg.akt.dev/go/node/bme/v1"
	ctypes "pkg.akt.dev/go/node/cert/v1"
	cltypes "pkg.akt.dev/go/node/client/types"
	"pkg.akt.dev/go/node/client/v1beta3"
	dtypes "pkg.akt.dev/go/node/deployment/v1beta5"
	etypes "pkg.akt.dev/go/node/escrow/v1"
	mtypes "pkg.akt.dev/go/node/market/v2beta1"
	otypes "pkg.akt.dev/go/node/oracle/v2"
	ptypes "pkg.akt.dev/go/node/provider/v1beta4"
)

// Re-export broadcast types that callers may need. The broadcast machinery is
// version-agnostic and lives in v1beta3.
type (
	BroadcastOption  = v1beta3.BroadcastOption
	BroadcastOptions = v1beta3.BroadcastOptions
	ConfirmFn        = v1beta3.ConfirmFn
)

// QueryClient is the interface that exposes query modules.
type QueryClient interface {
	Deployment() dtypes.QueryClient
	Escrow() etypes.QueryClient
	Market() mtypes.QueryClient
	Provider() ptypes.QueryClient
	Audit() atypes.QueryClient
	Certs() ctypes.QueryClient
	Auth() authtypes.QueryClient
	Authz() authz.QueryClient
	Bank() banktypes.QueryClient
	Distribution() disttypes.QueryClient
	Evidence() evdtypes.QueryClient
	Feegrant() feegranttypes.QueryClient
	GovLegacy() govtypes.QueryClient
	Gov() v1.QueryClient
	Mint() minttypes.QueryClient
	Slashing() slashtypes.QueryClient
	Staking() staketypes.QueryClient
	Upgrade() upgradetypes.QueryClient
	Params() paramstypes.QueryClient
	Wasm() wasmtypes.QueryClient
	Oracle() otypes.QueryClient
	BME() btypes.QueryClient

	ClientContext() sdkclient.Context
}

// TxClient is the interface that wraps the Broadcast method.
// Broadcast broadcasts a transaction. A transaction is composed of 1 or many messages. This allows several
// operations to be performed in a single transaction.
// A transaction broadcast can be configured with an arbitrary number of BroadcastOption.
type TxClient interface {
	BroadcastMsgs(context.Context, []sdk.Msg, ...BroadcastOption) (interface{}, error)
	BroadcastTx(context.Context, sdk.Tx, ...BroadcastOption) (interface{}, error)
}

type NodeClient interface {
	SyncInfo(context.Context) (*tmrpc.SyncInfo, error)
	CurrentBlockHeight(context.Context) (int64, error)
}

// LightClient is the umbrella interface that exposes every other client's modules.
type LightClient interface {
	Query() QueryClient
	Node() NodeClient
	ClientContext() sdkclient.Context
	PrintMessage(interface{}) error
	PrintJSON(msg interface{}) error
}

// Client is the umbrella interface that exposes every other client's modules.
type Client interface {
	LightClient
	Tx() TxClient
}

type lightClient struct {
	inner   v1beta3.LightClient
	qclient *queryClient
}

type client struct {
	lightClient
	tx TxClient
}

var (
	_ Client      = (*client)(nil)
	_ LightClient = (*lightClient)(nil)
)

// NewClient creates a new client. The transaction and node plumbing is shared
// with v1beta3; the query surface is built from this package's module versions.
func NewClient(ctx context.Context, cctx sdkclient.Context, opts ...cltypes.ClientOption) (Client, error) {
	inner, err := v1beta3.NewClient(ctx, cctx, opts...)
	if err != nil {
		return nil, err
	}

	cl := &client{
		lightClient: lightClient{
			inner:   inner,
			qclient: newQueryClient(inner.ClientContext()),
		},
		tx: inner.Tx(),
	}

	return cl, nil
}

// NewLightClient creates a new light client.
func NewLightClient(cctx sdkclient.Context) (LightClient, error) {
	inner, err := v1beta3.NewLightClient(cctx)
	if err != nil {
		return nil, err
	}

	cl := &lightClient{
		inner:   inner,
		qclient: newQueryClient(inner.ClientContext()),
	}

	return cl, nil
}

// NewQueryClient creates new query client instance based on a Cosmos SDK client context.
func NewQueryClient(cctx sdkclient.Context) QueryClient {
	return newQueryClient(cctx)
}

// Tx implements Client by returning the TxClient instance of the client.
func (cl *client) Tx() TxClient {
	return cl.tx
}

// Query implements Client by returning the QueryClient instance of the client.
func (cl *lightClient) Query() QueryClient {
	return cl.qclient
}

// Node implements Client by returning the NodeClient instance of the client.
func (cl *lightClient) Node() NodeClient {
	return cl.inner.Node()
}

// ClientContext implements Client by returning the Cosmos SDK client context instance of the client.
func (cl *lightClient) ClientContext() sdkclient.Context {
	return cl.qclient.cctx
}

// PrintMessage implements Client by printing the raw message passed as parameter.
func (cl *lightClient) PrintMessage(msg interface{}) error {
	return cl.inner.PrintMessage(msg)
}

// PrintJSON implements Client by printing the message passed as parameter as JSON.
func (cl *lightClient) PrintJSON(msg interface{}) error {
	return cl.inner.PrintJSON(msg)
}
