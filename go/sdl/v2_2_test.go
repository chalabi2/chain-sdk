package sdl

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	manifest "pkg.akt.dev/go/manifest/v2beta4"
	dv1 "pkg.akt.dev/go/node/deployment/v1"
	atypes "pkg.akt.dev/go/node/types/attributes/v1"
	rtypes "pkg.akt.dev/go/node/types/resources/v1beta4"
)

const (
	testOwner      = "akash1qufa3xzrf34qvwyejkhvnc4psf49fftaw0asmh"
	testOtherOwner = "akash1365yvmc4s7awdyj3n2sav7xfx76adc6dnmlx63"
)

// canonicalVolumeManifestVersion is the golden hash of the canonical empty
// manifest of the volume-deployment fixture (a single group named "us-west"
// with an empty service list). It is consensus-adjacent — the CLI computes
// it for MsgCreateDeployment.Hash and the provider re-derives it — and MUST
// NOT change across client versions.
const canonicalVolumeManifestVersion = "74066c155b207484e1e3671548c44e5c9dff4d2fbd1293f61fd4f7f466a24e0e"

const sdlV2_2VolumeDeployment = `
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

const sdlV2_2AttachFmt = `
version: "2.2"

volumes:
  myapp-pgdata:
    external:
      %s

services:
  db:
    image: postgres:16
    expose:
      - port: 5432
        to:
          - global: true
    params:
      storage:
        data:
          mount: /var/lib/postgresql/data
          volume: myapp-pgdata

profiles:
  compute:
    db:
      resources:
        cpu:
          units: 2
        memory:
          size: 4Gi
        storage: []
  placement:
    anywhere:
      pricing:
        db:
          denom: uakt
          amount: 800

deployment:
  db:
    anywhere:
      profile: db
      count: 1
`

func TestV2_2_VersionGate(t *testing.T) {
	mkSDL := func(version string) []byte {
		return []byte(fmt.Sprintf(`
version: "%s"

services:
  web:
    image: nginx
    expose:
      - port: 80
        to:
          - global: true

profiles:
  compute:
    web:
      resources:
        cpu:
          units: 1
        memory:
          size: 128Mi
        storage:
          size: 512Mi
  placement:
    westcoast:
      pricing:
        web:
          denom: uakt
          amount: 50

deployment:
  web:
    westcoast:
      profile: web
      count: 1
`, version))
	}

	for _, tc := range []struct {
		version string
		data    interface{}
	}{
		{"2.0", &v2{}},
		{"2.1", &v2_1{}},
		{"2.1.5", &v2_1{}},
		{"2.2", &v2_2{}},
		{"2.9", &v2_2{}},
	} {
		obj, err := Read(mkSDL(tc.version))
		require.NoError(t, err, "version %s", tc.version)
		require.IsType(t, tc.data, obj.(*sdl).data, "version %s", tc.version)

		// pre-2.2 versions declare no volumes
		vols, err := obj.Volumes()
		require.NoError(t, err)
		require.Empty(t, vols)
	}

	_, err := Read(mkSDL("3.0"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported version")
}

func TestV2_2_VolumeDeployment(t *testing.T) {
	obj, err := Read([]byte(sdlV2_2VolumeDeployment))
	require.NoError(t, err)

	groups, err := obj.DeploymentGroups()
	require.NoError(t, err)
	require.Len(t, groups, 1)

	group := groups[0]
	require.Equal(t, "us-west", group.Name)

	// the volume policy is typed — never storage attributes
	require.NotNil(t, group.Volume)
	require.Equal(t, dv1.VolumePolicy{
		Vid:            "myapp-pgdata",
		Reclaim:        dv1.VolumeReclaimRetain,
		Retention:      168 * time.Hour,
		MaxAttachments: 1,
		MaxReplicas:    2,
	}, *group.Volume)

	// capabilities/storage/volumes is injected into the placement
	// requirements so only opted-in providers match the order
	require.Equal(t, atypes.Attributes{
		{Key: StorageCapabilityVolumes, Value: "true"},
		{Key: "region", Value: "us-west"},
	}, group.Requirements.Attributes)

	// present-but-zero compute legs, exactly one storage entry
	require.Len(t, group.Resources, 1)
	res := group.Resources[0]
	require.Equal(t, uint32(1), res.Count)
	require.Empty(t, res.Volumes)

	require.NotNil(t, res.CPU)
	require.Equal(t, uint64(0), res.CPU.Units.Value())
	require.NotNil(t, res.Memory)
	require.Equal(t, uint64(0), res.Memory.Quantity.Value())
	require.NotNil(t, res.GPU)
	require.Equal(t, uint64(0), res.GPU.Units.Value())
	require.Empty(t, res.Endpoints)

	require.Equal(t, rtypes.Volumes{
		{
			Name:     "myapp-pgdata",
			Quantity: rtypes.NewResourceValue(200 * 1024 * 1024 * 1024),
			Attributes: atypes.Attributes{
				{Key: StorageAttributeClass, Value: "beta3"},
				{Key: StorageAttributePersistent, Value: "true"},
			},
		},
	}, res.Storage)

	// Volumes() surfaces the storage-only groups
	vols, err := obj.Volumes()
	require.NoError(t, err)
	require.Len(t, vols, 1)
	require.Equal(t, group, vols[0])

	// the manifest is the canonical empty group: on-chain group name, no
	// services — and it must pair with the compiled group spec
	m, err := obj.Manifest()
	require.NoError(t, err)
	require.Equal(t, manifest.Manifest{{Name: "us-west"}}, m)
	require.NoError(t, m.CheckAgainstGSpecs(groups))
}

// TestV2_2_CanonicalVolumeManifestVersion pins the canonical empty-manifest
// hash. The same value must fall out of the SDL parser and of a manifest
// constructed by hand — this is what the CLI signs into
// MsgCreateDeployment.Hash and what providers re-derive.
func TestV2_2_CanonicalVolumeManifestVersion(t *testing.T) {
	obj, err := Read([]byte(sdlV2_2VolumeDeployment))
	require.NoError(t, err)

	version, err := obj.Version()
	require.NoError(t, err)
	require.Equal(t, canonicalVolumeManifestVersion, hex.EncodeToString(version))

	handRolled, err := manifest.Manifest{{Name: "us-west"}}.Version()
	require.NoError(t, err)
	require.Equal(t, canonicalVolumeManifestVersion, hex.EncodeToString(handRolled))
}

func TestV2_2_AttachCompilesIdenticalRefs(t *testing.T) {
	buf := []byte(fmt.Sprintf(sdlV2_2AttachFmt, "dseq: 1234567"))

	obj, err := Read(buf, WithOwner(testOwner))
	require.NoError(t, err)

	groups, err := obj.DeploymentGroups()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Nil(t, groups[0].Volume)

	// no volume groups declared — only referenced
	vols, err := obj.Volumes()
	require.NoError(t, err)
	require.Empty(t, vols)

	ref := dv1.VolumeRef{
		Owner: testOwner,
		DSeq:  1234567,
		GSeq:  1,
		Name:  "myapp-pgdata",
	}

	require.Len(t, groups[0].Resources, 1)
	res := groups[0].Resources[0]
	require.Equal(t, []dv1.VolumeRef{ref}, res.Volumes)

	// the attached volume contributes zero local storage
	require.Empty(t, res.Storage)
	require.NotNil(t, res.Storage)

	m, err := obj.Manifest()
	require.NoError(t, err)
	require.Len(t, m, 1)
	require.Len(t, m[0].Services, 1)

	svc := m[0].Services[0]
	require.NotNil(t, svc.Params)
	require.Equal(t, []manifest.StorageParams{
		{
			Name:     "data",
			Mount:    "/var/lib/postgresql/data",
			ReadOnly: false,
			Volume:   ref.String(),
		},
	}, svc.Params.Storage)

	// identical content both sides — checkAgainstGSpec equality is
	// load-bearing
	require.NoError(t, m.CheckAgainstGSpecs(groups))
}

func TestV2_2_AttachExplicitOwnerWins(t *testing.T) {
	buf := []byte(fmt.Sprintf(sdlV2_2AttachFmt, fmt.Sprintf("owner: %s\n      dseq: 1234567", testOtherOwner)))

	obj, err := Read(buf, WithOwner(testOwner))
	require.NoError(t, err)

	groups, err := obj.DeploymentGroups()
	require.NoError(t, err)
	require.Equal(t, testOtherOwner, groups[0].Resources[0].Volumes[0].Owner)
}

func TestV2_2_AttachRequiresOwner(t *testing.T) {
	buf := []byte(fmt.Sprintf(sdlV2_2AttachFmt, "dseq: 1234567"))

	_, err := Read(buf)
	require.Error(t, err)
	require.Contains(t, err.Error(), "owner is unknown")
}

func TestV2_2_AdoptAndReplicaOf(t *testing.T) {
	mkSDL := func(policy string) []byte {
		return []byte(fmt.Sprintf(`
version: "2.2"

volumes:
  myapp-pgdata:
    size: 10Gi
    class: beta2
    %s

profiles:
  placement:
    us-west:
      pricing:
        myapp-pgdata:
          denom: uakt
          amount: 150

deployment:
  myapp-pgdata:
    us-west:
      profile: myapp-pgdata
      count: 1
`, policy))
	}

	t.Run("adopt", func(t *testing.T) {
		obj, err := Read(mkSDL("adopt:\n      dseq: 555"), WithOwner(testOwner))
		require.NoError(t, err)

		groups, err := obj.DeploymentGroups()
		require.NoError(t, err)
		require.Equal(t, &dv1.VolumeRef{
			Owner: testOwner,
			DSeq:  555,
			GSeq:  1,
			Name:  "myapp-pgdata", // defaults to the declaring volume's name
		}, groups[0].Volume.Adopt)
		require.Nil(t, groups[0].Volume.ReplicaOf)
	})

	t.Run("replica-of", func(t *testing.T) {
		obj, err := Read(mkSDL("replica-of:\n      dseq: 556\n      name: other-name"), WithOwner(testOwner))
		require.NoError(t, err)

		groups, err := obj.DeploymentGroups()
		require.NoError(t, err)
		require.Equal(t, &dv1.VolumeRef{
			Owner: testOwner,
			DSeq:  556,
			GSeq:  1,
			Name:  "other-name",
		}, groups[0].Volume.ReplicaOf)
		require.Nil(t, groups[0].Volume.Adopt)
	})

	t.Run("adopt requires owner", func(t *testing.T) {
		_, err := Read(mkSDL("adopt:\n      dseq: 555"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "requires the owner")
	})

	t.Run("mutually exclusive", func(t *testing.T) {
		_, err := Read(mkSDL("adopt:\n      dseq: 555\n    replica-of:\n      dseq: 556"), WithOwner(testOwner))
		require.Error(t, err)
		require.Contains(t, err.Error(), "mutually exclusive")
	})
}

func TestV2_2_VolumeGrammarErrors(t *testing.T) {
	tests := []struct {
		name   string
		sdl    string
		expErr string
	}{
		{
			// grammar: a volume reference requires a mount
			name: "volume ref without mount",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    external:
      dseq: 1234567

services:
  db:
    image: postgres:16
    expose:
      - port: 5432
        to:
          - global: true
    params:
      storage:
        data:
          volume: myapp-pgdata

profiles:
  compute:
    db:
      resources:
        cpu:
          units: 2
        memory:
          size: 4Gi
        storage: []
  placement:
    anywhere:
      pricing:
        db:
          denom: uakt
          amount: 800

deployment:
  db:
    anywhere:
      profile: db
      count: 1
`,
			expErr: "requires a mount",
		},
		{
			name: "unreferenced volume errors",
			sdl: `
version: "2.2"

volumes:
  unused-data:
    external:
      dseq: 42

services:
  web:
    image: nginx
    expose:
      - port: 80
        to:
          - global: true

profiles:
  compute:
    web:
      resources:
        cpu:
          units: 1
        memory:
          size: 128Mi
        storage:
          size: 512Mi
  placement:
    westcoast:
      pricing:
        web:
          denom: uakt
          amount: 50

deployment:
  web:
    westcoast:
      profile: web
      count: 1
`,
			expErr: `volume "unused-data" declared but never used`,
		},
		{
			name: "count>1 with RWO ref rejected",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    external:
      dseq: 1234567

services:
  db:
    image: postgres:16
    expose:
      - port: 5432
        to:
          - global: true
    params:
      storage:
        data:
          mount: /var/lib/postgresql/data
          volume: myapp-pgdata

profiles:
  compute:
    db:
      resources:
        cpu:
          units: 2
        memory:
          size: 4Gi
        storage: []
  placement:
    anywhere:
      pricing:
        db:
          denom: uakt
          amount: 800

deployment:
  db:
    anywhere:
      profile: db
      count: 2
`,
			expErr: "count must be 1",
		},
		{
			// grammar: volume excludes size/class/attributes on the service
			// side — a matching compute profile storage entry is rejected
			name: "volume ref excludes profile storage entry",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    external:
      dseq: 1234567

services:
  db:
    image: postgres:16
    expose:
      - port: 5432
        to:
          - global: true
    params:
      storage:
        data:
          mount: /var/lib/postgresql/data
          volume: myapp-pgdata

profiles:
  compute:
    db:
      resources:
        cpu:
          units: 2
        memory:
          size: 4Gi
        storage:
          - name: data
            size: 1Gi
  placement:
    anywhere:
      pricing:
        db:
          denom: uakt
          amount: 800

deployment:
  db:
    anywhere:
      profile: db
      count: 1
`,
			expErr: "excludes size/class/attributes",
		},
		{
			name: "volume declaration must be its own deployment",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    size: 10Gi
    class: beta2

services:
  web:
    image: nginx
    expose:
      - port: 80
        to:
          - global: true

profiles:
  compute:
    web:
      resources:
        cpu:
          units: 1
        memory:
          size: 128Mi
        storage:
          size: 512Mi
  placement:
    westcoast:
      pricing:
        web:
          denom: uakt
          amount: 50
        myapp-pgdata:
          denom: uakt
          amount: 150

deployment:
  web:
    westcoast:
      profile: web
      count: 1
  myapp-pgdata:
    westcoast:
      profile: myapp-pgdata
      count: 1
`,
			expErr: "must not declare services",
		},
		{
			name: "one volume per SDL",
			sdl: `
version: "2.2"

volumes:
  vol-one:
    size: 10Gi
    class: beta2
  vol-two:
    size: 10Gi
    class: beta2

profiles:
  placement:
    us-west:
      pricing:
        vol-one:
          denom: uakt
          amount: 150
        vol-two:
          denom: uakt
          amount: 150

deployment:
  vol-one:
    us-west:
      profile: vol-one
      count: 1
  vol-two:
    us-west:
      profile: vol-two
      count: 1
`,
			expErr: "only one volume may be declared",
		},
		{
			name: "ram cannot back a volume",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    size: 10Gi
    class: ram

profiles:
  placement:
    us-west:
      pricing:
        myapp-pgdata:
          denom: uakt
          amount: 150

deployment:
  myapp-pgdata:
    us-west:
      profile: myapp-pgdata
      count: 1
`,
			expErr: "cannot back a volume",
		},
		{
			name: "volume must declare a size",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    class: beta2

profiles:
  placement:
    us-west:
      pricing:
        myapp-pgdata:
          denom: uakt
          amount: 150

deployment:
  myapp-pgdata:
    us-west:
      profile: myapp-pgdata
      count: 1
`,
			expErr: "must declare a size",
		},
		{
			name: "invalid vid grammar",
			sdl: `
version: "2.2"

volumes:
  MyData:
    size: 10Gi
    class: beta2

profiles:
  placement:
    us-west:
      pricing:
        MyData:
          denom: uakt
          amount: 150

deployment:
  MyData:
    us-west:
      profile: MyData
      count: 1
`,
			expErr: "DNS-label grammar",
		},
		{
			name: "invalid reclaim policy",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    size: 10Gi
    class: beta2
    lifecycle:
      reclaim: keep

profiles:
  placement:
    us-west:
      pricing:
        myapp-pgdata:
          denom: uakt
          amount: 150

deployment:
  myapp-pgdata:
    us-west:
      profile: myapp-pgdata
      count: 1
`,
			expErr: "invalid volume lifecycle reclaim",
		},
		{
			name: "invalid retention",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    size: 10Gi
    class: beta2
    lifecycle:
      retention: forever

profiles:
  placement:
    us-west:
      pricing:
        myapp-pgdata:
          denom: uakt
          amount: 150

deployment:
  myapp-pgdata:
    us-west:
      profile: myapp-pgdata
      count: 1
`,
			expErr: "invalid volume lifecycle retention",
		},
		{
			name: "volume deployment count must be 1",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    size: 10Gi
    class: beta2

profiles:
  placement:
    us-west:
      pricing:
        myapp-pgdata:
          denom: uakt
          amount: 150

deployment:
  myapp-pgdata:
    us-west:
      profile: myapp-pgdata
      count: 2
`,
			expErr: "volume deployment count must be 1",
		},
		{
			name: "external ref must not carry declaration fields",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    external:
      dseq: 1234567
    size: 10Gi

services:
  db:
    image: postgres:16
    expose:
      - port: 5432
        to:
          - global: true
    params:
      storage:
        data:
          mount: /data
          volume: myapp-pgdata

profiles:
  compute:
    db:
      resources:
        cpu:
          units: 2
        memory:
          size: 4Gi
        storage: []
  placement:
    anywhere:
      pricing:
        db:
          denom: uakt
          amount: 800

deployment:
  db:
    anywhere:
      profile: db
      count: 1
`,
			expErr: "must not carry declaration fields",
		},
		{
			name: "external dseq must be positive",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    external:
      dseq: 0

services:
  db:
    image: postgres:16
    expose:
      - port: 5432
        to:
          - global: true
    params:
      storage:
        data:
          mount: /data
          volume: myapp-pgdata

profiles:
  compute:
    db:
      resources:
        cpu:
          units: 2
        memory:
          size: 4Gi
        storage: []
  placement:
    anywhere:
      pricing:
        db:
          denom: uakt
          amount: 800

deployment:
  db:
    anywhere:
      profile: db
      count: 1
`,
			expErr: "external dseq must be > 0",
		},
		{
			name: "service cannot reference a declared volume",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    size: 10Gi
    class: beta2

services:
  db:
    image: postgres:16
    expose:
      - port: 5432
        to:
          - global: true
    params:
      storage:
        data:
          mount: /data
          volume: myapp-pgdata

profiles:
  compute:
    db:
      resources:
        cpu:
          units: 2
        memory:
          size: 4Gi
        storage: []
  placement:
    anywhere:
      pricing:
        db:
          denom: uakt
          amount: 800

deployment:
  db:
    anywhere:
      profile: db
      count: 1
`,
			expErr: "must not declare services",
		},
		{
			name: "service references non-existing volume",
			sdl: `
version: "2.2"

services:
  db:
    image: postgres:16
    expose:
      - port: 5432
        to:
          - global: true
    params:
      storage:
        data:
          mount: /data
          volume: no-such-volume

profiles:
  compute:
    db:
      resources:
        cpu:
          units: 2
        memory:
          size: 4Gi
        storage: []
  placement:
    anywhere:
      pricing:
        db:
          denom: uakt
          amount: 800

deployment:
  db:
    anywhere:
      profile: db
      count: 1
`,
			expErr: "non-existing volume",
		},
		{
			name: "volume attaches to a single service",
			sdl: `
version: "2.2"

volumes:
  myapp-pgdata:
    external:
      dseq: 1234567

services:
  db:
    image: postgres:16
    expose:
      - port: 5432
        to:
          - global: true
    params:
      storage:
        data:
          mount: /data
          volume: myapp-pgdata
  backup:
    image: backup:1
    params:
      storage:
        data:
          mount: /backup
          volume: myapp-pgdata

profiles:
  compute:
    db:
      resources:
        cpu:
          units: 2
        memory:
          size: 4Gi
        storage: []
    backup:
      resources:
        cpu:
          units: 1
        memory:
          size: 1Gi
        storage: []
  placement:
    anywhere:
      pricing:
        db:
          denom: uakt
          amount: 800
        backup:
          denom: uakt
          amount: 100

deployment:
  db:
    anywhere:
      profile: db
      count: 1
  backup:
    anywhere:
      profile: backup
      count: 1
`,
			expErr: "single service (RWO)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Read([]byte(tc.sdl), WithOwner(testOwner))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expErr)
		})
	}
}

// pre-2.2 SDL versions reject the volume grammar outright rather than
// silently ignoring it
func TestV2_2_GrammarRejectedByOlderVersions(t *testing.T) {
	t.Run("volumes stanza", func(t *testing.T) {
		_, err := Read([]byte(`
version: "2.1"

volumes:
  myapp-pgdata:
    external:
      dseq: 1234567

services:
  web:
    image: nginx

profiles:
  compute:
    web:
      resources:
        cpu:
          units: 1
        memory:
          size: 128Mi
        storage:
          size: 512Mi
  placement:
    westcoast:
      pricing:
        web:
          denom: uakt
          amount: 50

deployment:
  web:
    westcoast:
      profile: web
      count: 1
`))
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected field volumes")
	})

	t.Run("service volume param", func(t *testing.T) {
		_, err := Read([]byte(`
version: "2.1"

services:
  web:
    image: nginx
    expose:
      - port: 80
        to:
          - global: true
    params:
      storage:
        data:
          mount: /data
          volume: myapp-pgdata

profiles:
  compute:
    web:
      resources:
        cpu:
          units: 1
        memory:
          size: 128Mi
        storage:
          size: 512Mi
  placement:
    westcoast:
      pricing:
        web:
          denom: uakt
          amount: 50

deployment:
  web:
    westcoast:
      profile: web
      count: 1
`))
		require.Error(t, err)
		require.Contains(t, err.Error(), "volume references require SDL version 2.2")
	})
}
