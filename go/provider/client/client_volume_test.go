package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLeaseVolume(t *testing.T) {
	status := LeaseVolumeStatus{Phase: "Importing", SizeBytes: 1 << 30, ImportFrom: "akash1previous"}
	cl := newBucketTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/lease/1/2/3/volume", r.URL.Path)
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(status)
	})

	got, err := cl.LeaseVolume(context.Background(), bucketTestLease())

	require.NoError(t, err)
	require.Equal(t, status, got)
}

func TestLeaseVolumeOfALeaseWithoutOne(t *testing.T) {
	cl := newBucketTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("no volume for this lease"))
	})

	_, err := cl.LeaseVolume(context.Background(), bucketTestLease())

	require.Equal(t, ClientResponseError{Status: http.StatusNotFound, Message: "no volume for this lease"}, err)
}
