package testutil

import (
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	dtypes "pkg.akt.dev/go/node/deployment/v1"
	dvbeta "pkg.akt.dev/go/node/deployment/v1beta4"
	attr "pkg.akt.dev/go/node/types/attributes/v1"
	rtypes "pkg.akt.dev/go/node/types/resources/v1beta4"
	"pkg.akt.dev/go/node/types/unit"
)

// VolumeGroupSpec generates a valid storage-only (volume) group spec for the
// given vid: zero compute, exactly one persistent non-ram storage entry,
// count 1, priced in uact.
func VolumeGroupSpec(t testing.TB, vid string) dvbeta.GroupSpec {
	t.Helper()
	return dvbeta.GroupSpec{
		Name: Name(t, "vgroup"),
		Volume: &dtypes.VolumePolicy{
			Vid:            vid,
			Reclaim:        dtypes.VolumeReclaimRetain,
			Retention:      24 * time.Hour,
			MaxAttachments: 1,
		},
		Resources: dvbeta.ResourceUnits{
			{
				Resources: rtypes.Resources{
					ID:     1,
					CPU:    &rtypes.CPU{Units: rtypes.NewResourceValue(0)},
					Memory: &rtypes.Memory{Quantity: rtypes.NewResourceValue(0)},
					GPU:    &rtypes.GPU{Units: rtypes.NewResourceValue(0)},
					Storage: rtypes.Volumes{
						{
							Name:     "data",
							Quantity: rtypes.NewResourceValue(10 * unit.Gi),
							Attributes: attr.Attributes{
								{Key: "class", Value: "beta3"},
								{Key: "persistent", Value: "true"},
							},
						},
					},
				},
				Count: 1,
				Price: sdk.NewDecCoin("uact", sdkmath.NewInt(1)),
			},
		},
	}
}

// VolumeDeploymentGroup generates a volume group in state `GroupOpen` carrying
// the given vid. A volume group is always its deployment's only group.
func VolumeDeploymentGroup(t testing.TB, did dtypes.DeploymentID, vid string) dvbeta.Group {
	t.Helper()
	return dvbeta.Group{
		ID:        dtypes.MakeGroupID(did, 1),
		State:     dvbeta.GroupOpen,
		GroupSpec: VolumeGroupSpec(t, vid),
	}
}
