package v2beta1

import (
	"time"
)

// Expired reports whether the request can no longer be accepted at now.
func (r LeaseResize) Expired(now time.Time) bool {
	return now.Unix() > r.ExpiresAt
}
