package cli

import (
	"fmt"

	sdkclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/spf13/cobra"

	cflags "pkg.akt.dev/go/cli/flags"
	dv1beta "pkg.akt.dev/go/node/deployment/v1beta5"
	mv1 "pkg.akt.dev/go/node/market/v1"
	mv2beta "pkg.akt.dev/go/node/market/v2beta1"
)

// GetQueryVolumeCmds returns the query commands for volumes — storage-only
// deployments of the decoupled storage market (AEP-87). Volumes ride the
// existing deployment and market query surfaces, filtered on
// GroupSpec.Volume != nil.
//
// TODO(aep-87): resolve volumes by vid through the x/deployment volumesByVid
// (0x13) index query once it lands with the node work; until then volumes are
// addressed by deployment ID and list filters client-side.
func GetQueryVolumeCmds() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "volume",
		Short:                      "Volume (decoupled storage) query commands",
		SuggestionsMinimumDistance: 2,
		RunE:                       sdkclient.ValidateCmd,
	}

	cmd.AddCommand(
		GetQueryVolumesCmd(),
		GetQueryVolumeStatusCmd(),
	)

	return cmd
}

func GetQueryVolumesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list",
		Short:             "Query for all volume deployments",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: QueryPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustLightClientFromContext(ctx)
			cctx := cl.ClientContext()

			dfilters, err := cflags.DepFiltersFromFlags(cmd.Flags())
			if err != nil {
				return err
			}

			pageReq, err := ReadPageRequest(cmd.Flags())
			if err != nil {
				return err
			}

			params := &dv1beta.QueryDeploymentsRequest{
				Filters: dv1beta.DeploymentFilters{
					Owner: dfilters.Owner,
					DSeq:  dfilters.DSeq,
					State: dfilters.State,
				},
				Pagination: pageReq,
			}

			// volume groups exist only on the v1beta5 surface; the composite
			// client still exposes v1beta4, so query it directly
			res, err := dv1beta.NewQueryClient(cctx).Deployments(ctx, params)
			if err != nil {
				return err
			}

			// TODO(aep-87): replace the client-side filter with the
			// volumesByVid index query once the node work lands.
			volumes := make(dv1beta.DeploymentResponses, 0, len(res.Deployments))
			for _, dep := range res.Deployments {
				if hasVolumeGroup(dep.Groups) {
					volumes = append(volumes, dep)
				}
			}

			res.Deployments = volumes

			return cctx.PrintProto(res)
		},
	}

	cflags.AddQueryFlagsToCmd(cmd)
	cflags.AddPaginationFlagsToCmd(cmd, "volumes")
	cflags.AddDeploymentFilterFlags(cmd.Flags())

	return cmd
}

func GetQueryVolumeStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "status",
		Short:             "Query the status of a volume deployment: its group, escrow account and leases",
		Args:              cobra.ExactArgs(0),
		PersistentPreRunE: QueryPersistentPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cl := MustLightClientFromContext(ctx)
			cctx := cl.ClientContext()

			id, err := cflags.DeploymentIDFromFlags(cmd.Flags())
			if err != nil {
				return err
			}

			dres, err := dv1beta.NewQueryClient(cctx).Deployment(ctx, &dv1beta.QueryDeploymentRequest{ID: id})
			if err != nil {
				return err
			}

			if !hasVolumeGroup(dres.Groups) {
				return fmt.Errorf("deployment %s has no volume group; use \"query deployment get\" for compute deployments", id)
			}

			lres, err := mv2beta.NewQueryClient(cctx).Leases(ctx, &mv2beta.QueryLeasesRequest{
				Filters: mv1.LeaseFilters{
					Owner: id.Owner,
					DSeq:  id.DSeq,
				},
			})
			if err != nil {
				return err
			}

			// TODO(aep-87): surface the escrow runway (EventAccountRunway)
			// and the post-close retention deadline once the node work lands.
			out := struct {
				Deployment dv1beta.QueryDeploymentResponse `json:"deployment"`
				Leases     []mv2beta.QueryLeaseResponse    `json:"leases"`
			}{
				Deployment: *dres,
				Leases:     lres.Leases,
			}

			return PrintJSON(cctx, out)
		},
	}

	cflags.AddQueryFlagsToCmd(cmd)
	cflags.AddDeploymentIDFlags(cmd.Flags())
	cflags.MarkReqDeploymentIDFlags(cmd)

	return cmd
}

// hasVolumeGroup reports whether any of the deployment's groups is a
// storage-only volume group. The VolumePolicy presence is the discriminator.
func hasVolumeGroup(groups dv1beta.Groups) bool {
	for _, group := range groups {
		if group.GroupSpec.Volume != nil {
			return true
		}
	}

	return false
}
