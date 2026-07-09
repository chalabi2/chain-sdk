package cli

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"pkg.akt.dev/go/sdl"
)

const volumeTestOwner = "akash1qufa3xzrf34qvwyejkhvnc4psf49fftaw0asmh"

// canonicalVolumeManifestVersion is the golden hash of the canonical empty
// manifest of the volume fixture below (a single group named "us-west" with
// an empty service list). It is what `tx volume create` signs into
// MsgCreateDeployment.Hash and MUST NOT change across client versions; the
// same golden is pinned in the sdl package tests.
const canonicalVolumeManifestVersion = "74066c155b207484e1e3671548c44e5c9dff4d2fbd1293f61fd4f7f466a24e0e"

const sdlVolumeDeployment = `
version: "2.2"

volumes:
  myapp-pgdata:
    size: 200Gi
    class: beta3
    lifecycle:
      reclaim: retain
      retention: 168h
    max-replicas: 2

profiles:
  placement:
    us-west:
      attributes:
        region: us-west
      pricing:
        myapp-pgdata:
          denom: uakt
          amount: 150

deployment:
  myapp-pgdata:
    us-west:
      profile: myapp-pgdata
      count: 1
`

const sdlComputeDeployment = `
version: "2.2"

services:
  web:
    image: nginx
    expose:
      - port: 80
        as: 80
        to:
          - global: true

profiles:
  compute:
    web:
      resources:
        cpu:
          units: "0.1"
        memory:
          size: "128Mi"
        storage:
          size: "512Mi"
  placement:
    anywhere:
      pricing:
        web:
          denom: uakt
          amount: 100

deployment:
  web:
    anywhere:
      profile: web
      count: 1
`

func TestVolumeGroupsFromSDL(t *testing.T) {
	obj, err := sdl.Read([]byte(sdlVolumeDeployment), sdl.WithOwner(volumeTestOwner))
	require.NoError(t, err)

	volumes, err := volumeGroupsFromSDL(obj)
	require.NoError(t, err)
	require.Len(t, volumes, 1)

	group := volumes[0]
	require.NotNil(t, group.Volume)
	require.Equal(t, "myapp-pgdata", group.Volume.Vid)

	// the deployment hash the create command signs is the canonical
	// empty-manifest version — consensus-adjacent, pinned as a golden
	version, err := obj.Version()
	require.NoError(t, err)
	require.Equal(t, canonicalVolumeManifestVersion, hex.EncodeToString(version))
}

func TestVolumeGroupsFromSDLRejectsComputeSDL(t *testing.T) {
	obj, err := sdl.Read([]byte(sdlComputeDeployment), sdl.WithOwner(volumeTestOwner))
	require.NoError(t, err)

	_, err = volumeGroupsFromSDL(obj)
	require.ErrorIs(t, err, errVolumeCreate)
	require.ErrorIs(t, err, errVolumeCreateNoVolumes)
}
