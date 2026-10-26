package testutil_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.akt.dev/go/testutil"
)

func TestVolumeGroupSpecIsValid(t *testing.T) {
	spec := testutil.VolumeGroupSpec(t, "pgdata")
	require.NoError(t, spec.ValidateBasic())

	group := testutil.VolumeDeploymentGroup(t, testutil.DeploymentID(t), "pgdata")
	require.EqualValues(t, 1, group.ID.GSeq)
	require.NoError(t, group.GroupSpec.ValidateBasic())
}
