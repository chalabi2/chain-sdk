package cli

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	marketmocks "pkg.akt.dev/go/mocks/node/client/market"
	dv1 "pkg.akt.dev/go/node/deployment/v1"
	dv1beta "pkg.akt.dev/go/node/deployment/v1beta4"
	mv1 "pkg.akt.dev/go/node/market/v1"
	mvbeta "pkg.akt.dev/go/node/market/v1beta5"
	rtypes "pkg.akt.dev/go/node/types/resources/v1beta4"
	"pkg.akt.dev/go/node/types/unit"
)

func resizeGroup(name string, gseq uint32, cpu uint64) dv1beta.Group {
	return dv1beta.Group{
		ID: dv1.GroupID{Owner: "akash1owner", DSeq: 1, GSeq: gseq},
		GroupSpec: dv1beta.GroupSpec{
			Name: name,
			Resources: dv1beta.ResourceUnits{
				{
					Resources: rtypes.Resources{
						ID:     1,
						CPU:    &rtypes.CPU{Units: rtypes.NewResourceValue(cpu)},
						Memory: &rtypes.Memory{Quantity: rtypes.NewResourceValue(unit.Gi)},
						GPU:    &rtypes.GPU{Units: rtypes.NewResourceValue(0)},
						Storage: rtypes.Volumes{
							{Name: "default", Quantity: rtypes.NewResourceValue(unit.Gi)},
						},
					},
					Count: 1,
					Price: sdk.NewDecCoin("uact", sdkmath.NewInt(10)),
				},
			},
		},
	}
}

func TestResizeTarget(t *testing.T) {
	existing := []dv1beta.Group{resizeGroup("web", 1, 500), resizeGroup("db", 2, 1000)}

	specs := func(cpuWeb, cpuDB uint64) dv1beta.GroupSpecs {
		return dv1beta.GroupSpecs{resizeGroup("web", 1, cpuWeb).GroupSpec, resizeGroup("db", 2, cpuDB).GroupSpec}
	}

	target, resources, err := resizeTarget(existing, specs(500, 2000))
	require.NoError(t, err)
	require.Equal(t, uint32(2), target.ID.GSeq)
	require.Equal(t, uint64(2000), resources[0].CPU.Units.Value())

	_, _, err = resizeTarget(existing, specs(500, 1000))
	require.ErrorIs(t, err, errDeploymentResize)
	require.ErrorContains(t, err, "no group changed")

	_, _, err = resizeTarget(existing, specs(1000, 2000))
	require.ErrorContains(t, err, "more than one group changed")

	_, _, err = resizeTarget(existing, specs(500, 2000)[:1])
	require.ErrorContains(t, err, "groups cannot be added or removed")

	renamed := specs(500, 2000)
	renamed[1].Name = "database"
	_, _, err = resizeTarget(existing, renamed)
	require.ErrorContains(t, err, "only resources can change")

	gpu := specs(500, 1000)
	gpu[1].Resources[0].GPU = &rtypes.GPU{Units: rtypes.NewResourceValue(1)}
	_, _, err = resizeTarget(existing, gpu)
	require.ErrorIs(t, err, dv1.ErrInvalidResize)
}

func TestPendingLeaseResizeDigestQueriesTheDeployment(t *testing.T) {
	id := mv1.LeaseID{Owner: "akash1owner", DSeq: 7, GSeq: 2, OSeq: 1, Provider: "akash1provider"}
	digest := []byte("digest")

	qc := &marketmocks.QueryClient{}
	qc.On("LeaseResizes", mock.Anything, &mvbeta.QueryLeaseResizesRequest{Owner: id.Owner, DSeq: id.DSeq}).
		Return(&mvbeta.QueryLeaseResizesResponse{
			LeaseResizes: []mvbeta.LeaseResize{{LeaseID: id, Digest: digest}},
		}, nil).Once()

	got, err := pendingLeaseResizeDigest(context.Background(), qc, id)
	require.NoError(t, err)
	require.Equal(t, digest, got)
	qc.AssertExpectations(t)
}

func TestPendingLeaseResizeDigestNeedsThisLease(t *testing.T) {
	id := mv1.LeaseID{Owner: "akash1owner", DSeq: 7, GSeq: 2, OSeq: 1, Provider: "akash1provider"}

	// the deployment's pending resize is for another of its groups
	other := id
	other.GSeq = 1

	qc := &marketmocks.QueryClient{}
	qc.On("LeaseResizes", mock.Anything, mock.Anything).
		Return(&mvbeta.QueryLeaseResizesResponse{
			LeaseResizes: []mvbeta.LeaseResize{{LeaseID: other, Digest: []byte("digest")}},
		}, nil).Once()

	_, err := pendingLeaseResizeDigest(context.Background(), qc, id)
	require.ErrorIs(t, err, mv1.ErrLeaseResizeNotFound)
}
