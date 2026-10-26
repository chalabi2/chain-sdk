package cli

import (
	"bytes"
	"context"
	"fmt"

	"github.com/spf13/cobra"

	sdkclient "github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"

	cflags "pkg.akt.dev/go/cli/flags"
	mv1 "pkg.akt.dev/go/node/market/v1"
	mtypes "pkg.akt.dev/go/node/market/v1beta5"
)

// GetTxMarketCmds returns the transaction commands for market module
func GetTxMarketCmds() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        mv1.ModuleName,
		Short:                      "Transaction subcommands",
		SuggestionsMinimumDistance: 2,
		RunE:                       sdkclient.ValidateCmd,
	}
	cmd.AddCommand(
		GetTxMarketBidCmds(),
		GetTxMarketLeaseCmds(),
	)
	return cmd
}

func GetTxMarketBidCmds() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "bid",
		Short:                      "Bid subcommands",
		SuggestionsMinimumDistance: 2,
		RunE:                       sdkclient.ValidateCmd,
	}
	cmd.AddCommand(
		GetTxMarketBidCreateCmd(),
		GetTxMarketBidCloseCmd(),
		GetTxMarketBidStartReclaimCmd(),
	)
	return cmd
}

func GetTxMarketBidCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "create",
		Short:             "Create a market bid",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			price, err := cmd.Flags().GetString("price")
			if err != nil {
				return err
			}

			coin, err := sdk.ParseDecCoin(price)
			if err != nil {
				return err
			}

			id, err := cflags.OrderIDFromFlags(cmd.Flags(), cflags.WithProvider(cctx.FromAddress))
			if err != nil {
				return err
			}

			deposit, err := DetectDeposit(ctx, cmd.Flags(), cl.Query(), DetectBidDeposit)
			if err != nil {
				return err
			}

			reclaimWindow, err := cflags.ReclamationWindowFromFlags(cmd.Flags())
			if err != nil {
				return err
			}

			// a bid on a volume order must carry the order's volume policy
			order, err := cl.Query().Market().Order(ctx, &mtypes.QueryOrderRequest{ID: id})
			if err != nil {
				return err
			}

			msg := &mtypes.MsgCreateBid{
				ID:                mv1.MakeBidID(id, cctx.GetFromAddress()),
				Price:             coin,
				Deposit:           deposit,
				ReclamationWindow: reclaimWindow,
				VolumePolicy:      order.Order.Spec.Volume,
			}

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			resp, err := cl.Tx().BroadcastMsgs(ctx, []sdk.Msg{msg})
			if err != nil {
				return err
			}

			return cl.PrintMessage(resp)
		},
	}

	cflags.AddTxFlagsToCmd(cmd)
	cflags.AddOrderIDFlags(cmd.Flags())

	cmd.Flags().String("price", "", "Bid Price")
	cflags.AddDepositFlags(cmd.Flags())
	cflags.AddReclamationWindowFlag(cmd.Flags())

	return cmd
}

func GetTxMarketBidCloseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "close",
		Short:             "Close a market bid",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			id, err := cflags.BidIDFromFlags(cmd.Flags(), cflags.WithProvider(cctx.FromAddress))
			if err != nil {
				return err
			}

			reason, err := cflags.BidClosedReasonFromFlags(cmd.Flags())
			if err != nil {
				return err
			}

			msg := &mtypes.MsgCloseBid{
				ID:     id,
				Reason: reason,
			}

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			resp, err := cl.Tx().BroadcastMsgs(ctx, []sdk.Msg{msg})
			if err != nil {
				return err
			}

			return cl.PrintMessage(resp)
		},
	}

	cflags.AddTxFlagsToCmd(cmd)
	cflags.AddBidIDFlags(cmd.Flags())
	cflags.AddBidClosedReasonFlag(cmd.Flags())

	return cmd
}

func GetTxMarketBidStartReclaimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "start-reclaim",
		Short:             "Start reclamation on an active lease (provider only)",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			id, err := cflags.LeaseIDFromFlags(cmd.Flags(), cflags.WithProvider(cctx.FromAddress))
			if err != nil {
				return err
			}

			reason, err := cflags.BidClosedReasonFromFlags(cmd.Flags())
			if err != nil {
				return err
			}

			msg := mtypes.NewMsgLeaseStartReclaim(id, reason)

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			resp, err := cl.Tx().BroadcastMsgs(ctx, []sdk.Msg{msg})
			if err != nil {
				return err
			}

			return cl.PrintMessage(resp)
		},
	}

	cflags.AddTxFlagsToCmd(cmd)
	cflags.AddLeaseIDFlags(cmd.Flags())
	cflags.AddBidClosedReasonFlag(cmd.Flags())

	return cmd
}

func GetTxMarketLeaseCmds() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "lease",
		Short:                      "Lease subcommands",
		SuggestionsMinimumDistance: 2,
		RunE:                       sdkclient.ValidateCmd,
	}

	cmd.AddCommand(
		GetTxMarketLeaseCreateCmd(),
		GetTxMarketLeaseWithdrawCmd(),
		GetTxMarketLeaseCloseCmd(),
		GetTxMarketLeaseResizeAcceptCmd(),
		GetTxMarketLeaseResizeRejectCmd(),
		GetTxMarketLeaseResizeCancelCmd(),
	)

	return cmd
}

func GetTxMarketLeaseCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "create",
		Short:             "Create a market lease",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			id, err := cflags.LeaseIDFromFlags(cmd.Flags(), cflags.WithOwner(cctx.FromAddress))
			if err != nil {
				return err
			}

			msg := &mtypes.MsgCreateLease{
				BidID: id.BidID(),
			}

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			resp, err := cl.Tx().BroadcastMsgs(ctx, []sdk.Msg{msg})
			if err != nil {
				return err
			}

			return cl.PrintMessage(resp)
		},
	}

	cflags.AddTxFlagsToCmd(cmd)
	cflags.AddLeaseIDFlags(cmd.Flags())
	cflags.MarkReqLeaseIDFlags(cmd, cflags.DeploymentIDOptionNoOwner(true))

	return cmd
}

func GetTxMarketLeaseWithdrawCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "withdraw",
		Short:             "Settle and withdraw available funds from market order escrow account",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)

			cctx := cl.ClientContext()

			id, err := cflags.LeaseIDFromFlags(cmd.Flags(), cflags.WithOwner(cctx.FromAddress))
			if err != nil {
				return err
			}

			msg := &mtypes.MsgWithdrawLease{
				ID: id,
			}

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			resp, err := cl.Tx().BroadcastMsgs(ctx, []sdk.Msg{msg})
			if err != nil {
				return err
			}

			return cl.PrintMessage(resp)
		},
	}

	cflags.AddTxFlagsToCmd(cmd)
	cflags.AddLeaseIDFlags(cmd.Flags())
	cflags.MarkReqLeaseIDFlags(cmd, cflags.DeploymentIDOptionNoOwner(true))

	return cmd
}

func GetTxMarketLeaseCloseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "close",
		Short:             "Close a market order",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			id, err := cflags.LeaseIDFromFlags(cmd.Flags(), cflags.WithOwner(cctx.FromAddress))
			if err != nil {
				return err
			}

			// for lease closed tx reason is always owner
			msg := &mtypes.MsgCloseLease{
				ID:     id,
				Reason: mv1.LeaseClosedReasonOwner,
			}

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			resp, err := cl.Tx().BroadcastMsgs(ctx, []sdk.Msg{msg})
			if err != nil {
				return err
			}

			return cl.PrintMessage(resp)
		},
	}

	cflags.AddTxFlagsToCmd(cmd)
	cflags.AddLeaseIDFlags(cmd.Flags())
	cflags.MarkReqLeaseIDFlags(cmd, cflags.DeploymentIDOptionNoOwner(true))

	return cmd
}

func GetTxMarketLeaseResizeAcceptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "resize-accept",
		Short:             "Apply a lease's pending resize at the given price (provider only)",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			id, err := cflags.LeaseIDFromFlags(cmd.Flags(), cflags.WithProvider(cctx.FromAddress))
			if err != nil {
				return err
			}

			price, err := cmd.Flags().GetString(cflags.FlagPrice)
			if err != nil {
				return err
			}

			coin, err := sdk.ParseDecCoin(price)
			if err != nil {
				return err
			}

			digest, err := pendingLeaseResizeDigest(ctx, cl.Query().Market(), id)
			if err != nil {
				return err
			}

			msg := mtypes.NewMsgAcceptLeaseResize(id, coin, digest)

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			resp, err := cl.Tx().BroadcastMsgs(ctx, []sdk.Msg{msg})
			if err != nil {
				return err
			}

			return cl.PrintMessage(resp)
		},
	}

	cflags.AddTxFlagsToCmd(cmd)
	cflags.AddLeaseIDFlags(cmd.Flags())
	cmd.Flags().String(cflags.FlagPrice, "", "New per-block lease price")

	if err := cmd.MarkFlagRequired(cflags.FlagPrice); err != nil {
		panic(err.Error())
	}

	return cmd
}

func GetTxMarketLeaseResizeRejectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "resize-reject",
		Short:             "Decline a lease's pending resize (provider only)",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			id, err := cflags.LeaseIDFromFlags(cmd.Flags(), cflags.WithProvider(cctx.FromAddress))
			if err != nil {
				return err
			}

			reason, err := cflags.LeaseResizeRejectReasonFromFlags(cmd.Flags())
			if err != nil {
				return err
			}

			digest, err := pendingLeaseResizeDigest(ctx, cl.Query().Market(), id)
			if err != nil {
				return err
			}

			msg := mtypes.NewMsgRejectLeaseResize(id, reason, digest)

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			resp, err := cl.Tx().BroadcastMsgs(ctx, []sdk.Msg{msg})
			if err != nil {
				return err
			}

			return cl.PrintMessage(resp)
		},
	}

	cflags.AddTxFlagsToCmd(cmd)
	cflags.AddLeaseIDFlags(cmd.Flags())
	cflags.AddLeaseResizeRejectReasonFlag(cmd.Flags())

	return cmd
}

func GetTxMarketLeaseResizeCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "resize-cancel",
		Short:             "Withdraw a lease's pending resize",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			id, err := cflags.LeaseIDFromFlags(cmd.Flags(), cflags.WithOwner(cctx.FromAddress))
			if err != nil {
				return err
			}

			msg := mtypes.NewMsgCancelLeaseResize(id)

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			resp, err := cl.Tx().BroadcastMsgs(ctx, []sdk.Msg{msg})
			if err != nil {
				return err
			}

			return cl.PrintMessage(resp)
		},
	}

	cflags.AddTxFlagsToCmd(cmd)
	cflags.AddLeaseIDFlags(cmd.Flags())
	cflags.MarkReqLeaseIDFlags(cmd, cflags.DeploymentIDOptionNoOwner(true))

	return cmd
}

// pendingLeaseResizeDigest returns the digest of the lease's pending resize,
// which an accept or reject names so that it answers this request and no
// other.
func pendingLeaseResizeDigest(ctx context.Context, qc mtypes.QueryClient, id mv1.LeaseID) ([]byte, error) {
	req := &mtypes.QueryLeaseResizesRequest{
		Owner:    id.Owner,
		Provider: id.Provider,
	}

	for {
		res, err := qc.LeaseResizes(ctx, req)
		if err != nil {
			return nil, err
		}

		for _, resize := range res.LeaseResizes {
			if resize.LeaseID.Equals(id) {
				return resize.Digest, nil
			}
		}

		if res.Pagination == nil || len(res.Pagination.NextKey) == 0 {
			return nil, fmt.Errorf("%w: lease %s", mv1.ErrLeaseResizeNotFound, id)
		}

		if req.Pagination != nil && bytes.Equal(req.Pagination.Key, res.Pagination.NextKey) {
			return nil, fmt.Errorf("lease resizes pagination repeats key %x", res.Pagination.NextKey)
		}

		req.Pagination = &sdkquery.PageRequest{Key: res.Pagination.NextKey}
	}
}
