package v2beta1_test

import (
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	dvbeta "pkg.akt.dev/go/node/deployment/v1beta5"
	v1 "pkg.akt.dev/go/node/market/v1"
	"pkg.akt.dev/go/node/market/v2beta1"
	rtypes "pkg.akt.dev/go/node/types/resources/v1beta4"
	"pkg.akt.dev/go/testutil"
)

func resizeResources() dvbeta.ResourceUnits {
	return dvbeta.ResourceUnits{
		{
			Resources: rtypes.Resources{
				ID:     1,
				CPU:    &rtypes.CPU{Units: rtypes.NewResourceValue(1000)},
				Memory: &rtypes.Memory{Quantity: rtypes.NewResourceValue(1 << 30)},
				GPU:    &rtypes.GPU{Units: rtypes.NewResourceValue(0)},
			},
			Count: 1,
			Price: sdk.NewDecCoin("uact", sdkmath.NewInt(1)),
		},
	}
}

func TestMsgResizeLeaseValidateBasic(t *testing.T) {
	id := testutil.LeaseID(t)

	require.NoError(t, v2beta1.NewMsgResizeLease(id, resizeResources(), make([]byte, 32)).ValidateBasic())
	require.NoError(t, v2beta1.NewMsgResizeLease(id, resizeResources(), nil).ValidateBasic(), "volume groups carry no hash")

	err := v2beta1.NewMsgResizeLease(id, nil, nil).ValidateBasic()
	require.ErrorIs(t, err, v1.ErrInvalidLeaseResize)

	err = v2beta1.NewMsgResizeLease(id, resizeResources(), make([]byte, 31)).ValidateBasic()
	require.ErrorIs(t, err, v1.ErrInvalidLeaseResize)

	bad := id
	bad.Owner = "akash1invalid"
	require.Error(t, v2beta1.NewMsgResizeLease(bad, resizeResources(), nil).ValidateBasic())

	require.Equal(t, []sdk.AccAddress{sdk.MustAccAddressFromBech32(id.Owner)},
		v2beta1.NewMsgResizeLease(id, resizeResources(), nil).GetSigners())
}

func TestMsgAcceptLeaseResizeValidateBasic(t *testing.T) {
	id := testutil.LeaseID(t)

	msg := v2beta1.NewMsgAcceptLeaseResize(id, sdk.NewDecCoin("uact", sdkmath.NewInt(7)))
	require.NoError(t, msg.ValidateBasic())
	require.Equal(t, []sdk.AccAddress{sdk.MustAccAddressFromBech32(id.Provider)}, msg.GetSigners())

	err := v2beta1.NewMsgAcceptLeaseResize(id, sdk.NewDecCoin("uact", sdkmath.ZeroInt())).ValidateBasic()
	require.ErrorIs(t, err, v1.ErrInvalidLeaseResize)

	err = v2beta1.NewMsgAcceptLeaseResize(id, sdk.DecCoin{Denom: "", Amount: sdkmath.LegacyNewDec(1)}).ValidateBasic()
	require.ErrorIs(t, err, v1.ErrInvalidLeaseResize)
}

func TestMsgRejectLeaseResizeValidateBasic(t *testing.T) {
	id := testutil.LeaseID(t)

	for _, reason := range []v1.LeaseResizeCancelReason{
		v1.LeaseResizeRejectCapacity,
		v1.LeaseResizeRejectPrice,
		v1.LeaseResizeRejectUnsupported,
		v1.LeaseResizeRejectOther,
	} {
		msg := v2beta1.NewMsgRejectLeaseResize(id, reason)
		require.NoError(t, msg.ValidateBasic(), reason.String())
		require.Equal(t, []sdk.AccAddress{sdk.MustAccAddressFromBech32(id.Provider)}, msg.GetSigners())
	}

	for _, reason := range []v1.LeaseResizeCancelReason{
		v1.LeaseResizeCancelReasonInvalid,
		v1.LeaseResizeCancelReasonOwner,
		v1.LeaseResizeCancelReasonLeaseClosed,
		v1.LeaseResizeCancelReason(150),
	} {
		err := v2beta1.NewMsgRejectLeaseResize(id, reason).ValidateBasic()
		require.ErrorIs(t, err, v1.ErrInvalidLeaseResize, reason.String())
	}
}

func TestMsgCancelLeaseResizeValidateBasic(t *testing.T) {
	id := testutil.LeaseID(t)

	msg := v2beta1.NewMsgCancelLeaseResize(id)
	require.NoError(t, msg.ValidateBasic())
	require.Equal(t, []sdk.AccAddress{sdk.MustAccAddressFromBech32(id.Owner)}, msg.GetSigners())

	bad := id
	bad.Provider = ""
	require.Error(t, v2beta1.NewMsgCancelLeaseResize(bad).ValidateBasic())
}

func TestLeaseResizeParams(t *testing.T) {
	params := v2beta1.DefaultParams()
	require.NoError(t, params.Validate())
	require.False(t, params.LeaseResizeEnabled)
	require.Equal(t, v2beta1.DefaultLeaseResizeTTL, params.LeaseResizeTTL)

	params.LeaseResizeTTL = 0
	require.ErrorIs(t, params.Validate(), v1.ErrInvalidParam)
}

func TestLeaseResizeExpired(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	resize := v2beta1.LeaseResize{ExpiresAt: now.Unix()}

	require.False(t, resize.Expired(now), "the request is still acceptable at its expiry second")
	require.True(t, resize.Expired(now.Add(time.Second)))
}
