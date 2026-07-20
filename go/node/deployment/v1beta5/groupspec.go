package v1beta5

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	atypes "pkg.akt.dev/go/node/audit/v1"
	v1 "pkg.akt.dev/go/node/deployment/v1"
	attr "pkg.akt.dev/go/node/types/attributes/v1"
)

type ResourceGroup interface {
	GetName() string
	GetResourceUnits() ResourceUnits
}

var _ ResourceGroup = (*GroupSpec)(nil)

type GroupSpecs []GroupSpec

func (gspecs GroupSpecs) Dup() GroupSpecs {
	res := make(GroupSpecs, 0, len(gspecs))

	for _, gspec := range gspecs {
		gs := gspec.Dup()
		res = append(res, gs)
	}
	return res
}

func (g GroupSpec) Dup() GroupSpec {
	res := GroupSpec{
		Name:         g.Name,
		Requirements: g.Requirements.Dup(),
		Resources:    g.Resources.Dup(),
		Volume:       g.Volume.Dup(),
	}

	return res
}

// ValidateBasic asserts non-zero values
func (g GroupSpec) ValidateBasic() error {
	return g.validate()
}

// GetResourceUnits method returns resources list in group
func (g GroupSpec) GetResourceUnits() ResourceUnits {
	resources := make(ResourceUnits, 0, len(g.Resources))

	resources = append(resources, g.Resources...)

	return resources
}

// GetName method returns group name
func (g GroupSpec) GetName() string {
	return g.Name
}

// Price method returns price of group
func (g GroupSpec) Price() sdk.DecCoin {
	var price sdk.DecCoin
	for idx, resource := range g.Resources {
		if idx == 0 {
			price = resource.FullPrice()
			continue
		}
		price = price.Add(resource.FullPrice())
	}
	return price
}

// MatchResourcesRequirements check if resources attributes match provider's capabilities
func (g GroupSpec) MatchResourcesRequirements(pattr attr.Attributes) bool {
	for _, rgroup := range g.GetResourceUnits() {
		pgroup := pattr.GetCapabilitiesGroup("storage")
		for _, storage := range rgroup.Storage {
			if len(storage.Attributes) == 0 {
				continue
			}

			if !storage.Attributes.IN(pgroup) {
				return false
			}
		}
		if gpu := rgroup.GPU; gpu.Units.Val.Uint64() > 0 {
			attr := gpu.Attributes
			if len(attr) == 0 {
				continue
			}

			pgroup = pattr.GetCapabilitiesMap("gpu")

			if !gpu.Attributes.AnyIN(pgroup) {
				return false
			}
		}
	}

	return true
}

// MatchRequirements method compares provided attributes with specific group attributes.
// Argument provider is a bit cumbersome. First element is attributes from x/provider store
// in case tenant does not need signed attributes at all
// rest of elements (if any) are attributes signed by various auditors
func (g GroupSpec) MatchRequirements(provider []atypes.AuditedProvider) bool {
	if (len(g.Requirements.SignedBy.AnyOf) != 0) || (len(g.Requirements.SignedBy.AllOf) != 0) {
		// we cannot match if there is no signed attributes
		if len(provider) < 2 {
			return false
		}

		existingRequirements := make(attributesMatching)

		for _, existing := range provider[1:] {
			existingRequirements[existing.Auditor] = existing.Attributes
		}

		if len(g.Requirements.SignedBy.AllOf) != 0 {
			for _, validator := range g.Requirements.SignedBy.AllOf {
				// if at least one signature does not exist or no match on attributes - requirements cannot match
				if existingAttr, exists := existingRequirements[validator]; !exists ||
					!attr.AttributesSubsetOf(g.Requirements.Attributes, existingAttr) {
					return false
				}
			}
		}

		if len(g.Requirements.SignedBy.AnyOf) != 0 {
			for _, validator := range g.Requirements.SignedBy.AnyOf {
				if existingAttr, exists := existingRequirements[validator]; exists &&
					attr.AttributesSubsetOf(g.Requirements.Attributes, existingAttr) {
					return true
				}
			}

			return false
		}

		return true
	}

	return attr.AttributesSubsetOf(g.Requirements.Attributes, provider[0].Attributes)
}

// validate does validation for provided deployment group
func (g *GroupSpec) validate() error {
	if g.Name == "" {
		return fmt.Errorf("empty group spec name denomination")
	}

	if g.Volume != nil {
		if err := g.validateVolumeGroup(); err != nil {
			return err
		}
	} else if err := g.GetResourceUnits().Validate(); err != nil {
		return err
	}

	if err := g.validatePricing(); err != nil {
		return err
	}

	return nil
}

// validateVolumeGroup does stateless validation of a storage-only (volume)
// group: the VolumePolicy fields plus the present-but-zero resource shape.
// The compute-group totals in ResourceUnits.Validate do not apply — a volume
// group requests zero CPU/memory/GPU by construction.
func (g *GroupSpec) validateVolumeGroup() error {
	vol := g.Volume

	if err := v1.ValidateVID(vol.Vid); err != nil {
		return err
	}

	switch vol.Reclaim {
	case v1.VolumeReclaimRetain, v1.VolumeReclaimDelete:
	default:
		return fmt.Errorf("error: invalid volume reclaim policy (%v)", vol.Reclaim)
	}

	if vol.Retention < 0 {
		return fmt.Errorf("error: invalid volume retention (%v < 0 fails)", vol.Retention)
	}

	if vol.MaxAttachments != 1 {
		return fmt.Errorf("error: invalid volume max_attachments (%v != 1 fails)", vol.MaxAttachments)
	}

	if vol.Adopt != nil && vol.ReplicaOf != nil {
		return fmt.Errorf("error: volume adopt and replica_of are mutually exclusive")
	}

	if err := validateVolumePolicyRef("adopt", vol.Adopt); err != nil {
		return err
	}

	if err := validateVolumePolicyRef("replica_of", vol.ReplicaOf); err != nil {
		return err
	}

	if len(g.Resources) != 1 {
		return fmt.Errorf("error: invalid volume group resources (exactly one unit required, %v given)", len(g.Resources))
	}

	if err := g.Resources[0].validateVolumeUnit(); err != nil {
		return err
	}

	return nil
}

func validateVolumePolicyRef(field string, ref *v1.VolumeRef) error {
	if ref == nil {
		return nil
	}

	if err := ref.Validate(); err != nil {
		return err
	}

	if ref.GSeq != 1 {
		return fmt.Errorf("error: invalid volume %s reference %q (gseq must be 1)", field, ref.String())
	}

	return nil
}

// ValidateVolumeBounds enforces the param-bounded volume group rules against
// the deployment module params: storage size <= MaxVolumeSize, retention <=
// MaxVolumeRetention, max_replicas <= MaxVolumeReplicas. ValidateBasic covers
// the stateless shape; the deployment msg server calls this with current
// chain state. A nil-Volume (compute) group always passes.
func (g GroupSpec) ValidateVolumeBounds(p Params) error {
	if g.Volume == nil {
		return nil
	}

	for i := range g.Resources {
		for _, storage := range g.Resources[i].Storage {
			if storage.Quantity.Value() > p.MaxVolumeSize {
				return fmt.Errorf("%w: volume size too large (%v > %v fails)",
					v1.ErrInvalidParam, storage.Quantity.Value(), p.MaxVolumeSize)
			}
		}
	}

	if g.Volume.Retention > p.MaxVolumeRetention {
		return fmt.Errorf("%w: volume retention too long (%v > %v fails)",
			v1.ErrInvalidParam, g.Volume.Retention, p.MaxVolumeRetention)
	}

	if g.Volume.MaxReplicas > p.MaxVolumeReplicas {
		return fmt.Errorf("%w: volume max_replicas too large (%v > %v fails)",
			v1.ErrInvalidParam, g.Volume.MaxReplicas, p.MaxVolumeReplicas)
	}

	return nil
}

func (g *GroupSpec) validatePricing() error {
	var price sdk.DecCoin

	mem := sdkmath.NewInt(0)

	for idx, resource := range g.Resources {
		if err := resource.validatePricing(); err != nil {
			return fmt.Errorf("group %v: %w", g.GetName(), err)
		}

		// all must be same denomination
		if idx == 0 {
			price = resource.FullPrice()
		} else {
			rprice := resource.FullPrice()
			if rprice.Denom != price.Denom {
				return fmt.Errorf("multi-denonimation group: (%v == %v fails)", rprice.Denom, price.Denom)
			}
			price = price.Add(rprice)
		}

		memCount := sdkmath.NewInt(0)
		if u := resource.Memory; u != nil {
			memCount.Add(sdkmath.NewIntFromUint64(u.Quantity.Value()))
		}

		mem = mem.Add(memCount.Mul(sdkmath.NewIntFromUint64(uint64(resource.Count))))
	}

	return nil
}
