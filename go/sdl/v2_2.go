package sdl

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"

	manifest "pkg.akt.dev/go/manifest/v2beta4"
	dv1 "pkg.akt.dev/go/node/deployment/v1"
	dtypes "pkg.akt.dev/go/node/deployment/v1beta5"
	types "pkg.akt.dev/go/node/types/attributes/v1"
)

const (
	// StorageCapabilityVolumes is the placement capability a volume group
	// injects into its PlacementRequirements so only providers that opted
	// into the decoupled storage market match the order.
	StorageCapabilityVolumes = "capabilities/storage/volumes"

	volumeReclaimRetain = "retain"
	volumeReclaimDelete = "delete"
)

var _ SDL = (*v2_2)(nil)

// v2_2VolumeExternal references an existing, separately leased volume
// deployment. The owner is the tx signer and is normally supplied by the
// caller via sdl.WithOwner; an explicit owner in the SDL takes precedence.
type v2_2VolumeExternal struct {
	Owner string `yaml:"owner,omitempty"`
	DSeq  uint64 `yaml:"dseq"`
}

// v2_2VolumeLifecycle carries the volume lifecycle contract.
type v2_2VolumeLifecycle struct {
	Reclaim   string `yaml:"reclaim,omitempty"`   // retain (default) | delete
	Retention string `yaml:"retention,omitempty"` // duration, e.g. "168h"
}

// v2_2VolumePolicyRef names another volume deployment of the same owner
// (adopt / replica-of). Name defaults to the declaring volume's own name.
type v2_2VolumePolicyRef struct {
	DSeq uint64 `yaml:"dseq"`
	Name string `yaml:"name,omitempty"`
}

// v2_2Volume is one entry of the top-level `volumes:` stanza: either an
// external reference (attach grammar) or a new volume declaration (volume
// deployment grammar). The two forms are mutually exclusive.
type v2_2Volume struct {
	External    *v2_2VolumeExternal  `yaml:"external,omitempty"`
	Size        byteQuantity         `yaml:"size,omitempty"`
	Class       string               `yaml:"class,omitempty"`
	Lifecycle   *v2_2VolumeLifecycle `yaml:"lifecycle,omitempty"`
	MaxReplicas uint32               `yaml:"max-replicas,omitempty"`
	Adopt       *v2_2VolumePolicyRef `yaml:"adopt,omitempty"`
	ReplicaOf   *v2_2VolumePolicyRef `yaml:"replica-of,omitempty"`
}

func (v *v2_2Volume) isExternal() bool {
	return v.External != nil
}

// reclaim maps the SDL lifecycle reclaim string onto the on-chain policy.
func (v *v2_2Volume) reclaim() (dv1.VolumePolicy_ReclaimPolicy, error) {
	reclaim := volumeReclaimRetain
	if v.Lifecycle != nil && v.Lifecycle.Reclaim != "" {
		reclaim = v.Lifecycle.Reclaim
	}

	switch reclaim {
	case volumeReclaimRetain:
		return dv1.VolumeReclaimRetain, nil
	case volumeReclaimDelete:
		return dv1.VolumeReclaimDelete, nil
	default:
		return dv1.VolumeReclaimInvalid, fmt.Errorf("%w: invalid volume lifecycle reclaim %q, expected %q or %q",
			errSDLInvalid, reclaim, volumeReclaimRetain, volumeReclaimDelete)
	}
}

// retention parses the SDL lifecycle retention duration.
func (v *v2_2Volume) retention() (time.Duration, error) {
	if v.Lifecycle == nil || v.Lifecycle.Retention == "" {
		return 0, nil
	}

	d, err := time.ParseDuration(v.Lifecycle.Retention)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid volume lifecycle retention %q: %v", errSDLInvalid, v.Lifecycle.Retention, err)
	}

	if d < 0 {
		return 0, fmt.Errorf("%w: volume lifecycle retention must not be negative", errSDLInvalid)
	}

	return d, nil
}

type v2_2 struct {
	Include     []string              `yaml:",omitempty"`
	Services    map[string]v2Service  `yaml:"services,omitempty"`
	Profiles    v2profiles            `yaml:"profiles,omitempty"`
	Deployments v2Deployments         `yaml:"deployment"`
	Endpoints   map[string]v2Endpoint `yaml:"endpoints"`
	VolumesCfg  map[string]v2_2Volume `yaml:"volumes,omitempty"`
	ReclaimCfg  *v2Reclamation        `yaml:"-"`

	// owner is the deployment owner (the tx signer, bech32), supplied via
	// sdl.WithOwner. Volume references compile to on-chain VolumeRefs and
	// manifest volume strings keyed by it, so it must be known before the
	// groups are built. An explicit `external.owner` in the SDL wins.
	owner string

	built    bool
	buildErr error

	result struct {
		dgroups dtypes.GroupSpecs
		mgroups manifest.Groups
	}
}

func (sdl *v2_2) DeploymentGroups() (dtypes.GroupSpecs, error) {
	if err := sdl.build(); err != nil {
		return dtypes.GroupSpecs{}, err
	}

	return sdl.result.dgroups, nil
}

func (sdl *v2_2) Manifest() (manifest.Manifest, error) {
	if err := sdl.build(); err != nil {
		return manifest.Manifest{}, err
	}

	return manifest.Manifest(sdl.result.mgroups), nil
}

// Version creates the deterministic Deployment Version hash from the SDL.
func (sdl *v2_2) Version() ([]byte, error) {
	if err := sdl.build(); err != nil {
		return nil, err
	}

	return manifest.Manifest(sdl.result.mgroups).Version()
}

func (sdl *v2_2) Reclamation() (*dv1.DeploymentReclamation, error) {
	return sdl.ReclaimCfg.toDeploymentReclamation()
}

// Volumes returns the storage-only (volume) groups declared by the SDL.
func (sdl *v2_2) Volumes() (dtypes.GroupSpecs, error) {
	if err := sdl.build(); err != nil {
		return dtypes.GroupSpecs{}, err
	}

	res := dtypes.GroupSpecs{}
	for _, group := range sdl.result.dgroups {
		if group.Volume != nil {
			res = append(res, group)
		}
	}

	return res, nil
}

func (sdl *v2_2) UnmarshalYAML(node *yaml.Node) error {
	result := v2_2{}

loop:
	for i := 0; i < len(node.Content); i += 2 {
		var val interface{}
		switch node.Content[i].Value {
		case "include":
			val = &result.Include
		case "services":
			val = &result.Services
		case "profiles":
			val = &result.Profiles
		case "deployment":
			val = &result.Deployments
		case "endpoints":
			val = &result.Endpoints
		case "volumes":
			val = &result.VolumesCfg
		case "reclamation":
			result.ReclaimCfg = &v2Reclamation{}
			val = result.ReclaimCfg
		case sdlVersionField:
			// version is already verified
			continue loop
		default:
			return fmt.Errorf("sdl: unexpected field %s", node.Content[i].Value)
		}

		if err := node.Content[i+1].Decode(val); err != nil {
			return err
		}
	}

	// Group construction is deferred to build(): volume references compile
	// to owner-keyed VolumeRefs and the owner (sdl.WithOwner) is injected
	// after unmarshalling.
	*sdl = result

	return nil
}

// build runs semantic validation and compiles the groups exactly once. It is
// the single entry point behind validate() and every accessor, so the owner
// injected by Read is always applied before any group is materialized.
func (sdl *v2_2) build() error {
	if sdl.built {
		return sdl.buildErr
	}

	sdl.built = true

	if err := sdl.validateContent(); err != nil {
		sdl.buildErr = err
		return err
	}

	if err := sdl.buildGroups(); err != nil {
		sdl.buildErr = err
		return err
	}

	return nil
}

func (sdl *v2_2) validate() error {
	return sdl.build()
}

// declaredVolumeNames returns the sorted names of the `volumes:` entries
// that declare a new volume (as opposed to referencing an external one).
func (sdl *v2_2) declaredVolumeNames() []string {
	names := make([]string, 0, len(sdl.VolumesCfg))
	for name, vol := range sdl.VolumesCfg {
		if !vol.isExternal() {
			names = append(names, name)
		}
	}

	sort.Strings(names)

	return names
}

// externalVolumeOwner resolves the owner a reference to the named external
// volume compiles with: the explicit `external.owner`, falling back to the
// owner supplied via sdl.WithOwner.
func (sdl *v2_2) externalVolumeOwner(name string) (string, error) {
	vol := sdl.VolumesCfg[name]

	owner := vol.External.Owner
	if owner == "" {
		owner = sdl.owner
	}

	if owner == "" {
		return "", fmt.Errorf(
			"%w: volume %q: owner is unknown; set volumes.%s.external.owner or supply the signer via sdl.WithOwner",
			errSDLInvalid,
			name,
			name,
		)
	}

	return owner, nil
}

// policyRefOwner resolves the owner for adopt/replica-of references: they
// always belong to the deployment owner (the signer).
func (sdl *v2_2) policyRefOwner(volName string, field string) (string, error) {
	if sdl.owner == "" {
		return "", fmt.Errorf(
			"%w: volume %q: %s requires the owner; supply the signer via sdl.WithOwner",
			errSDLInvalid,
			volName,
			field,
		)
	}

	return sdl.owner, nil
}

// validateVolumesStanza validates the top-level `volumes:` entries in
// isolation. Cross-references (deployment section, service params) are
// validated in validateContent.
func (sdl *v2_2) validateVolumesStanza() error {
	names := make([]string, 0, len(sdl.VolumesCfg))
	for name := range sdl.VolumesCfg {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		vol := sdl.VolumesCfg[name]

		// the volume name doubles as the vid data-continuity label
		if err := dv1.ValidateVID(name); err != nil {
			return fmt.Errorf("%w: volume name %q must match the DNS-label grammar", errSDLInvalid, name)
		}

		if _, exists := sdl.Services[name]; exists {
			return fmt.Errorf("%w: volume %q collides with a service of the same name", errSDLInvalid, name)
		}

		if vol.isExternal() {
			if vol.External.DSeq == 0 {
				return fmt.Errorf("%w: volume %q: external dseq must be > 0", errSDLInvalid, name)
			}

			if vol.Size != 0 || vol.Class != "" || vol.Lifecycle != nil ||
				vol.MaxReplicas != 0 || vol.Adopt != nil || vol.ReplicaOf != nil {
				return fmt.Errorf(
					"%w: volume %q: external reference must not carry declaration fields (size/class/lifecycle/max-replicas/adopt/replica-of)",
					errSDLInvalid,
					name,
				)
			}

			continue
		}

		// new volume declaration
		if vol.Size == 0 {
			return fmt.Errorf("%w: volume %q must declare a size", errSDLInvalid, name)
		}

		class := vol.Class
		if class == "" {
			class = StorageClassDefault
		}

		if class == StorageClassRAM {
			return fmt.Errorf("%w: volume %q: storage class %q cannot back a volume", errSDLInvalid, name, StorageClassRAM)
		}

		if _, valid := allowedStorageClasses[class]; !valid {
			return fmt.Errorf("%w: volume %q: invalid storage class %q", errSDLInvalid, name, class)
		}

		if _, err := vol.reclaim(); err != nil {
			return fmt.Errorf("volume %q: %w", name, err)
		}

		if _, err := vol.retention(); err != nil {
			return fmt.Errorf("volume %q: %w", name, err)
		}

		if vol.Adopt != nil && vol.ReplicaOf != nil {
			return fmt.Errorf("%w: volume %q: adopt and replica-of are mutually exclusive", errSDLInvalid, name)
		}

		if vol.Adopt != nil && vol.Adopt.DSeq == 0 {
			return fmt.Errorf("%w: volume %q: adopt dseq must be > 0", errSDLInvalid, name)
		}

		if vol.ReplicaOf != nil && vol.ReplicaOf.DSeq == 0 {
			return fmt.Errorf("%w: volume %q: replica-of dseq must be > 0", errSDLInvalid, name)
		}
	}

	return nil
}

// nolint: gocyclo
func (sdl *v2_2) validateContent() error {
	if sdl.ReclaimCfg != nil {
		if _, err := sdl.ReclaimCfg.toDeploymentReclamation(); err != nil {
			return err
		}
	}

	if err := sdl.validateVolumesStanza(); err != nil {
		return err
	}

	declaredVolumes := sdl.declaredVolumeNames()

	// A volume is its own deployment: a v2.2 SDL either declares exactly one
	// volume (and no services), or declares services (attaching external
	// volumes at most). Mixed files are CLI sugar over two deployments.
	if len(declaredVolumes) > 0 {
		if len(sdl.Services) != 0 {
			return fmt.Errorf(
				"%w: a volume deployment must not declare services; create the volume deployment separately and reference it via volumes.<name>.external",
				errSDLInvalid,
			)
		}

		if len(declaredVolumes) > 1 {
			return fmt.Errorf(
				"%w: only one volume may be declared per SDL (each volume is its own deployment); found %d",
				errSDLInvalid,
				len(declaredVolumes),
			)
		}
	}

	for endpointName, endpoint := range sdl.Endpoints {
		if !endpointNameValidationRegex.MatchString(endpointName) {
			return fmt.Errorf(
				"%w: endpoint named %q is not a valid name",
				errSDLInvalid,
				endpointName,
			)
		}

		if len(endpoint.Kind) == 0 {
			return fmt.Errorf("%w: endpoint named %q has no kind", errSDLInvalid, endpointName)
		}

		// Validate endpoint kind, there is only one allowed value for now
		if endpoint.Kind != endpointKindIP {
			return fmt.Errorf(
				"%w: endpoint named %q, unknown kind %q",
				errSDLInvalid,
				endpointName,
				endpoint.Kind,
			)
		}
	}

	endpointsUsed := make(map[string]struct{})
	portsUsed := make(map[string]string)
	volumesUsed := make(map[string]string) // volume name -> referencing service

	for _, svcName := range sdl.Deployments.svcNames() {
		depl := sdl.Deployments[svcName]

		// volume deployment entry: the deployment section consumes the
		// declared volume directly; there is no service or compute profile.
		if vol, isVolume := sdl.VolumesCfg[svcName]; isVolume && !vol.isExternal() {
			if err := sdl.validateVolumeDeployment(svcName, depl); err != nil {
				return err
			}

			volumesUsed[svcName] = svcName

			continue
		}

		for _, placementName := range v2DeploymentPlacementNames(depl) {
			svcdepl := depl[placementName]

			compute, ok := sdl.Profiles.Compute[svcdepl.Profile]
			if !ok {
				return fmt.Errorf(
					"%w: %v.%v: no compute profile named %v",
					errSDLInvalid,
					svcName,
					placementName,
					svcdepl.Profile,
				)
			}

			infra, ok := sdl.Profiles.Placement[placementName]
			if !ok {
				return fmt.Errorf(
					"%w: %v.%v: no placement profile named %v",
					errSDLInvalid,
					svcName,
					placementName,
					placementName,
				)
			}

			if _, ok := infra.Pricing[svcdepl.Profile]; !ok {
				return fmt.Errorf(
					"%w: %v.%v: no pricing for profile %v",
					errSDLInvalid,
					svcName,
					placementName,
					svcdepl.Profile,
				)
			}

			svc, ok := sdl.Services[svcName]
			if !ok {
				return fmt.Errorf(
					"%w: %v.%v: no service profile named %v",
					errSDLInvalid,
					svcName,
					placementName,
					svcName,
				)
			}

			if svc.Credentials != nil {
				if err := svc.Credentials.validate(); err != nil {
					return fmt.Errorf(
						"%w: %v.%v: %v",
						errSDLInvalid,
						svcName,
						placementName,
						err,
					)
				}
			}

			for _, serviceExpose := range svc.Expose {
				for _, to := range serviceExpose.To {
					// Check to see if an IP endpoint is also specified
					if len(to.IP) != 0 {
						if !to.Global {
							return fmt.Errorf(
								"%w: error on %q if an IP is declared the directive must be declared as global",
								errSDLInvalid,
								svcName,
							)
						}
						endpoint, endpointExists := sdl.Endpoints[to.IP]
						if !endpointExists {
							return fmt.Errorf(
								"%w: error on service %q no endpoint named %q exists",
								errSDLInvalid,
								svcName,
								to.IP,
							)
						}

						if endpoint.Kind != endpointKindIP {
							return fmt.Errorf(
								"%w: error on service %q endpoint %q has type %q, should be %q",
								errSDLInvalid,
								svcName,
								to.IP,
								endpoint.Kind,
								endpointKindIP,
							)
						}

						endpointsUsed[to.IP] = struct{}{}

						// Endpoint exists. Now check for port collisions across a single endpoint, port, & protocol
						portKey := fmt.Sprintf(
							"%s-%d-%s",
							to.IP,
							serviceExpose.As,
							serviceExpose.Proto,
						)
						otherServiceName, inUse := portsUsed[portKey]
						if inUse {
							return fmt.Errorf(
								"%w: IP endpoint %q port: %d protocol: %s specified by service %q already in use by %q",
								errSDLInvalid,
								to.IP,
								serviceExpose.Port,
								serviceExpose.Proto,
								svcName,
								otherServiceName,
							)
						}
						portsUsed[portKey] = svcName
					}
				}
			}

			// validate storage's attributes and parameters
			volumes := make(map[string]v2ResourceStorage)
			for _, volume := range compute.Resources.Storage {
				// making deepcopy here as we gonna merge compute attributes and service parameters for validation below
				attr := make(v2StorageAttributes, len(volume.Attributes))

				copy(attr, volume.Attributes)

				volumes[volume.Name] = v2ResourceStorage{
					Name:       volume.Name,
					Quantity:   volume.Quantity,
					Attributes: attr,
				}
			}

			if svc.Params != nil {
				mounts := make(map[string]string)

				storageNames := make([]string, 0, len(svc.Params.Storage))
				for name := range svc.Params.Storage {
					storageNames = append(storageNames, name)
				}
				sort.Strings(storageNames)

				for _, name := range storageNames {
					params := svc.Params.Storage[name]

					if params.Volume != "" {
						if err := sdl.validateServiceVolumeParams(svcName, svcdepl, name, params, volumes); err != nil {
							return err
						}

						if otherSvc, used := volumesUsed[params.Volume]; used && otherSvc != svcName {
							return fmt.Errorf(
								"%w: volume %q is referenced by services %q and %q; a volume attaches to a single service (RWO)",
								errSDLInvalid,
								params.Volume,
								otherSvc,
								svcName,
							)
						}

						volumesUsed[params.Volume] = svcName
					} else if _, exists := volumes[name]; !exists {
						return fmt.Errorf(
							"%w: service \"%s\" references to no-existing compute volume named \"%s\"",
							errSDLInvalid,
							svcName,
							name,
						)
					}

					if !path.IsAbs(params.Mount) {
						return fmt.Errorf(
							"%w: invalid value for \"service.%s.params.%s.mount\" parameter. expected absolute path",
							errSDLInvalid,
							svcName,
							name,
						)
					}

					if vlname, exists := mounts[params.Mount]; exists {
						if params.Mount == "" {
							return errStorageMultipleRootEphemeral
						}

						return fmt.Errorf(
							"%w: mount %q already in use by volume %q",
							errStorageDupMountPoint,
							params.Mount,
							vlname,
						)
					}

					mounts[params.Mount] = name

					if params.Volume != "" {
						continue
					}

					volume := volumes[name]

					attr := make(map[string]string)
					attr[StorageAttributeMount] = params.Mount
					attr[StorageAttributeReadOnly] = strconv.FormatBool(params.ReadOnly)

					for _, nd := range types.Attributes(volume.Attributes) {
						attr[nd.Key] = nd.Value
					}

					persistent, _ := strconv.ParseBool(attr[StorageAttributePersistent])
					class := attr[StorageAttributeClass]

					if persistent && params.Mount == "" {
						return fmt.Errorf(
							"%w: compute.storage.%s has persistent=true which requires service.%s.params.storage.%s to have mount",
							errSDLInvalid,
							name,
							svcName,
							name,
						)
					}

					if class == StorageClassRAM && params.ReadOnly {
						return fmt.Errorf(
							"%w: services.%s.params.storage.%s has readOnly=true which is not allowed for storage class \"%s\"",
							errSDLInvalid,
							svcName,
							name,
							class,
						)
					}
				}
			}
		}
	}

	for endpointName := range sdl.Endpoints {
		_, inUse := endpointsUsed[endpointName]
		if !inUse {
			return fmt.Errorf(
				"%w: endpoint %q declared but never used",
				errSDLInvalid,
				endpointName,
			)
		}
	}

	// AEP-17 endpoints pattern: declared-but-unreferenced volumes error.
	volumeNames := make([]string, 0, len(sdl.VolumesCfg))
	for name := range sdl.VolumesCfg {
		volumeNames = append(volumeNames, name)
	}
	sort.Strings(volumeNames)

	for _, name := range volumeNames {
		if _, inUse := volumesUsed[name]; !inUse {
			return fmt.Errorf(
				"%w: volume %q declared but never used",
				errSDLInvalid,
				name,
			)
		}
	}

	// v2.2 inherits the full GPU interconnect SDL grammar from v2.1, so
	// the cross-field validation rules must also apply.
	if err := validateInterconnect(sdl.Profiles, sdl.Deployments); err != nil {
		return err
	}

	return nil
}

// validateVolumeDeployment validates the deployment-section entry consuming
// a declared volume: one placement, profile named after the volume, priced,
// count 1.
func (sdl *v2_2) validateVolumeDeployment(volName string, depl v2Deployment) error {
	if len(depl) != 1 {
		return fmt.Errorf(
			"%w: volume %q must be deployed to exactly one placement (%d given)",
			errSDLInvalid,
			volName,
			len(depl),
		)
	}

	if _, exists := sdl.Profiles.Compute[volName]; exists {
		return fmt.Errorf(
			"%w: volume %q collides with a compute profile of the same name",
			errSDLInvalid,
			volName,
		)
	}

	for _, placementName := range v2DeploymentPlacementNames(depl) {
		svcdepl := depl[placementName]

		if svcdepl.Profile != volName {
			return fmt.Errorf(
				"%w: %v.%v: volume deployment profile must be the volume name %q, got %q",
				errSDLInvalid,
				volName,
				placementName,
				volName,
				svcdepl.Profile,
			)
		}

		infra, ok := sdl.Profiles.Placement[placementName]
		if !ok {
			return fmt.Errorf(
				"%w: %v.%v: no placement profile named %v",
				errSDLInvalid,
				volName,
				placementName,
				placementName,
			)
		}

		if _, ok := infra.Pricing[svcdepl.Profile]; !ok {
			return fmt.Errorf(
				"%w: %v.%v: no pricing for profile %v",
				errSDLInvalid,
				volName,
				placementName,
				svcdepl.Profile,
			)
		}

		if svcdepl.Count != 1 {
			return fmt.Errorf(
				"%w: %v.%v: volume deployment count must be 1, got %d",
				errSDLInvalid,
				volName,
				placementName,
				svcdepl.Count,
			)
		}
	}

	// adopt/replica-of compile to owner-keyed refs; fail early when the
	// owner cannot be resolved.
	vol := sdl.VolumesCfg[volName]
	if vol.Adopt != nil {
		if _, err := sdl.policyRefOwner(volName, "adopt"); err != nil {
			return err
		}
	}
	if vol.ReplicaOf != nil {
		if _, err := sdl.policyRefOwner(volName, "replica-of"); err != nil {
			return err
		}
	}

	return nil
}

// validateServiceVolumeParams validates one service storage param that
// references an external volume: the reference resolves, the grammar
// exclusions hold (no matching profile storage entry, i.e. no
// size/class/attributes on the service side), a mount is present, and the
// RWO contract keeps the replica count at 1.
func (sdl *v2_2) validateServiceVolumeParams(
	svcName string,
	svcdepl v2ServiceDeployment,
	paramName string,
	params v2ServiceStorageParams,
	profileVolumes map[string]v2ResourceStorage,
) error {
	vol, exists := sdl.VolumesCfg[params.Volume]
	if !exists {
		return fmt.Errorf(
			"%w: service %q references to non-existing volume named %q",
			errSDLInvalid,
			svcName,
			params.Volume,
		)
	}

	if !vol.isExternal() {
		return fmt.Errorf(
			"%w: service %q references volume %q which is declared in this SDL; a volume is its own deployment — create it first and reference it via volumes.%s.external",
			errSDLInvalid,
			svcName,
			params.Volume,
			params.Volume,
		)
	}

	if _, exists := profileVolumes[paramName]; exists {
		return fmt.Errorf(
			"%w: service %q storage param %q sets volume and matches a compute profile storage entry; a volume reference excludes size/class/attributes",
			errSDLInvalid,
			svcName,
			paramName,
		)
	}

	if params.Mount == "" {
		return fmt.Errorf(
			"%w: service %q storage param %q references volume %q and requires a mount",
			errSDLInvalid,
			svcName,
			paramName,
			params.Volume,
		)
	}

	if svcdepl.Count != 1 {
		return fmt.Errorf(
			"%w: service %q references volume %q with count %d; volumes attach once (RWO), count must be 1",
			errSDLInvalid,
			svcName,
			params.Volume,
			svcdepl.Count,
		)
	}

	if _, err := sdl.externalVolumeOwner(params.Volume); err != nil {
		return err
	}

	return nil
}

func (sdl *v2_2) computeEndpointSequenceNumbers() map[string]uint32 {
	var endpointNames []string
	res := make(map[string]uint32)

	for _, serviceName := range sdl.Deployments.svcNames() {
		for _, expose := range sdl.Services[serviceName].Expose {
			for _, to := range expose.To {
				if to.Global && len(to.IP) == 0 {
					continue
				}

				endpointNames = append(endpointNames, to.IP)
			}
		}
	}

	if len(endpointNames) == 0 {
		return res
	}

	// Make the assignment stable
	sort.Strings(endpointNames)

	// Start at zero, so the first assigned one is 1
	endpointSeqNumber := uint32(0)
	for _, name := range endpointNames {
		endpointSeqNumber++
		seqNo := endpointSeqNumber
		res[name] = seqNo
	}

	return res
}
