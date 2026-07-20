package cli

import (
	"errors"
	"fmt"
	"io"

	sdkclient "github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	cflags "pkg.akt.dev/go/cli/flags"
	dv1beta "pkg.akt.dev/go/node/deployment/v1beta5"
	"pkg.akt.dev/go/sdl"
)

var (
	errVolumeCreate            = errors.New("volume create failed")
	errVolumeCreateNoVolumes   = fmt.Errorf("%w: SDL declares no volumes; use \"tx deployment create\" for compute deployments", errVolumeCreate)
	errVolumeCreateMixedSDL    = fmt.Errorf("%w: SDL mixes volumes and services; create the volume deployment first, then the compute deployment", errVolumeCreate)
	errVolumeCreateSingleGroup = fmt.Errorf("%w: a volume deployment holds exactly one volume; declare one volume per SDL", errVolumeCreate)
)

// GetTxVolumeCmds returns the transaction commands for volumes — storage-only
// deployments of the decoupled storage market (AEP-87). A volume is an
// ordinary deployment whose single group carries a VolumePolicy, so these
// commands ride the existing deployment messages; no new Msg types exist.
func GetTxVolumeCmds() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "volume",
		Short:                      "Volume (decoupled storage) transaction subcommands",
		SuggestionsMinimumDistance: 2,
		RunE:                       sdkclient.ValidateCmd,
	}

	cmd.AddCommand(
		GetTxVolumeCreateCmd(),
		GetTxVolumeCloseCmd(),
	)

	return cmd
}

func GetTxVolumeCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [sdl-file]",
		Short: "Create a volume deployment from an SDL v2.2 volumes stanza",
		Long: `Create a volume deployment from an SDL v2.2 file declaring a single volume.

The volume compiles to a storage-only deployment group; the deployment hash is
the canonical empty manifest (the on-chain group name with no services), so no
certificate or manifest submission is required. Accept a bid with
"tx market lease create" as with any deployment.`,
		Args:              cobra.ExactArgs(1),
		PersistentPreRunE: TxPersistentPreRunE,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cl := MustClientFromContext(ctx)
			cctx := cl.ClientContext()

			// volume references compile to owner-keyed VolumeRefs; the owner
			// is always the tx signer
			sdlManifest, err := sdl.ReadFile(args[0], sdl.WithOwner(cctx.FromAddress.String()))
			if err != nil {
				return err
			}

			volumes, err := volumeGroupsFromSDL(sdlManifest)
			if err != nil {
				return err
			}

			warnVolumePolicy(cmd.ErrOrStderr(), volumes)

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

			// the canonical empty-manifest hash: a single group with the
			// on-chain group name and an empty service list
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
				Groups:      volumes,
				Deposit:     dep,
				Reclamation: reclamation,
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
	cflags.AddDeploymentIDFlags(cmd.Flags())
	cflags.AddDepositFlags(cmd.Flags())

	return cmd
}

func GetTxVolumeCloseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close",
		Short: "Close a volume deployment",
		Long: `Close a volume deployment.

Attached compute leases must be closed first. The close routes through
reclamation: the volume lease enters the reclaiming state for at least the
network's minimum volume reclamation window before the close completes, and
the provider retains the data for the volume's retention window afterwards.`,
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

// volumeGroupsFromSDL extracts the volume group specs a volume deployment is
// created from. It enforces the client-side shape of the create command: the
// SDL declares volumes only (mixed SDLs compile to two deployments; create
// the volume first), and exactly one of them — a deployment containing a
// volume group contains only that group (the single-group invariant; gseq is
// always 1).
func volumeGroupsFromSDL(sdlManifest sdl.SDL) (dv1beta.GroupSpecs, error) {
	groups, err := sdlManifest.DeploymentGroups()
	if err != nil {
		return nil, err
	}

	volumes, err := sdlManifest.Volumes()
	if err != nil {
		return nil, err
	}

	switch {
	case len(volumes) == 0:
		return nil, errVolumeCreateNoVolumes
	case len(groups) != len(volumes):
		return nil, errVolumeCreateMixedSDL
	case len(volumes) > 1:
		return nil, errVolumeCreateSingleGroup
	}

	return volumes, nil
}

// warnVolumePolicy surfaces policy choices that leave the volume without
// redundancy or an adoption window; both are valid but easy to pick by
// accident. Warnings go to stderr — stdout carries the tx JSON response
// and must stay machine-parseable.
func warnVolumePolicy(out io.Writer, volumes dv1beta.GroupSpecs) {
	for _, group := range volumes {
		vol := group.Volume
		if vol == nil {
			continue
		}

		if vol.MaxReplicas == 0 {
			_, _ = fmt.Fprintf(out, "volume %q declares max-replicas: 0\n"+
				"no provider is obliged to serve replica exports; the volume has no cross-provider redundancy\n", vol.Vid)
		}

		if vol.Retention == 0 {
			_, _ = fmt.Fprintf(out, "volume %q declares no retention window\n"+
				"after the volume closes there is no adoption window; the data is immediately eligible for garbage collection\n", vol.Vid)
		}
	}
}
