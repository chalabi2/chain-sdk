package v2beta1

import (
	"fmt"
	"reflect"

	cerrors "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"

	dv1 "pkg.akt.dev/go/node/deployment/v1"
	dvbeta "pkg.akt.dev/go/node/deployment/v1beta5"
	v1 "pkg.akt.dev/go/node/market/v1"
	deposit "pkg.akt.dev/go/node/types/deposit/v1"
)

var (
	msgTypeCreateBid         = ""
	msgTypeCloseBid          = ""
	msgTypeCreateLease       = ""
	msgTypeCloseLease        = ""
	msgTypeWithdrawLease     = ""
	msgTypeUpdateParams      = ""
	msgTypeLeaseStartReclaim = ""
	msgTypeResizeLease       = ""
	msgTypeAcceptLeaseResize = ""
	msgTypeRejectLeaseResize = ""
	msgTypeCancelLeaseResize = ""
)

var (
	_ sdk.Msg            = &MsgCreateBid{}
	_ sdk.LegacyMsg      = &MsgCreateBid{}
	_ deposit.HasDeposit = &MsgCreateBid{}
	_ sdk.Msg            = &MsgCloseBid{}
	_ sdk.Msg            = &MsgCreateLease{}
	_ sdk.Msg            = &MsgWithdrawLease{}
	_ sdk.Msg            = &MsgCloseLease{}
	_ sdk.Msg            = &MsgUpdateParams{}
	_ sdk.Msg            = &MsgLeaseStartReclaim{}
	_ sdk.Msg            = &MsgResizeLease{}
	_ sdk.Msg            = &MsgAcceptLeaseResize{}
	_ sdk.Msg            = &MsgRejectLeaseResize{}
	_ sdk.Msg            = &MsgCancelLeaseResize{}
)

func init() {
	msgTypeCreateBid = reflect.TypeOf(&MsgCreateBid{}).Elem().Name()
	msgTypeCloseBid = reflect.TypeOf(&MsgCloseBid{}).Elem().Name()
	msgTypeCreateLease = reflect.TypeOf(&MsgCreateLease{}).Elem().Name()
	msgTypeCloseLease = reflect.TypeOf(&MsgCloseLease{}).Elem().Name()
	msgTypeWithdrawLease = reflect.TypeOf(&MsgWithdrawLease{}).Elem().Name()
	msgTypeUpdateParams = reflect.TypeOf(&MsgUpdateParams{}).Elem().Name()
	msgTypeLeaseStartReclaim = reflect.TypeOf(&MsgLeaseStartReclaim{}).Elem().Name()
	msgTypeResizeLease = reflect.TypeOf(&MsgResizeLease{}).Elem().Name()
	msgTypeAcceptLeaseResize = reflect.TypeOf(&MsgAcceptLeaseResize{}).Elem().Name()
	msgTypeRejectLeaseResize = reflect.TypeOf(&MsgRejectLeaseResize{}).Elem().Name()
	msgTypeCancelLeaseResize = reflect.TypeOf(&MsgCancelLeaseResize{}).Elem().Name()
}

// NewMsgCreateBid creates a new MsgCreateBid instance
func NewMsgCreateBid(id v1.BidID, price sdk.DecCoin, deposit deposit.Deposit, roffer ResourcesOffer) *MsgCreateBid {
	return &MsgCreateBid{
		ID:             id,
		Price:          price,
		Deposit:        deposit,
		ResourcesOffer: roffer,
	}
}

// Type implements the sdk.Msg interface
func (msg *MsgCreateBid) Type() string { return msgTypeCreateBid }

// GetSigners defines whose signature is required
func (msg *MsgCreateBid) GetSigners() []sdk.AccAddress {
	provider, err := sdk.AccAddressFromBech32(msg.ID.Provider)
	if err != nil {
		panic(err)
	}

	return []sdk.AccAddress{provider}
}

// ValidateBasic does basic validation of provider and order
func (msg *MsgCreateBid) ValidateBasic() error {
	if err := msg.ID.Validate(); err != nil {
		return err
	}

	if err := msg.Deposit.Validate(); err != nil {
		return err
	}

	provider, err := sdk.AccAddressFromBech32(msg.ID.Provider)
	if err != nil {
		return v1.ErrEmptyProvider
	}

	owner, err := sdk.AccAddressFromBech32(msg.ID.Owner)
	if err != nil {
		return fmt.Errorf("%w: empty owner", v1.ErrInvalidBid)
	}

	if provider.Equals(owner) {
		return v1.ErrSameAccount
	}

	if msg.Price.IsZero() {
		return v1.ErrBidZeroPrice
	}

	if msg.ReclamationWindow != nil && *msg.ReclamationWindow < 0 {
		return v1.ErrReclamationWindowInvalid
	}

	return nil
}

// NewMsgWithdrawLease creates a new MsgWithdrawLease instance
func NewMsgWithdrawLease(id v1.LeaseID) *MsgWithdrawLease {
	return &MsgWithdrawLease{
		ID: id,
	}
}

// Type implements the sdk.Msg interface
func (msg *MsgWithdrawLease) Type() string { return msgTypeWithdrawLease }

// GetSigners defines whose signature is required
func (msg *MsgWithdrawLease) GetSigners() []sdk.AccAddress {
	provider, err := sdk.AccAddressFromBech32(msg.GetID().Provider)
	if err != nil {
		panic(err)
	}

	return []sdk.AccAddress{provider}
}

// ValidateBasic does basic validation of provider and order
func (msg *MsgWithdrawLease) ValidateBasic() error {
	if err := msg.ID.Validate(); err != nil {
		return err
	}
	return nil
}

// NewMsgCreateLease creates a new MsgCreateLease instance
func NewMsgCreateLease(id v1.BidID) *MsgCreateLease {
	return &MsgCreateLease{
		BidID: id,
	}
}

// Type implements the sdk.Msg interface
func (msg *MsgCreateLease) Type() string { return msgTypeCreateLease }

// GetSigners defines whose signature is required
func (msg *MsgCreateLease) GetSigners() []sdk.AccAddress {
	provider, err := sdk.AccAddressFromBech32(msg.BidID.Owner)
	if err != nil {
		panic(err)
	}

	return []sdk.AccAddress{provider}
}

// ValidateBasic method for MsgCreateLease
func (msg *MsgCreateLease) ValidateBasic() error {
	return msg.BidID.Validate()
}

// NewMsgCloseBid creates a new MsgCloseBid instance
func NewMsgCloseBid(id v1.BidID, reason v1.LeaseClosedReason) *MsgCloseBid {
	return &MsgCloseBid{
		ID:     id,
		Reason: reason,
	}
}

// Type implements the sdk.Msg interface
func (msg *MsgCloseBid) Type() string { return msgTypeCloseBid }

// GetSigners defines whose signature is required
func (msg *MsgCloseBid) GetSigners() []sdk.AccAddress {
	provider, err := sdk.AccAddressFromBech32(msg.ID.Provider)
	if err != nil {
		panic(err)
	}

	return []sdk.AccAddress{provider}
}

// ValidateBasic method for MsgCloseBid
func (msg *MsgCloseBid) ValidateBasic() error {
	if err := msg.ID.Validate(); err != nil {
		return err
	}

	if !msg.Reason.IsRange(v1.LeaseClosedReasonRangeProvider) {
		return cerrors.Wrapf(v1.ErrInvalidLeaseClosedReason, "value \"%d\" range 10000..19999", msg.Reason)
	}

	return nil
}

// NewMsgCloseLease creates a new MsgCloseLease instance
func NewMsgCloseLease(id v1.LeaseID) *MsgCloseLease {
	return &MsgCloseLease{
		ID: id,
	}
}

// Type implements the sdk.Msg interface
func (msg *MsgCloseLease) Type() string { return msgTypeCloseLease }

// GetSigners defines whose signature is required
func (msg *MsgCloseLease) GetSigners() []sdk.AccAddress {
	owner, err := sdk.AccAddressFromBech32(msg.ID.Owner)
	if err != nil {
		panic(err)
	}

	return []sdk.AccAddress{owner}
}

// ValidateBasic method for MsgCloseLease
func (msg *MsgCloseLease) ValidateBasic() error {
	return msg.ID.Validate()
}

// Type implements the sdk.Msg interface
func (msg *MsgUpdateParams) Type() string { return msgTypeUpdateParams }

// GetSigners returns the expected signers for a MsgUpdateParams message.
func (msg *MsgUpdateParams) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

// ValidateBasic does a sanity check on the provided data.
func (msg *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return cerrors.Wrap(err, "invalid authority address")
	}

	if err := msg.Params.Validate(); err != nil {
		return err
	}

	return nil
}

// NewMsgLeaseStartReclaim creates a new MsgLeaseStartReclaim instance
func NewMsgLeaseStartReclaim(id v1.LeaseID, reason v1.LeaseClosedReason) *MsgLeaseStartReclaim {
	return &MsgLeaseStartReclaim{
		ID:     id,
		Reason: reason,
	}
}

// Type implements the sdk.Msg interface
func (msg *MsgLeaseStartReclaim) Type() string { return msgTypeLeaseStartReclaim }

// GetSigners defines whose signature is required
func (msg *MsgLeaseStartReclaim) GetSigners() []sdk.AccAddress {
	provider, err := sdk.AccAddressFromBech32(msg.ID.Provider)
	if err != nil {
		panic(err)
	}

	return []sdk.AccAddress{provider}
}

// ValidateBasic method for MsgLeaseStartReclaim
func (msg *MsgLeaseStartReclaim) ValidateBasic() error {
	if err := msg.ID.Validate(); err != nil {
		return err
	}

	if !msg.Reason.IsRange(v1.LeaseClosedReasonRangeProvider) {
		return cerrors.Wrapf(v1.ErrInvalidLeaseClosedReason, "value \"%d\" range 10000..19999", msg.Reason)
	}

	return nil
}

// NewMsgResizeLease creates a new MsgResizeLease instance
func NewMsgResizeLease(id v1.LeaseID, resources dvbeta.ResourceUnits, hash []byte) *MsgResizeLease {
	return &MsgResizeLease{
		ID:        id,
		Resources: resources,
		Hash:      hash,
	}
}

// Type implements the sdk.Msg interface
func (msg *MsgResizeLease) Type() string { return msgTypeResizeLease }

// GetSigners defines whose signature is required
func (msg *MsgResizeLease) GetSigners() []sdk.AccAddress {
	owner, err := sdk.AccAddressFromBech32(msg.ID.Owner)
	if err != nil {
		panic(err)
	}

	return []sdk.AccAddress{owner}
}

// ValidateBasic method for MsgResizeLease. What the new resources may change
// depends on the stored group, so the full check runs on chain.
func (msg *MsgResizeLease) ValidateBasic() error {
	if err := msg.ID.Validate(); err != nil {
		return err
	}

	if len(msg.Resources) == 0 {
		return cerrors.Wrap(v1.ErrInvalidLeaseResize, "no resources")
	}

	if len(msg.Hash) != 0 && len(msg.Hash) != dv1.HashLength {
		return cerrors.Wrapf(v1.ErrInvalidLeaseResize, "hash must be %d bytes", dv1.HashLength)
	}

	return nil
}

// NewMsgAcceptLeaseResize creates a new MsgAcceptLeaseResize instance
func NewMsgAcceptLeaseResize(id v1.LeaseID, price sdk.DecCoin) *MsgAcceptLeaseResize {
	return &MsgAcceptLeaseResize{
		ID:    id,
		Price: price,
	}
}

// Type implements the sdk.Msg interface
func (msg *MsgAcceptLeaseResize) Type() string { return msgTypeAcceptLeaseResize }

// GetSigners defines whose signature is required
func (msg *MsgAcceptLeaseResize) GetSigners() []sdk.AccAddress {
	provider, err := sdk.AccAddressFromBech32(msg.ID.Provider)
	if err != nil {
		panic(err)
	}

	return []sdk.AccAddress{provider}
}

// ValidateBasic method for MsgAcceptLeaseResize
func (msg *MsgAcceptLeaseResize) ValidateBasic() error {
	if err := msg.ID.Validate(); err != nil {
		return err
	}

	if err := msg.Price.Validate(); err != nil {
		return cerrors.Wrap(v1.ErrInvalidLeaseResize, err.Error())
	}

	if !msg.Price.IsPositive() {
		return cerrors.Wrap(v1.ErrInvalidLeaseResize, "price must be positive")
	}

	return nil
}

// NewMsgRejectLeaseResize creates a new MsgRejectLeaseResize instance
func NewMsgRejectLeaseResize(id v1.LeaseID, reason v1.LeaseResizeCancelReason) *MsgRejectLeaseResize {
	return &MsgRejectLeaseResize{
		ID:     id,
		Reason: reason,
	}
}

// Type implements the sdk.Msg interface
func (msg *MsgRejectLeaseResize) Type() string { return msgTypeRejectLeaseResize }

// GetSigners defines whose signature is required
func (msg *MsgRejectLeaseResize) GetSigners() []sdk.AccAddress {
	provider, err := sdk.AccAddressFromBech32(msg.ID.Provider)
	if err != nil {
		panic(err)
	}

	return []sdk.AccAddress{provider}
}

// ValidateBasic method for MsgRejectLeaseResize
func (msg *MsgRejectLeaseResize) ValidateBasic() error {
	if err := msg.ID.Validate(); err != nil {
		return err
	}

	if !msg.Reason.IsProviderReject() {
		return cerrors.Wrapf(v1.ErrInvalidLeaseResize, "reason %q is not a provider rejection", msg.Reason)
	}

	return nil
}

// NewMsgCancelLeaseResize creates a new MsgCancelLeaseResize instance
func NewMsgCancelLeaseResize(id v1.LeaseID) *MsgCancelLeaseResize {
	return &MsgCancelLeaseResize{
		ID: id,
	}
}

// Type implements the sdk.Msg interface
func (msg *MsgCancelLeaseResize) Type() string { return msgTypeCancelLeaseResize }

// GetSigners defines whose signature is required
func (msg *MsgCancelLeaseResize) GetSigners() []sdk.AccAddress {
	owner, err := sdk.AccAddressFromBech32(msg.ID.Owner)
	if err != nil {
		panic(err)
	}

	return []sdk.AccAddress{owner}
}

// ValidateBasic method for MsgCancelLeaseResize
func (msg *MsgCancelLeaseResize) ValidateBasic() error {
	return msg.ID.Validate()
}

// ============= GetSignBytes =============
// ModuleCdc is defined in codec.go
// TODO @troian to check if we need them at all

// GetSignBytes encodes the message for signing
//
// Deprecated: GetSignBytes is deprecated
func (msg *MsgCreateBid) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}

// GetSignBytes encodes the message for signing
//
// Deprecated: GetSignBytes is deprecated
func (msg *MsgWithdrawLease) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg)) // nolint: staticcheck
}

// GetSignBytes encodes the message for signing
func (msg *MsgCreateLease) GetSignBytes() []byte {
	//
	// Deprecated: GetSignBytes is deprecated
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg)) // nolint: staticcheck
}

// GetSignBytes encodes the message for signing
//
// Deprecated: GetSignBytes is deprecated
func (msg *MsgCloseBid) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg)) // nolint: staticcheck
}

// GetSignBytes encodes the message for signing
//
// Deprecated: GetSignBytes is deprecated
func (msg *MsgCloseLease) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg)) // nolint: staticcheck
}

// GetSignBytes encodes the message for signing
//
// Deprecated: GetSignBytes is deprecated
func (msg *MsgUpdateParams) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg)) // nolint: staticcheck
}

// GetSignBytes encodes the message for signing
//
// Deprecated: GetSignBytes is deprecated
func (msg *MsgLeaseStartReclaim) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg)) // nolint: staticcheck
}

// GetSignBytes encodes the message for signing
//
// Deprecated: GetSignBytes is deprecated
func (msg *MsgResizeLease) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg)) // nolint: staticcheck
}

// GetSignBytes encodes the message for signing
//
// Deprecated: GetSignBytes is deprecated
func (msg *MsgAcceptLeaseResize) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg)) // nolint: staticcheck
}

// GetSignBytes encodes the message for signing
//
// Deprecated: GetSignBytes is deprecated
func (msg *MsgRejectLeaseResize) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg)) // nolint: staticcheck
}

// GetSignBytes encodes the message for signing
//
// Deprecated: GetSignBytes is deprecated
func (msg *MsgCancelLeaseResize) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg)) // nolint: staticcheck
}

// ============= Route =============
// ModuleCdc is defined in codec.go
// TODO @troian to check if we need them at all since sdk.Msg does not not have Route defined anymore

// Route implements the sdk.Msg interface
//
// Deprecated: Route is deprecated
func (msg *MsgCreateBid) Route() string { return v1.RouterKey }

// Route implements the sdk.Msg interface
//
// Deprecated: Route is deprecated
func (msg *MsgWithdrawLease) Route() string { return v1.RouterKey }

// Route implements the sdk.Msg interface
//
// Deprecated: Route is deprecated
func (msg *MsgCreateLease) Route() string { return v1.RouterKey }

// Route implements the sdk.Msg interface
//
// Deprecated: Route is deprecated
func (msg *MsgCloseBid) Route() string { return v1.RouterKey }

// Route implements the sdk.Msg interface
//
// Deprecated: Route is deprecated
func (msg *MsgCloseLease) Route() string { return v1.RouterKey }

// Route implements the sdk.Msg interface
//
// Deprecated: Route is deprecated
func (msg *MsgUpdateParams) Route() string { return v1.RouterKey }

// Route implements the sdk.Msg interface
//
// Deprecated: Route is deprecated
func (msg *MsgLeaseStartReclaim) Route() string { return v1.RouterKey }

// Route implements the sdk.Msg interface
//
// Deprecated: Route is deprecated
func (msg *MsgResizeLease) Route() string { return v1.RouterKey }

// Route implements the sdk.Msg interface
//
// Deprecated: Route is deprecated
func (msg *MsgAcceptLeaseResize) Route() string { return v1.RouterKey }

// Route implements the sdk.Msg interface
//
// Deprecated: Route is deprecated
func (msg *MsgRejectLeaseResize) Route() string { return v1.RouterKey }

// Route implements the sdk.Msg interface
//
// Deprecated: Route is deprecated
func (msg *MsgCancelLeaseResize) Route() string { return v1.RouterKey }
