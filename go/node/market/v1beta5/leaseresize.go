package v1beta5

import (
	"crypto/sha256"
	"time"
)

// LeaseResizeDigestSize is the length of a lease resize request's digest.
const LeaseResizeDigestSize = sha256.Size

// Expired reports whether the request can no longer be accepted at now.
func (r LeaseResize) Expired(now time.Time) bool {
	return now.Unix() > r.ExpiresAt
}

// ComputeDigest returns the request's digest: the SHA-256 of its encoding with
// the digest left empty. The chain sets it when the request is made; a
// provider answering the request copies the stored one rather than computing
// it, so a provider built against an older encoding still names it exactly.
func (r LeaseResize) ComputeDigest() ([]byte, error) {
	r.Digest = nil

	bz, err := r.Marshal()
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(bz)

	return sum[:], nil
}
