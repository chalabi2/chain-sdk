package v1beta5_test

import (
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	v1 "pkg.akt.dev/go/node/deployment/v1"
	"pkg.akt.dev/go/node/deployment/v1beta5"
	attr "pkg.akt.dev/go/node/types/attributes/v1"
	deposit "pkg.akt.dev/go/node/types/deposit/v1"
	rtypes "pkg.akt.dev/go/node/types/resources/v1beta4"
	"pkg.akt.dev/go/node/types/unit"
	"pkg.akt.dev/go/testutil"
)

// volumeGroupSpec returns the canonical valid storage-only (volume) group:
// compute legs present-but-zero, exactly one persistent non-ram Storage
// entry, count 1, no attach references.
func volumeGroupSpec() v1beta5.GroupSpec {
	return v1beta5.GroupSpec{
		Name: "data",
		Volume: &v1.VolumePolicy{
			Vid:            "pg-data",
			Reclaim:        v1.VolumeReclaimRetain,
			Retention:      24 * time.Hour,
			MaxAttachments: 1,
			MaxReplicas:    0,
		},
		Resources: v1beta5.ResourceUnits{
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

// computeGroupSpec returns a valid compute group carrying one attach
// reference to the owner's volume group.
func computeGroupSpec(owner string) v1beta5.GroupSpec {
	return v1beta5.GroupSpec{
		Name: "web",
		Resources: v1beta5.ResourceUnits{
			{
				Resources: rtypes.Resources{
					ID:      1,
					CPU:     &rtypes.CPU{Units: rtypes.NewResourceValue(1000)},
					Memory:  &rtypes.Memory{Quantity: rtypes.NewResourceValue(unit.Gi)},
					GPU:     &rtypes.GPU{Units: rtypes.NewResourceValue(0)},
					Storage: rtypes.Volumes{},
				},
				Count: 1,
				Price: sdk.NewDecCoin("uact", sdkmath.NewInt(1)),
				Volumes: []v1.VolumeRef{
					{
						Owner: owner,
						DSeq:  42,
						GSeq:  1,
						Name:  "data",
					},
				},
			},
		},
	}
}

func TestVolumeGroupSpec_Valid(t *testing.T) {
	owner := testutil.AccAddress(t).String()

	for _, tc := range []struct {
		name   string
		mutate func(*v1beta5.GroupSpec)
	}{
		{"canonical", func(*v1beta5.GroupSpec) {}},
		{"reclaim delete", func(g *v1beta5.GroupSpec) {
			g.Volume.Reclaim = v1.VolumeReclaimDelete
		}},
		{"zero retention", func(g *v1beta5.GroupSpec) {
			g.Volume.Retention = 0
		}},
		{"replicas declared", func(g *v1beta5.GroupSpec) {
			g.Volume.MaxReplicas = 2
		}},
		{"adopt set", func(g *v1beta5.GroupSpec) {
			g.Volume.Adopt = &v1.VolumeRef{Owner: owner, DSeq: 7, GSeq: 1, Name: "data"}
		}},
		{"replica_of set", func(g *v1beta5.GroupSpec) {
			g.Volume.ReplicaOf = &v1.VolumeRef{Owner: owner, DSeq: 7, GSeq: 1, Name: "data"}
		}},
		{"minimum size", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Storage[0].Quantity = rtypes.NewResourceValue(5 * unit.Mi)
		}},
	} {
		gspec := volumeGroupSpec()
		tc.mutate(&gspec)
		require.NoError(t, gspec.ValidateBasic(), tc.name)
	}
}

func TestVolumeGroupSpec_Invalid(t *testing.T) {
	owner := testutil.AccAddress(t).String()

	for _, tc := range []struct {
		name   string
		mutate func(*v1beta5.GroupSpec)
	}{
		{"nil CPU", func(g *v1beta5.GroupSpec) {
			g.Resources[0].CPU = nil
		}},
		{"nonzero CPU", func(g *v1beta5.GroupSpec) {
			g.Resources[0].CPU.Units = rtypes.NewResourceValue(100)
		}},
		{"nil memory", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Memory = nil
		}},
		{"nonzero memory", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Memory.Quantity = rtypes.NewResourceValue(unit.Mi)
		}},
		{"nil GPU", func(g *v1beta5.GroupSpec) {
			g.Resources[0].GPU = nil
		}},
		{"nonzero GPU", func(g *v1beta5.GroupSpec) {
			g.Resources[0].GPU.Units = rtypes.NewResourceValue(1)
		}},
		{"endpoints declared", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Endpoints = rtypes.Endpoints{{SequenceNumber: 1}}
		}},
		{"no storage entry", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Storage = rtypes.Volumes{}
		}},
		{"two storage entries", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Storage = append(g.Resources[0].Storage, g.Resources[0].Storage[0])
		}},
		{"unnamed storage entry", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Storage[0].Name = ""
		}},
		{"persistent missing", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Storage[0].Attributes = attr.Attributes{
				{Key: "class", Value: "beta3"},
			}
		}},
		{"persistent false", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Storage[0].Attributes = attr.Attributes{
				{Key: "class", Value: "beta3"},
				{Key: "persistent", Value: "false"},
			}
		}},
		{"class missing", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Storage[0].Attributes = attr.Attributes{
				{Key: "persistent", Value: "true"},
			}
		}},
		{"class ram", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Storage[0].Attributes = attr.Attributes{
				{Key: "class", Value: "ram"},
				{Key: "persistent", Value: "true"},
			}
		}},
		{"size below 5Mi floor", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Storage[0].Quantity = rtypes.NewResourceValue(5*unit.Mi - 1)
		}},
		{"size above network cap", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Storage[0].Quantity = rtypes.NewResourceValue(32*unit.Ti + 1)
		}},
		{"count zero", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Count = 0
		}},
		{"count two", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Count = 2
		}},
		{"volume refs on volume group", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Volumes = []v1.VolumeRef{
				{Owner: owner, DSeq: 42, GSeq: 1, Name: "data"},
			}
		}},
		{"zero resources ID", func(g *v1beta5.GroupSpec) {
			g.Resources[0].ID = 0
		}},
		{"two resource units", func(g *v1beta5.GroupSpec) {
			ru := g.Resources[0].Dup()
			ru.ID = 2
			g.Resources = append(g.Resources, ru)
		}},
		{"empty vid", func(g *v1beta5.GroupSpec) {
			g.Volume.Vid = ""
		}},
		{"invalid vid", func(g *v1beta5.GroupSpec) {
			g.Volume.Vid = "Not-A-DNS-Label"
		}},
		{"invalid reclaim", func(g *v1beta5.GroupSpec) {
			g.Volume.Reclaim = v1.VolumeReclaimInvalid
		}},
		{"negative retention", func(g *v1beta5.GroupSpec) {
			g.Volume.Retention = -time.Hour
		}},
		{"max_attachments zero", func(g *v1beta5.GroupSpec) {
			g.Volume.MaxAttachments = 0
		}},
		{"max_attachments two", func(g *v1beta5.GroupSpec) {
			g.Volume.MaxAttachments = 2
		}},
		{"adopt and replica_of both set", func(g *v1beta5.GroupSpec) {
			g.Volume.Adopt = &v1.VolumeRef{Owner: owner, DSeq: 7, GSeq: 1, Name: "data"}
			g.Volume.ReplicaOf = &v1.VolumeRef{Owner: owner, DSeq: 8, GSeq: 1, Name: "data"}
		}},
		{"adopt bad owner", func(g *v1beta5.GroupSpec) {
			g.Volume.Adopt = &v1.VolumeRef{Owner: "notanaddress", DSeq: 7, GSeq: 1, Name: "data"}
		}},
		{"adopt zero dseq", func(g *v1beta5.GroupSpec) {
			g.Volume.Adopt = &v1.VolumeRef{Owner: owner, DSeq: 0, GSeq: 1, Name: "data"}
		}},
		{"adopt gseq not 1", func(g *v1beta5.GroupSpec) {
			g.Volume.Adopt = &v1.VolumeRef{Owner: owner, DSeq: 7, GSeq: 2, Name: "data"}
		}},
		{"adopt empty name", func(g *v1beta5.GroupSpec) {
			g.Volume.Adopt = &v1.VolumeRef{Owner: owner, DSeq: 7, GSeq: 1, Name: ""}
		}},
		{"replica_of gseq not 1", func(g *v1beta5.GroupSpec) {
			g.Volume.ReplicaOf = &v1.VolumeRef{Owner: owner, DSeq: 7, GSeq: 2, Name: "data"}
		}},
	} {
		gspec := volumeGroupSpec()
		tc.mutate(&gspec)
		require.Error(t, gspec.ValidateBasic(), tc.name)
	}
}

func TestVolumeGroupSpec_ValidateVolumeBounds(t *testing.T) {
	params := v1beta5.DefaultParams()

	t.Run("within bounds", func(t *testing.T) {
		gspec := volumeGroupSpec()
		require.NoError(t, gspec.ValidateVolumeBounds(params))
	})

	t.Run("size above MaxVolumeSize", func(t *testing.T) {
		gspec := volumeGroupSpec()
		gspec.Resources[0].Storage[0].Quantity = rtypes.NewResourceValue(20 * unit.Gi)

		p := params
		p.MaxVolumeSize = 10 * unit.Gi
		require.Error(t, gspec.ValidateVolumeBounds(p))
	})

	t.Run("retention above MaxVolumeRetention", func(t *testing.T) {
		gspec := volumeGroupSpec()
		gspec.Volume.Retention = params.MaxVolumeRetention + time.Hour
		require.Error(t, gspec.ValidateVolumeBounds(params))
	})

	t.Run("max_replicas above MaxVolumeReplicas", func(t *testing.T) {
		gspec := volumeGroupSpec()
		gspec.Volume.MaxReplicas = params.MaxVolumeReplicas + 1
		require.Error(t, gspec.ValidateVolumeBounds(params))
	})

	t.Run("compute group always passes", func(t *testing.T) {
		owner := testutil.AccAddress(t).String()
		gspec := computeGroupSpec(owner)

		p := params
		p.MaxVolumeSize = 1
		require.NoError(t, gspec.ValidateVolumeBounds(p))
	})
}

func TestComputeGroupSpec_VolumeRefs(t *testing.T) {
	owner := testutil.AccAddress(t).String()

	t.Run("valid attach ref", func(t *testing.T) {
		gspec := computeGroupSpec(owner)
		require.NoError(t, gspec.ValidateBasic())
	})

	t.Run("no refs stays valid", func(t *testing.T) {
		gspec := computeGroupSpec(owner)
		gspec.Resources[0].Volumes = nil
		require.NoError(t, gspec.ValidateBasic())
	})

	t.Run("existing compute rules still apply", func(t *testing.T) {
		gspec := computeGroupSpec(owner)
		gspec.Resources[0].CPU.Units = rtypes.NewResourceValue(0)
		require.Error(t, gspec.ValidateBasic())
	})

	for _, tc := range []struct {
		name   string
		mutate func(*v1beta5.GroupSpec)
	}{
		{"bad owner", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Volumes[0].Owner = "notanaddress"
		}},
		{"zero dseq", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Volumes[0].DSeq = 0
		}},
		{"zero gseq", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Volumes[0].GSeq = 0
		}},
		{"gseq not 1", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Volumes[0].GSeq = 2
		}},
		{"empty name", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Volumes[0].Name = ""
		}},
		{"duplicate ref", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Volumes = append(g.Resources[0].Volumes, g.Resources[0].Volumes[0])
		}},
		{"storage entry shadows ref", func(g *v1beta5.GroupSpec) {
			g.Resources[0].Storage = rtypes.Volumes{
				{
					Name:     g.Resources[0].Volumes[0].Name,
					Quantity: rtypes.NewResourceValue(unit.Gi),
					Attributes: attr.Attributes{
						{Key: "class", Value: "beta3"},
						{Key: "persistent", Value: "true"},
					},
				},
			}
		}},
	} {
		gspec := computeGroupSpec(owner)
		tc.mutate(&gspec)
		require.Error(t, gspec.ValidateBasic(), tc.name)
	}
}

func TestValidateDeploymentGroups_VolumeSingleGroup(t *testing.T) {
	owner := testutil.AccAddress(t).String()

	t.Run("volume group alone", func(t *testing.T) {
		require.NoError(t, v1beta5.ValidateDeploymentGroups([]v1beta5.GroupSpec{volumeGroupSpec()}))
	})

	t.Run("volume then compute", func(t *testing.T) {
		err := v1beta5.ValidateDeploymentGroups([]v1beta5.GroupSpec{volumeGroupSpec(), computeGroupSpec(owner)})
		require.ErrorIs(t, err, v1.ErrInvalidGroups)
	})

	t.Run("compute then volume", func(t *testing.T) {
		err := v1beta5.ValidateDeploymentGroups([]v1beta5.GroupSpec{computeGroupSpec(owner), volumeGroupSpec()})
		require.ErrorIs(t, err, v1.ErrInvalidGroups)
	})

	t.Run("two compute groups", func(t *testing.T) {
		second := computeGroupSpec(owner)
		second.Name = "worker"
		require.NoError(t, v1beta5.ValidateDeploymentGroups([]v1beta5.GroupSpec{computeGroupSpec(owner), second}))
	})
}

func newVolumeCreateMsg(t *testing.T, owner string, groups []v1beta5.GroupSpec) *v1beta5.MsgCreateDeployment {
	t.Helper()

	return v1beta5.NewMsgCreateDeployment(
		v1.DeploymentID{Owner: owner, DSeq: 1},
		groups,
		testutil.DefaultDeploymentHash[:],
		deposit.Deposit{
			Amount:  sdk.NewCoin("uact", sdkmath.NewInt(500000)),
			Sources: deposit.Sources{deposit.SourceBalance},
		},
	)
}

func TestMsgCreateDeployment_VolumeGroups(t *testing.T) {
	owner := testutil.AccAddress(t).String()
	other := testutil.AccAddress(t).String()

	t.Run("volume group alone", func(t *testing.T) {
		msg := newVolumeCreateMsg(t, owner, []v1beta5.GroupSpec{volumeGroupSpec()})
		require.NoError(t, msg.ValidateBasic())
	})

	t.Run("volume group with compute sibling", func(t *testing.T) {
		msg := newVolumeCreateMsg(t, owner, []v1beta5.GroupSpec{volumeGroupSpec(), computeGroupSpec(owner)})
		err := msg.ValidateBasic()
		require.ErrorIs(t, err, v1.ErrInvalidGroups)
	})

	t.Run("adopt owner matches", func(t *testing.T) {
		gspec := volumeGroupSpec()
		gspec.Volume.Adopt = &v1.VolumeRef{Owner: owner, DSeq: 7, GSeq: 1, Name: "data"}

		msg := newVolumeCreateMsg(t, owner, []v1beta5.GroupSpec{gspec})
		require.NoError(t, msg.ValidateBasic())
	})

	t.Run("adopt owner mismatch", func(t *testing.T) {
		gspec := volumeGroupSpec()
		gspec.Volume.Adopt = &v1.VolumeRef{Owner: other, DSeq: 7, GSeq: 1, Name: "data"}

		msg := newVolumeCreateMsg(t, owner, []v1beta5.GroupSpec{gspec})
		err := msg.ValidateBasic()
		require.ErrorIs(t, err, v1.ErrInvalidGroups)
	})

	t.Run("replica_of owner mismatch", func(t *testing.T) {
		gspec := volumeGroupSpec()
		gspec.Volume.ReplicaOf = &v1.VolumeRef{Owner: other, DSeq: 7, GSeq: 1, Name: "data"}

		msg := newVolumeCreateMsg(t, owner, []v1beta5.GroupSpec{gspec})
		err := msg.ValidateBasic()
		require.ErrorIs(t, err, v1.ErrInvalidGroups)
	})
}
