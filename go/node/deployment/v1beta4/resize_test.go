package v1beta4_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	v1 "pkg.akt.dev/go/node/deployment/v1"
	"pkg.akt.dev/go/node/deployment/v1beta4"
	attr "pkg.akt.dev/go/node/types/attributes/v1"
	rtypes "pkg.akt.dev/go/node/types/resources/v1beta4"
	"pkg.akt.dev/go/node/types/unit"
	"pkg.akt.dev/go/testutil"
)

// resizeComputeSpec returns a compute group with a GPU unit and a second
// unit carrying ephemeral and persistent storage plus an endpoint.
func resizeComputeSpec() v1beta4.GroupSpec {
	return v1beta4.GroupSpec{
		Name: "app",
		Resources: v1beta4.ResourceUnits{
			{
				Resources: rtypes.Resources{
					ID:     1,
					CPU:    &rtypes.CPU{Units: rtypes.NewResourceValue(1000)},
					Memory: &rtypes.Memory{Quantity: rtypes.NewResourceValue(2 * unit.Gi)},
					GPU: &rtypes.GPU{
						Units: rtypes.NewResourceValue(1),
						Attributes: attr.Attributes{
							{Key: "vendor/nvidia/model/a100", Value: "true"},
						},
					},
					Storage: rtypes.Volumes{
						{Name: "default", Quantity: rtypes.NewResourceValue(5 * unit.Gi)},
					},
				},
				Count: 1,
				Price: sdk.NewDecCoin("uact", sdkmath.NewInt(100)),
			},
			{
				Resources: rtypes.Resources{
					ID:     2,
					CPU:    &rtypes.CPU{Units: rtypes.NewResourceValue(500)},
					Memory: &rtypes.Memory{Quantity: rtypes.NewResourceValue(unit.Gi)},
					GPU:    &rtypes.GPU{Units: rtypes.NewResourceValue(0)},
					Storage: rtypes.Volumes{
						{Name: "default", Quantity: rtypes.NewResourceValue(unit.Gi)},
						{
							Name:     "data",
							Quantity: rtypes.NewResourceValue(10 * unit.Gi),
							Attributes: attr.Attributes{
								{Key: "class", Value: "beta3"},
								{Key: "persistent", Value: "true"},
							},
						},
					},
					Endpoints: rtypes.Endpoints{
						{Kind: rtypes.Endpoint_SHARED_HTTP, SequenceNumber: 0},
					},
				},
				Count: 2,
				Price: sdk.NewDecCoin("uact", sdkmath.NewInt(10)),
			},
		},
	}
}

func TestGroupSpecResize(t *testing.T) {
	tests := []struct {
		name   string
		spec   func() v1beta4.GroupSpec
		mutate func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits
		err    string
	}{
		{
			name: "cpu and memory up",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[0].CPU.Units = rtypes.NewResourceValue(4000)
				rus[0].Memory.Quantity = rtypes.NewResourceValue(8 * unit.Gi)
				return rus
			},
		},
		{
			name: "cpu down, count up, price up",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[1].CPU.Units = rtypes.NewResourceValue(250)
				rus[1].Count = 4
				rus[1].Price = sdk.NewDecCoin("uact", sdkmath.NewInt(20))
				return rus
			},
		},
		{
			name: "gpu count changes with the same model",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[0].GPU.Units = rtypes.NewResourceValue(2)
				return rus
			},
		},
		{
			name: "ephemeral storage changes",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[1].Storage[0].Quantity = rtypes.NewResourceValue(4 * unit.Gi)
				return rus
			},
		},
		{
			name: "units given in a different order",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[0].CPU.Units = rtypes.NewResourceValue(2000)
				return v1beta4.ResourceUnits{rus[1], rus[0]}
			},
		},
		{
			name: "volume grows",
			spec: volumeGroupSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[0].Storage[0].Quantity = rtypes.NewResourceValue(50 * unit.Gi)
				return rus
			},
		},
		{
			name:   "nothing changes",
			spec:   resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits { return rus },
			err:    "resources unchanged",
		},
		{
			name: "unit dropped",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				return rus[:1]
			},
			err: "resource unit count changes",
		},
		{
			name: "unit id replaced",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[1].ID = 3
				return rus
			},
			err: "resource unit 2 missing",
		},
		{
			name: "duplicate unit id",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[1].ID = 1
				return rus
			},
			err: "duplicate resource unit 1",
		},
		{
			name: "gpu added",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[1].GPU.Units = rtypes.NewResourceValue(1)
				return rus
			},
			err: "gpus cannot be added or removed",
		},
		{
			name: "gpu removed",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[0].GPU = &rtypes.GPU{Units: rtypes.NewResourceValue(0)}
				return rus
			},
			err: "gpus cannot be added or removed",
		},
		{
			name: "gpu model changes",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[0].GPU.Attributes = attr.Attributes{{Key: "vendor/nvidia/model/h100", Value: "true"}}
				return rus
			},
			err: "gpu attributes change",
		},
		{
			name: "persistent storage resized on a compute group",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[1].Storage[1].Quantity = rtypes.NewResourceValue(20 * unit.Gi)
				return rus
			},
			err: "persistent storage \"data\" keeps its size",
		},
		{
			name: "storage class changes",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[1].Storage[1].Attributes = attr.Attributes{
					{Key: "class", Value: "beta2"},
					{Key: "persistent", Value: "true"},
				}
				return rus
			},
			err: "storage \"data\" attributes change",
		},
		{
			name: "storage renamed",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[0].Storage[0].Name = "scratch"
				return rus
			},
			err: "storage \"default\" renamed",
		},
		{
			name: "endpoint added",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[0].Endpoints = rtypes.Endpoints{{Kind: rtypes.Endpoint_RANDOM_PORT}}
				return rus
			},
			err: "endpoints change",
		},
		{
			name: "price denom changes",
			spec: resizeComputeSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[0].Price = sdk.NewDecCoin("uakt", sdkmath.NewInt(100))
				return rus
			},
			err: "price denom changes",
		},
		{
			name: "volume shrinks",
			spec: volumeGroupSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[0].Storage[0].Quantity = rtypes.NewResourceValue(5 * unit.Gi)
				return rus
			},
			err: "volume \"data\" cannot shrink",
		},
		{
			name: "volume group gains compute",
			spec: volumeGroupSpec,
			mutate: func(rus v1beta4.ResourceUnits) v1beta4.ResourceUnits {
				rus[0].CPU.Units = rtypes.NewResourceValue(100)
				return rus
			},
			err: "invalid volume group CPU",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := test.spec()
			resources := test.mutate(spec.Resources.Dup())

			next, err := spec.Resize(resources)
			if test.err != "" {
				require.ErrorIs(t, err, v1.ErrInvalidResize)
				require.ErrorContains(t, err, test.err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, spec.Name, next.Name)
			require.Equal(t, spec.Requirements, next.Requirements)
			require.Equal(t, spec.Volume, next.Volume)

			for _, want := range resources {
				var got *v1beta4.ResourceUnit
				for i := range next.Resources {
					if next.Resources[i].ID == want.ID {
						got = &next.Resources[i]
					}
				}
				require.NotNil(t, got)
				require.True(t, want.Equal(got))
			}
		})
	}
}

func TestGroupSpecResizeKeepsVolumeMountCount(t *testing.T) {
	spec := computeGroupSpec(testutil.AccAddress(t).String())

	resources := spec.Resources.Dup()
	resources[0].Memory.Quantity = rtypes.NewResourceValue(4 * unit.Gi)

	next, err := spec.Resize(resources)
	require.NoError(t, err)
	require.Equal(t, spec.Resources[0].Volumes, next.Resources[0].Volumes)

	resources[0].Count = 2

	_, err = spec.Resize(resources)
	require.ErrorIs(t, err, v1.ErrInvalidResize)
	require.ErrorContains(t, err, "count of a unit that mounts volumes changes")
}
