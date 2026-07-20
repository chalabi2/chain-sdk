package sdl

import (
	"fmt"
	"sort"
	"strings"

	manifest "pkg.akt.dev/go/manifest/v2beta4"
	dv1 "pkg.akt.dev/go/node/deployment/v1"
	dtypes "pkg.akt.dev/go/node/deployment/v1beta5"
	types "pkg.akt.dev/go/node/types/attributes/v1"
	rtypes "pkg.akt.dev/go/node/types/resources/v1beta4"
)

type groupsBuilderV2_2 struct {
	dgroup        *dtypes.GroupSpec
	mgroup        *manifest.Group
	boundComputes map[string]map[string]int
	teeType       string // tracks the TEE type projected for this group
}

// buildGroups
func (sdl *v2_2) buildGroups() error {
	endpointsNames := sdl.computeEndpointSequenceNumbers()

	groups := make(map[string]*groupsBuilderV2_2)

	for _, svcName := range sdl.Deployments.svcNames() {
		depl := sdl.Deployments[svcName]

		// A volume deployment entry compiles to a storage-only group: the
		// typed VolumePolicy plus present-but-zero compute resources and a
		// single Storage entry. Its manifest group is the canonical empty
		// group (on-chain group name, no services).
		if vol, isVolume := sdl.VolumesCfg[svcName]; isVolume && !vol.isExternal() {
			for _, placementName := range depl.placementNames() {
				group, err := sdl.buildVolumeGroup(svcName, placementName, depl[placementName])
				if err != nil {
					return err
				}

				groups[placementName] = group
			}

			continue
		}

		for _, placementName := range depl.placementNames() {
			// objects below have been ensured to exist
			svcdepl := depl[placementName]
			compute := sdl.Profiles.Compute[svcdepl.Profile]
			svc := sdl.Services[svcName]
			infra := sdl.Profiles.Placement[placementName]
			price := infra.Pricing[svcdepl.Profile]

			group := groups[placementName]

			if group == nil {
				group = &groupsBuilderV2_2{
					dgroup: &dtypes.GroupSpec{
						Name: placementName,
					},
					mgroup: &manifest.Group{
						Name: placementName,
					},
					boundComputes: make(map[string]map[string]int),
				}

				group.dgroup.Requirements.Attributes = types.Attributes(infra.Attributes)
				group.dgroup.Requirements.SignedBy = infra.SignedBy

				// keep ordering stable
				sort.Sort(group.dgroup.Requirements.Attributes)

				groups[placementName] = group
			}

			// Validate TEE + GPU consistency
			if svc.Params != nil && svc.Params.TEE != "" {
				teeType, err := parseTEEParam(svc.Params.TEE)
				if err != nil {
					return err
				}
				hasGPU := compute.Resources.GPU != nil && compute.Resources.GPU.Units > 0
				if err := validateTEEWithGPU(teeType, hasGPU); err != nil {
					return err
				}

				// Project TEE type as a placement requirement attribute so the bid
				// engine matches only providers that support confidential compute.
				if group.teeType != "" && group.teeType != teeType {
					return fmt.Errorf("%w: group %q has %q and %q",
						errTEETypeMismatch, placementName, group.teeType, teeType)
				}
				if group.teeType == "" {
					group.teeType = teeType
					group.dgroup.Requirements.Attributes = append(
						group.dgroup.Requirements.Attributes,
						types.Attribute{Key: "tee/type", Value: teeType},
					)
					sort.Sort(group.dgroup.Requirements.Attributes)
				}
			}

			if _, exists := group.boundComputes[placementName]; !exists {
				group.boundComputes[placementName] = make(map[string]int)
			}

			expose, err := sdl.Services[svcName].Expose.toManifestExpose(endpointsNames)
			if err != nil {
				return err
			}

			// externally-leased volumes this service mounts; they ride the
			// ResourceUnit so the refs flow into the order with zero extra
			// market surface
			volumeRefs, err := sdl.serviceVolumeRefs(svcName)
			if err != nil {
				return err
			}

			resources := compute.Resources.toResources()
			resources.Endpoints = expose.GetEndpoints()

			// an attach-only service may declare no local storage at all
			// (the attached volume contributes zero storage quantity);
			// normalize to present-but-empty so validation stays crash-safe
			if resources.Storage == nil && len(volumeRefs) > 0 {
				resources.Storage = rtypes.Volumes{}
			}

			if location, bound := group.boundComputes[placementName][svcdepl.Profile]; !bound {
				res := compute.Resources.toResources()
				res.Endpoints = expose.GetEndpoints()

				if res.Storage == nil && len(volumeRefs) > 0 {
					res.Storage = rtypes.Volumes{}
				}

				var resID uint32
				if ln := uint32(len(group.dgroup.Resources)); ln > 0 { // nolint: gosec
					resID = ln + 1
				} else {
					resID = 1
				}

				res.ID = resID
				resources.ID = res.ID

				group.dgroup.Resources = append(group.dgroup.Resources, dtypes.ResourceUnit{
					Resources: res,
					Price:     price.Value,
					Count:     svcdepl.Count,
					Volumes:   volumeRefs,
				})

				group.boundComputes[placementName][svcdepl.Profile] = len(group.dgroup.Resources) - 1
			} else {
				// Services sharing one compute profile share a ResourceUnit,
				// and the manifest<->group-spec handshake requires every
				// service of a unit to carry exactly the unit's volume refs.
				if !volumeRefsEqual(group.dgroup.Resources[location].Volumes, volumeRefs) {
					return fmt.Errorf(
						"%w: service %q shares compute profile %q but not its volume references; give volume-mounting services a dedicated profile",
						errSDLInvalid,
						svcName,
						svcdepl.Profile,
					)
				}

				resources.ID = group.dgroup.Resources[location].ID

				group.dgroup.Resources[location].Count += svcdepl.Count
				group.dgroup.Resources[location].Endpoints = append(group.dgroup.Resources[location].Endpoints, expose.GetEndpoints()...)

				sort.Sort(group.dgroup.Resources[location].Endpoints)
			}

			msvc := manifest.Service{
				Name:      svcName,
				Image:     svc.Image,
				Args:      svc.Args,
				Env:       svc.Env,
				Resources: resources,
				Count:     svcdepl.Count,
				Command:   svc.Command,
				Expose:    expose,
			}

			// See groupBuilder_v2.go for rationale.
			if compute.Resources != nil && compute.Resources.GPU != nil {
				msvc.InterconnectGroup = compute.Resources.GPU.interconnectGroup
			}

			if svc.Params != nil {
				params := &manifest.ServiceParams{}

				if len(svc.Params.Storage) > 0 {
					params.Storage = make([]manifest.StorageParams, 0, len(svc.Params.Storage))
					storageNames := make([]string, 0, len(svc.Params.Storage))
					for volName := range svc.Params.Storage {
						storageNames = append(storageNames, volName)
					}
					sort.Strings(storageNames)
					for _, volName := range storageNames {
						volParams := svc.Params.Storage[volName]

						sparams := manifest.StorageParams{
							Name:     volName,
							Mount:    volParams.Mount,
							ReadOnly: volParams.ReadOnly,
						}

						// mirror the on-chain VolumeRef as
						// "owner/dseq/gseq/name" — identical content on both
						// sides is what checkAgainstGSpec verifies
						if volParams.Volume != "" {
							ref, err := sdl.volumeRef(volParams.Volume)
							if err != nil {
								return err
							}

							sparams.Volume = ref.String()
						}

						params.Storage = append(params.Storage, sparams)
					}
				}

				if svc.Params.Permissions != nil {
					params.Permissions = &manifest.ServicePermissions{
						Read: svc.Params.Permissions.Read,
					}
				}

				if svc.Params.TEE != "" {
					params.TEE = &manifest.TEEParams{
						Type:        svc.Params.TEE,
						Attestation: true,
					}
				}

				msvc.Params = params
			}

			if svc.Credentials != nil {
				msvc.Credentials = &manifest.ImageCredentials{
					Host:     strings.TrimSpace(svc.Credentials.Host),
					Email:    strings.TrimSpace(svc.Credentials.Email),
					Username: strings.TrimSpace(svc.Credentials.Username),
					Password: strings.TrimSpace(svc.Credentials.Password),
				}
			}

			group.mgroup.Services = append(group.mgroup.Services, msvc)
		}
	}

	// keep ordering stable
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	sdl.result.dgroups = make(dtypes.GroupSpecs, 0, len(names))
	sdl.result.mgroups = make(manifest.Groups, 0, len(names))

	for _, name := range names {
		mgroup := *groups[name].mgroup
		// stable ordering services by name
		sort.Sort(mgroup.Services)

		sdl.result.dgroups = append(sdl.result.dgroups, *groups[name].dgroup)
		sdl.result.mgroups = append(sdl.result.mgroups, mgroup)
	}

	return nil
}

// buildVolumeGroup compiles a declared volume into its storage-only
// GroupSpec: VolumePolicy carries the lifecycle contract (never
// Storage.Attributes — attributes flow into provider matching and any
// tenant-unique value there makes the order unbiddable), compute legs are
// present-but-zero, and capabilities/storage/volumes is injected into the
// placement requirements so only opted-in providers match. The paired
// manifest group is the canonical empty group.
func (sdl *v2_2) buildVolumeGroup(volName string, placementName string, svcdepl v2ServiceDeployment) (*groupsBuilderV2_2, error) {
	vol := sdl.VolumesCfg[volName]
	infra := sdl.Profiles.Placement[placementName]
	price := infra.Pricing[svcdepl.Profile]

	reclaim, err := vol.reclaim()
	if err != nil {
		return nil, err
	}

	retention, err := vol.retention()
	if err != nil {
		return nil, err
	}

	policy := &dv1.VolumePolicy{
		Vid:            volName,
		Reclaim:        reclaim,
		Retention:      retention,
		MaxAttachments: 1, // v1 volumes are RWO; RWX is additive later
		MaxReplicas:    vol.MaxReplicas,
	}

	if vol.Adopt != nil {
		ref, err := sdl.volumePolicyRef(volName, "adopt", vol.Adopt)
		if err != nil {
			return nil, err
		}

		policy.Adopt = ref
	}

	if vol.ReplicaOf != nil {
		ref, err := sdl.volumePolicyRef(volName, "replica-of", vol.ReplicaOf)
		if err != nil {
			return nil, err
		}

		policy.ReplicaOf = ref
	}

	class := vol.Class
	if class == "" {
		class = StorageClassDefault
	}

	dgroup := &dtypes.GroupSpec{
		Name:   placementName,
		Volume: policy,
		Resources: dtypes.ResourceUnits{
			{
				Resources: rtypes.Resources{
					ID: 1,
					CPU: &rtypes.CPU{
						Units: rtypes.NewResourceValue(0),
					},
					Memory: &rtypes.Memory{
						Quantity: rtypes.NewResourceValue(0),
					},
					GPU: &rtypes.GPU{
						Units: rtypes.NewResourceValue(0),
					},
					Storage: rtypes.Volumes{
						{
							Name:     volName,
							Quantity: rtypes.NewResourceValue(uint64(vol.Size)),
							Attributes: types.Attributes{
								{Key: StorageAttributeClass, Value: class},
								{Key: StorageAttributePersistent, Value: "true"},
							},
						},
					},
					Endpoints: rtypes.Endpoints{},
				},
				Count: 1,
				Price: price.Value,
			},
		},
	}

	attrs := make(types.Attributes, 0, len(infra.Attributes)+1)
	attrs = append(attrs, types.Attributes(infra.Attributes)...)
	attrs = append(attrs, types.Attribute{Key: StorageCapabilityVolumes, Value: "true"})

	// keep ordering stable
	sort.Sort(attrs)

	dgroup.Requirements.Attributes = attrs
	dgroup.Requirements.SignedBy = infra.SignedBy

	return &groupsBuilderV2_2{
		dgroup: dgroup,
		mgroup: &manifest.Group{
			Name: placementName,
		},
	}, nil
}

// volumeRef resolves a service-side volume reference to the on-chain
// VolumeRef it compiles to. gseq is always 1: a volume deployment holds
// exactly one group.
func (sdl *v2_2) volumeRef(volName string) (dv1.VolumeRef, error) {
	owner, err := sdl.externalVolumeOwner(volName)
	if err != nil {
		return dv1.VolumeRef{}, err
	}

	return dv1.VolumeRef{
		Owner: owner,
		DSeq:  sdl.VolumesCfg[volName].External.DSeq,
		GSeq:  1,
		Name:  volName,
	}, nil
}

// volumePolicyRef resolves an adopt/replica-of reference: the owner is
// always the signer, and the name defaults to the declaring volume's own
// name (same vid).
func (sdl *v2_2) volumePolicyRef(volName string, field string, ref *v2_2VolumePolicyRef) (*dv1.VolumeRef, error) {
	owner, err := sdl.policyRefOwner(volName, field)
	if err != nil {
		return nil, err
	}

	name := ref.Name
	if name == "" {
		name = volName
	}

	return &dv1.VolumeRef{
		Owner: owner,
		DSeq:  ref.DSeq,
		GSeq:  1,
		Name:  name,
	}, nil
}

// serviceVolumeRefs collects the volume references of a service's storage
// params in canonical order (sorted by the "owner/dseq/gseq/name" form —
// the same order checkAgainstGSpec compares against).
func (sdl *v2_2) serviceVolumeRefs(svcName string) ([]dv1.VolumeRef, error) {
	svc := sdl.Services[svcName]

	if svc.Params == nil || len(svc.Params.Storage) == 0 {
		return nil, nil
	}

	var refs []dv1.VolumeRef

	for _, params := range svc.Params.Storage {
		if params.Volume == "" {
			continue
		}

		ref, err := sdl.volumeRef(params.Volume)
		if err != nil {
			return nil, err
		}

		refs = append(refs, ref)
	}

	sort.Slice(refs, func(i, j int) bool {
		return refs[i].String() < refs[j].String()
	})

	return refs, nil
}

func volumeRefsEqual(a []dv1.VolumeRef, b []dv1.VolumeRef) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !a[i].Equals(b[i]) {
			return false
		}
	}

	return true
}
