package v1

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	sdkerrors "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	dsdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// vidValidationRegex is the DNS-label grammar the vid data-continuity
// label must match.
var vidValidationRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

// ValidateVID validates the tenant-chosen data-continuity label against
// the DNS-label grammar [a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?.
func ValidateVID(vid string) error {
	if !vidValidationRegex.MatchString(vid) {
		return fmt.Errorf("%w: vid %q does not match DNS-label grammar", ErrInvalidRequest, vid)
	}

	return nil
}

// Dup returns a deep copy of the volume reference.
func (v *VolumeRef) Dup() *VolumeRef {
	if v == nil {
		return nil
	}

	res := *v

	return &res
}

// Dup returns a deep copy of the volume policy.
func (v *VolumePolicy) Dup() *VolumePolicy {
	if v == nil {
		return nil
	}

	res := *v
	res.Adopt = v.Adopt.Dup()
	res.ReplicaOf = v.ReplicaOf.Dup()

	return &res
}

// GroupID method returns the GroupID of the volume group this reference
// addresses.
func (v VolumeRef) GroupID() GroupID {
	return GroupID{
		Owner: v.Owner,
		DSeq:  v.DSeq,
		GSeq:  v.GSeq,
	}
}

// Equals method compares this volume reference with the provided one.
func (v VolumeRef) Equals(other VolumeRef) bool {
	return v.GroupID().Equals(other.GroupID()) && v.Name == other.Name
}

// Validate method performs stateless validation of the volume reference.
func (v VolumeRef) Validate() error {
	if _, err := sdk.AccAddressFromBech32(v.Owner); err != nil {
		return sdkerrors.Wrap(dsdkerrors.ErrInvalidAddress, "VolumeRef: Invalid Owner Address")
	}
	if v.DSeq == 0 {
		return sdkerrors.Wrap(dsdkerrors.ErrInvalidSequence, "VolumeRef: Invalid Deployment Sequence")
	}
	if v.GSeq == 0 {
		return sdkerrors.Wrap(dsdkerrors.ErrInvalidSequence, "VolumeRef: Invalid Group Sequence")
	}
	if v.Name == "" {
		return sdkerrors.Wrap(ErrInvalidRequest, "VolumeRef: Empty Name")
	}

	return nil
}

// String method provides human-readable representation of VolumeRef
// as "owner/dseq/gseq/name". This is the canonical form mirrored by
// the manifest StorageParams.volume field.
func (v VolumeRef) String() string {
	return fmt.Sprintf("%s/%d/%d/%s", v.Owner, v.DSeq, v.GSeq, v.Name)
}

// ParseVolumeRef parses a "owner/dseq/gseq/name" string into a VolumeRef.
func ParseVolumeRef(val string) (VolumeRef, error) {
	parts := strings.Split(val, "/")
	if len(parts) != 4 {
		return VolumeRef{}, ErrInvalidIDPath
	}

	owner, err := sdk.AccAddressFromBech32(parts[0])
	if err != nil {
		return VolumeRef{}, err
	}

	dseq, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return VolumeRef{}, err
	}

	gseq, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return VolumeRef{}, err
	}

	if parts[3] == "" {
		return VolumeRef{}, ErrInvalidIDPath
	}

	return VolumeRef{
		Owner: owner.String(),
		DSeq:  dseq,
		GSeq:  uint32(gseq),
		Name:  parts[3],
	}, nil
}
