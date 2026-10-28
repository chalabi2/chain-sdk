package rest

import (
	"context"
	"net/http"
	"time"

	mtypes "pkg.akt.dev/go/node/market/v1"
)

// LeaseVolumeStatus is where a volume lease's volume stands on its provider.
type LeaseVolumeStatus struct {
	// Phase is the provider's phase for the volume: Pending, Importing,
	// Provisioned, Attached, Exporting, Adopting, Retained or Releasing.
	Phase     string `json:"phase"`
	SizeBytes uint64 `json:"size_bytes"`
	// ImportFrom is the provider a moved volume's data is still being copied
	// from; empty once it all arrived.
	ImportFrom    string     `json:"import_from,omitempty"`
	AttachedLease string     `json:"attached_lease,omitempty"`
	RetainedUntil *time.Time `json:"retained_until,omitempty"`
}

func (c *client) LeaseVolume(ctx context.Context, id mtypes.LeaseID) (LeaseVolumeStatus, error) {
	var status LeaseVolumeStatus
	err := c.jsonRequest(ctx, http.MethodGet, LeaseVolumePath(id), nil, &status)
	return status, err
}
