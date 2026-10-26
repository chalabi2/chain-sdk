package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	sdkclient "github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	cflags "pkg.akt.dev/go/cli/flags"
	dv1 "pkg.akt.dev/go/node/deployment/v1"
	dv1beta "pkg.akt.dev/go/node/deployment/v1beta5"
	mv1 "pkg.akt.dev/go/node/market/v1"
	mvbeta "pkg.akt.dev/go/node/market/v2beta1"
	"pkg.akt.dev/go/node/types/constants"
	"pkg.akt.dev/go/sdl"
	cutils "pkg.akt.dev/go/util/tls"
)

var (
	errDeploymentUpdate              = errors.New("deployment update failed")
	errDeploymentUpdateGroupsChanged = fmt.Errorf("%w: groups are different than existing deployment, you cannot update groups; use resize to change a group's resources", errDeploymentUpdate)
	errDeploymentResize              = errors.New("deployment resize failed")
)

// GetTxDeploymentCmds returns the transaction commands for this module
func GetTxDeploymentCmds() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        dv1.ModuleName,
		Short:                      "Deployment transaction subcommands",
		SuggestionsMinimumDistance: 2,
		RunE:                       sdkclient.ValidateCmd,
	}
	cmd.AddCommand(
		GetTxDeploymentCreateCmd(),
		GetTxDeploymentUpdateCmd(),
		GetTxDeploymentResizeCmd(),
		GetTxDeploymentCloseCmd(),
		GetTxDeploymentGroupCmds(),
	)
	return cmd
}

func GetTxDeploymentCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "create [sdl-file]",
		Short:             "Create deployment",
		Args:              cobra.ExactArgs(1),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			// first lets validate certificate exists for given account
			if _, err := cutils.LoadAndQueryCertificateForAccount(ctx, cctx, nil); err != nil {
				if os.IsNotExist(err) {
					err = fmt.Errorf("no certificate file found for account %q.\n"+
						"consider creating it as certificate required to submit manifest", cctx.FromAddress.String())
				}

				return err
			}

			// SDL v2.2 volume references compile to owner-keyed VolumeRefs;
			// the owner is always the tx signer. Ignored by earlier versions.
			sdlManifest, err := sdl.ReadFile(args[0], sdl.WithOwner(cctx.FromAddress.String()))
			if err != nil {
				return err
			}

			groups, err := sdlManifest.DeploymentGroups()
			if err != nil {
				return err
			}

			warnIfGroupVolumesExceeds(cctx, groups)

			id, err := cflags.DeploymentIDFromFlags(cmd.Flags(), cflags.WithOwner(cctx.FromAddress))
			if err != nil {
				return err
			}

			// Default DSeq to the current block height
			if id.DSeq == 0 {
				syncInfo, err := cl.Node().SyncInfo(ctx)
				if err != nil {
					return err
				}

				if syncInfo.CatchingUp {
					return fmt.Errorf("cannot generate DSEQ from last block height. node is catching up")
				}

				id.DSeq = uint64(syncInfo.LatestBlockHeight) // nolint: gosec
			}

			version, err := sdlManifest.Version()
			if err != nil {
				return err
			}

			dep, err := DetectDeposit(ctx, cmd.Flags(), cl.Query(), DetectDeploymentDeposit)
			if err != nil {
				return err
			}

			reclamation, err := sdlManifest.Reclamation()
			if err != nil {
				return err
			}

			msg := &dv1beta.MsgCreateDeployment{
				ID:          id,
				Hash:        version,
				Groups:      make(dv1beta.GroupSpecs, 0, len(groups)),
				Deposit:     dep,
				Reclamation: reclamation,
			}

			msg.Groups = append(msg.Groups, groups...)

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
	cflags.AddDeploymentIDFlags(cmd.Flags())
	cflags.AddDepositFlags(cmd.Flags())

	return cmd
}

func GetTxDeploymentCloseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "close",
		Short:             "Close deployment",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			id, err := cflags.DeploymentIDFromFlags(cmd.Flags(), cflags.WithOwner(cctx.FromAddress))
			if err != nil {
				return err
			}

			msg := &dv1beta.MsgCloseDeployment{ID: id}

			resp, err := cl.Tx().BroadcastMsgs(ctx, []sdk.Msg{msg})
			if err != nil {
				return err
			}

			return cl.PrintMessage(resp)
		},
	}

	cflags.AddTxFlagsToCmd(cmd)
	cflags.AddDeploymentIDFlags(cmd.Flags())
	return cmd
}

func GetTxDeploymentUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "update [sdl-file]",
		Short:             "update deployment",
		Args:              cobra.ExactArgs(1),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			id, err := cflags.DeploymentIDFromFlags(cmd.Flags(), cflags.WithOwner(cctx.FromAddress))
			if err != nil {
				return err
			}

			sdlManifest, err := sdl.ReadFile(args[0], sdl.WithOwner(cctx.FromAddress.String()))
			if err != nil {
				return err
			}

			hash, err := sdlManifest.Version()
			if err != nil {
				return err
			}

			groups, err := sdlManifest.DeploymentGroups()
			if err != nil {
				return err
			}

			// Query the RPC node to make sure the existing groups are identical.
			// The SDL compiles to deployment/v1beta5 group specs, so the
			// comparison must read the deployment through the v1beta5 query
			// surface (the composite client still exposes v1beta4).
			existingDeployment, err := dv1beta.NewQueryClient(cctx).Deployment(ctx, &dv1beta.QueryDeploymentRequest{
				ID: id,
			})
			if err != nil {
				return err
			}

			// do not send the transaction if the groups have changed
			existingGroups := existingDeployment.GetGroups()

			if len(existingGroups) != len(groups) {
				return errDeploymentUpdateGroupsChanged
			}

			for i, existingGroup := range existingGroups {
				if !existingGroup.GroupSpec.Equal(&groups[i]) {
					return errDeploymentUpdateGroupsChanged
				}
			}

			warnIfGroupVolumesExceeds(cctx, groups)

			msg := &dv1beta.MsgUpdateDeployment{
				ID:   id,
				Hash: hash,
			}

			resp, err := cl.Tx().BroadcastMsgs(ctx, []sdk.Msg{msg})
			if err != nil {
				return err
			}

			return cl.PrintMessage(resp)
		},
	}

	cflags.AddTxFlagsToCmd(cmd)
	cflags.AddDeploymentIDFlags(cmd.Flags())

	return cmd
}

func GetTxDeploymentResizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resize [sdl-file]",
		Short: "ask the provider to resize an active lease in place",
		Long: `Compare the SDL with the deployment on chain and ask the provider of the one
group whose resources changed to resize its lease in place. The provider
accepts at a price within the group's new maximum price, or rejects. Once it
accepts, send the updated manifest to the provider.`,
		Args:              cobra.ExactArgs(1),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			id, err := cflags.DeploymentIDFromFlags(cmd.Flags(), cflags.WithOwner(cctx.FromAddress))
			if err != nil {
				return err
			}

			sdlManifest, err := sdl.ReadFile(args[0], sdl.WithOwner(cctx.FromAddress.String()))
			if err != nil {
				return err
			}

			hash, err := sdlManifest.Version()
			if err != nil {
				return err
			}

			groups, err := sdlManifest.DeploymentGroups()
			if err != nil {
				return err
			}

			existing, err := dv1beta.NewQueryClient(cctx).Deployment(ctx, &dv1beta.QueryDeploymentRequest{
				ID: id,
			})
			if err != nil {
				return err
			}

			target, resources, err := resizeTarget(existing.GetGroups(), groups)
			if err != nil {
				return err
			}

			leases, err := mvbeta.NewQueryClient(cctx).Leases(ctx, &mvbeta.QueryLeasesRequest{
				Filters: mv1.LeaseFilters{
					Owner: id.Owner,
					DSeq:  id.DSeq,
					GSeq:  target.ID.GSeq,
					State: mv1.LeaseActive.String(),
				},
			})
			if err != nil {
				return err
			}

			if len(leases.Leases) != 1 {
				return fmt.Errorf("%w: group %d has no active lease", errDeploymentResize, target.ID.GSeq)
			}

			// a volume's size is not part of the manifest, and an unchanged
			// hash needs no update
			var msgHash []byte
			if target.GroupSpec.Volume == nil && !bytes.Equal(hash, existing.Deployment.Hash) {
				msgHash = hash
			}

			msg := mvbeta.NewMsgResizeLease(leases.Leases[0].Lease.ID, resources, msgHash)

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
	cflags.AddDeploymentIDFlags(cmd.Flags())

	return cmd
}

// resizeTarget finds the one group whose resources differ between the
// deployment on chain and the SDL, and checks the change with the same rules
// the chain applies.
func resizeTarget(existing []dv1beta.Group, groups dv1beta.GroupSpecs) (dv1beta.Group, dv1beta.ResourceUnits, error) {
	if len(existing) != len(groups) {
		return dv1beta.Group{}, nil, fmt.Errorf("%w: groups cannot be added or removed", errDeploymentResize)
	}

	var target *dv1beta.Group
	var resources dv1beta.ResourceUnits

	for i := range existing {
		cur := existing[i].GroupSpec
		next := groups[i]

		if cur.Equal(&next) {
			continue
		}

		if next.Name != cur.Name || !next.Requirements.Equal(&cur.Requirements) || !next.Volume.Equal(cur.Volume) {
			return dv1beta.Group{}, nil, fmt.Errorf("%w: group %q: only resources can change", errDeploymentResize, cur.Name)
		}

		if target != nil {
			return dv1beta.Group{}, nil, fmt.Errorf("%w: more than one group changed; resize one group at a time", errDeploymentResize)
		}

		if _, err := cur.Resize(next.Resources); err != nil {
			return dv1beta.Group{}, nil, err
		}

		target = &existing[i]
		resources = next.Resources
	}

	if target == nil {
		return dv1beta.Group{}, nil, fmt.Errorf("%w: no group changed", errDeploymentResize)
	}

	return *target, resources, nil
}

func GetTxDeploymentGroupCmds() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Modify a Deployment's specific Group",
	}

	cmd.AddCommand(
		GetTxDeploymentGroupCloseCmd(),
		GetDeploymentGroupPauseCmd(),
		GetDeploymentGroupStartCmd(),
	)

	return cmd
}

func GetTxDeploymentGroupCloseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "close",
		Short:             "close a Deployment's specific Group",
		Example:           "akash tx deployment group-close --owner=[Account Address] --dseq=[uint64] --gseq=[uint32]",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)

			id, err := cflags.GroupIDFromFlags(cmd.Flags())
			if err != nil {
				return err
			}

			msg := &dv1beta.MsgCloseGroup{
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
	cflags.AddGroupIDFlags(cmd.Flags())
	cflags.MarkReqGroupIDFlags(cmd)

	return cmd
}

func GetDeploymentGroupPauseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "pause",
		Short:             "pause a Deployment's specific Group",
		Example:           "akash tx deployment group pause --owner=[Account Address] --dseq=[uint64] --gseq=[uint32]",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)

			id, err := cflags.GroupIDFromFlags(cmd.Flags())
			if err != nil {
				return err
			}

			msg := &dv1beta.MsgPauseGroup{
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
	cflags.AddGroupIDFlags(cmd.Flags())
	cflags.MarkReqGroupIDFlags(cmd)

	return cmd
}

func GetDeploymentGroupStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "start",
		Short:             "start a Deployment's specific Group",
		Example:           "akash tx deployment group pause --owner=[Account Address] --dseq=[uint64] --gseq=[uint32]",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)

			id, err := cflags.GroupIDFromFlags(cmd.Flags())
			if err != nil {
				return err
			}

			msg := &dv1beta.MsgStartGroup{
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
	cflags.AddGroupIDFlags(cmd.Flags())
	cflags.MarkReqGroupIDFlags(cmd)

	return cmd
}

func warnIfGroupVolumesExceeds(cctx sdkclient.Context, dgroups []dv1beta.GroupSpec) {
	for _, group := range dgroups {
		for _, resources := range group.GetResourceUnits() {
			if len(resources.Storage) > constants.DefaultMaxGroupVolumes {
				_ = cctx.PrintString(fmt.Sprintf("amount of volumes for service exceeds recommended value (%v > %v)\n"+
					"there may no providers on network to bid", len(resources.Storage), constants.DefaultMaxGroupVolumes))
			}
		}
	}
}
