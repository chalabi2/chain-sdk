package v2beta4

import (
	"testing"

	"github.com/stretchr/testify/require"

	dv1 "pkg.akt.dev/go/node/deployment/v1"
	dtypes "pkg.akt.dev/go/node/deployment/v1beta5"
	"pkg.akt.dev/go/testutil"
)

func simpleVolumeGSpec(name string) dtypes.GroupSpec {
	return dtypes.GroupSpec{
		Name: name,
		Volume: &dv1.VolumePolicy{
			Vid:            "data",
			Reclaim:        dv1.VolumeReclaimRetain,
			MaxAttachments: 1,
		},
		Resources: dtypes.ResourceUnits{
			{
				Resources: randUnits1,
				Count:     1,
			},
		},
	}
}

// The canonical volume manifest is a single group with the on-chain group
// name and an empty service list. It validates standalone and pairs only
// with an on-chain volume group.
func TestVolumeManifestAgainstVolumeGroup(t *testing.T) {
	m := Manifest{
		Group{Name: nameOfTestGroup},
	}

	require.NoError(t, m.Validate())

	err := m.CheckAgainstGSpecs(dtypes.GroupSpecs{simpleVolumeGSpec(nameOfTestGroup)})
	require.NoError(t, err)
}

func TestVolumeManifestAgainstComputeGroup(t *testing.T) {
	m := Manifest{
		Group{Name: nameOfTestGroup},
	}

	gspec := simpleVolumeGSpec(nameOfTestGroup)
	gspec.Volume = nil

	err := m.CheckAgainstGSpecs(dtypes.GroupSpecs{gspec})
	require.Error(t, err)
	require.Regexp(t, "^.*contains no services.*$", err)
}

func TestServicesAgainstVolumeGroup(t *testing.T) {
	m := simpleManifest(1)
	m[0].Name = nameOfTestGroup

	gspec := simpleVolumeGSpec(nameOfTestGroup)

	err := m.CheckAgainstGSpecs(dtypes.GroupSpecs{gspec})
	require.Error(t, err)
	require.Regexp(t, "^.*volume group must not contain services.*$", err)
}

// The set of manifest StorageParams.volume refs must equal the on-chain
// ResourceUnit.Volumes exactly.
func TestVolumeRefsCrossValidation(t *testing.T) {
	owner := testutil.AccAddress(t).String()

	ref := dv1.VolumeRef{
		Owner: owner,
		DSeq:  1,
		GSeq:  1,
		Name:  "data",
	}

	mkManifest := func(volume string) Manifest {
		m := simpleManifest(1)
		m[0].Name = nameOfTestGroup

		if volume != "" {
			m[0].Services[0].Params = &ServiceParams{
				Storage: []StorageParams{
					{
						Name:   "data",
						Mount:  "/data",
						Volume: volume,
					},
				},
			}
		}

		return m
	}

	mkGSpec := func(refs ...dv1.VolumeRef) dtypes.GroupSpecs {
		m := simpleManifest(1)

		return dtypes.GroupSpecs{
			{
				Name: nameOfTestGroup,
				Resources: dtypes.ResourceUnits{
					{
						Resources: m[0].Services[0].Resources,
						Count:     m[0].Services[0].Count,
						Volumes:   refs,
					},
				},
			},
		}
	}

	// matching refs on both sides
	err := mkManifest(ref.String()).CheckAgainstGSpecs(mkGSpec(ref))
	require.NoError(t, err)

	// manifest carries a ref the group spec does not
	err = mkManifest(ref.String()).CheckAgainstGSpecs(mkGSpec())
	require.Error(t, err)
	require.Regexp(t, "^.*volume refs count mismatch.*$", err)

	// group spec carries a ref the manifest does not
	err = mkManifest("").CheckAgainstGSpecs(mkGSpec(ref))
	require.Error(t, err)
	require.Regexp(t, "^.*volume refs count mismatch.*$", err)

	// same count, different ref
	other := ref
	other.Name = "other"
	err = mkManifest(other.String()).CheckAgainstGSpecs(mkGSpec(ref))
	require.Error(t, err)
	require.Regexp(t, "^.*volume ref mismatch.*$", err)
}
