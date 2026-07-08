package v1_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	v1 "pkg.akt.dev/go/node/deployment/v1"
	"pkg.akt.dev/go/testutil"
)

func TestVolumeRefStringRoundTrip(t *testing.T) {
	ref := v1.VolumeRef{
		Owner: testutil.AccAddress(t).String(),
		DSeq:  42,
		GSeq:  1,
		Name:  "data",
	}

	require.NoError(t, ref.Validate())

	parsed, err := v1.ParseVolumeRef(ref.String())
	require.NoError(t, err)
	require.Equal(t, ref, parsed)
	require.True(t, ref.Equals(parsed))
}

func TestVolumeRefParseInvalid(t *testing.T) {
	owner := testutil.AccAddress(t).String()

	for _, val := range []string{
		"",
		"a/b",
		owner + "/1/1",
		owner + "/1/1/data/extra",
		"notanaddress/1/1/data",
		owner + "/x/1/data",
		owner + "/1/x/data",
		owner + "/1/1/",
	} {
		_, err := v1.ParseVolumeRef(val)
		require.Error(t, err, val)
	}
}

func TestVolumeRefValidate(t *testing.T) {
	owner := testutil.AccAddress(t).String()

	for _, tc := range []struct {
		name string
		ref  v1.VolumeRef
	}{
		{"bad owner", v1.VolumeRef{Owner: "invalid", DSeq: 1, GSeq: 1, Name: "data"}},
		{"zero dseq", v1.VolumeRef{Owner: owner, DSeq: 0, GSeq: 1, Name: "data"}},
		{"zero gseq", v1.VolumeRef{Owner: owner, DSeq: 1, GSeq: 0, Name: "data"}},
		{"empty name", v1.VolumeRef{Owner: owner, DSeq: 1, GSeq: 1, Name: ""}},
	} {
		require.Error(t, tc.ref.Validate(), tc.name)
	}
}

func TestValidateVID(t *testing.T) {
	for _, vid := range []string{"a", "data", "pg-data-01", "0a"} {
		require.NoError(t, v1.ValidateVID(vid), vid)
	}

	for _, vid := range []string{"", "-data", "data-", "Data", "d_ata", "a.b",
		"aaaaaaaaaabbbbbbbbbbccccccccccddddddddddeeeeeeeeeeffffffffffgggg"} {
		require.Error(t, v1.ValidateVID(vid), vid)
	}
}
