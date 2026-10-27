package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	mtypes "pkg.akt.dev/go/node/market/v1"
)

func TestBucketInfo(t *testing.T) {
	info := BucketInfo{
		Bucket:    "media-1a2b3c4d",
		Endpoint:  "https://s3.provider.example.com",
		Region:    "default",
		SizeBytes: 10 << 30,
		UsedBytes: 512,
		Keys:      []BucketKeyInfo{{Name: "web", Access: BucketKeyRead, AccessKeyID: "AKIDWEB"}},
	}
	cl := newBucketTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/lease/1/2/3/bucket", r.URL.Path)
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(info)
	})

	got, err := cl.BucketInfo(context.Background(), bucketTestLease())

	require.NoError(t, err)
	require.Equal(t, info, got)
}

func TestCreateBucketKey(t *testing.T) {
	cl := newBucketTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/lease/1/2/3/bucket/keys", r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"name":"web","access":"readwrite"}`, string(body))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"web","access":"readwrite","access_key_id":"AKIDWEB","secret_access_key":"shh"}`))
	})

	key, err := cl.CreateBucketKey(context.Background(), bucketTestLease(), BucketKeyRequest{Name: "web", Access: BucketKeyReadWrite})

	require.NoError(t, err)
	require.Equal(t, BucketKey{
		BucketKeyInfo:   BucketKeyInfo{Name: "web", Access: BucketKeyReadWrite, AccessKeyID: "AKIDWEB"},
		SecretAccessKey: "shh",
	}, key)
}

func TestRevokeBucketKey(t *testing.T) {
	cl := newBucketTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/lease/1/2/3/bucket/keys/web%20key", r.URL.EscapedPath())
		w.WriteHeader(http.StatusNoContent)
	})

	require.NoError(t, cl.RevokeBucketKey(context.Background(), bucketTestLease(), "web key"))
}

func TestBucketRequestReportsRefusals(t *testing.T) {
	cl := newBucketTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("bucket is retained"))
	})

	_, err := cl.BucketInfo(context.Background(), bucketTestLease())

	require.Equal(t, ClientResponseError{Status: http.StatusConflict, Message: "bucket is retained"}, err)
}

func newBucketTestClient(t *testing.T, handler http.HandlerFunc) *client {
	t.Helper()

	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)

	host, err := url.Parse(srv.URL)
	require.NoError(t, err)

	return &client{
		ctx:    context.Background(),
		host:   host,
		tlsCfg: srv.Client().Transport.(*http.Transport).TLSClientConfig,
		opts:   clientOptions{token: "test-token"},
	}
}

func bucketTestLease() mtypes.LeaseID {
	return mtypes.LeaseID{Owner: "akash1owner", DSeq: 1, GSeq: 2, OSeq: 3, Provider: "akash1provider"}
}
