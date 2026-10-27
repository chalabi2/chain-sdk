package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	mtypes "pkg.akt.dev/go/node/market/v1"
)

// BucketKeyAccess is what a bucket access key may do.
type BucketKeyAccess string

const (
	// BucketKeyReadWrite reads, writes and deletes objects. It cannot change the
	// bucket's ACLs or policy, so a key holder cannot make the bucket public.
	BucketKeyReadWrite BucketKeyAccess = "readwrite"
	// BucketKeyRead lists and reads objects.
	BucketKeyRead BucketKeyAccess = "read"
)

// BucketKeyInfo describes an access key without its secret.
type BucketKeyInfo struct {
	Name        string          `json:"name"`
	Access      BucketKeyAccess `json:"access"`
	AccessKeyID string          `json:"access_key_id"`
}

// BucketKey is a newly created access key. The provider returns the secret
// once, in the response that creates the key, and never again.
type BucketKey struct {
	BucketKeyInfo
	SecretAccessKey string `json:"secret_access_key"`
}

// BucketKeyRequest asks for a new access key.
type BucketKeyRequest struct {
	Name   string          `json:"name"`
	Access BucketKeyAccess `json:"access"`
}

// BucketInfo is what a bucket lease's owner needs to reach the bucket.
type BucketInfo struct {
	Bucket    string          `json:"bucket"`
	Endpoint  string          `json:"endpoint"`
	Region    string          `json:"region"`
	SizeBytes uint64          `json:"size_bytes"`
	UsedBytes uint64          `json:"used_bytes"`
	Keys      []BucketKeyInfo `json:"keys"`
}

func (c *client) BucketInfo(ctx context.Context, id mtypes.LeaseID) (BucketInfo, error) {
	var info BucketInfo
	err := c.bucketRequest(ctx, http.MethodGet, BucketPath(id), nil, &info)
	return info, err
}

func (c *client) CreateBucketKey(ctx context.Context, id mtypes.LeaseID, req BucketKeyRequest) (BucketKey, error) {
	var key BucketKey
	err := c.bucketRequest(ctx, http.MethodPost, BucketKeysPath(id), req, &key)
	return key, err
}

func (c *client) RevokeBucketKey(ctx context.Context, id mtypes.LeaseID, name string) error {
	return c.bucketRequest(ctx, http.MethodDelete, BucketKeyPath(id, name), nil, nil)
}

// bucketRequest sends an authenticated JSON request and accepts any 2xx, since
// creating a key answers 201 and revoking one answers 204.
func (c *client) bucketRequest(ctx context.Context, method, path string, body, out any) error {
	uri, err := MakeURI(c.host, path)
	if err != nil {
		return err
	}

	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, uri, reqBody)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", contentTypeJSON)
	if err = c.setAuth(req.Header); err != nil {
		return err
	}

	resp, err := c.NewReqClient(ctx).Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	buf := &bytes.Buffer{}
	if _, err = io.Copy(buf, resp.Body); err != nil {
		return err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ClientResponseError{Status: resp.StatusCode, Message: buf.String()}
	}

	if out == nil || buf.Len() == 0 {
		return nil
	}

	return json.NewDecoder(buf).Decode(out)
}
