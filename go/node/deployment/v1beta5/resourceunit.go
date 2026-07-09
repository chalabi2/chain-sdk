package v1beta5

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	v1 "pkg.akt.dev/go/node/deployment/v1"
	types "pkg.akt.dev/go/node/types/resources/v1beta4"
)

// Storage attribute vocabulary shared with the SDL compiler. Volume groups
// carry lifecycle data in the typed VolumePolicy only; these two attributes
// remain the class/persistence contract on the Storage entry itself.
const (
	storageAttributePersistent = "persistent"
	storageAttributeClass      = "class"
	storageClassRAM            = "ram"
)

// FullPrice method returns full price of resource
func (r *ResourceUnit) FullPrice() sdk.DecCoin {
	return sdk.NewDecCoinFromDec(r.Price.Denom, r.Price.Amount.MulInt64(int64(r.Count)))
}

func (r *ResourceUnit) Dup() ResourceUnit {
	res := ResourceUnit{
		Resources: r.Resources.Dup(),
		Count:     r.Count,
		Price:     r.GetPrice(),
	}

	if len(r.Volumes) > 0 {
		res.Volumes = make([]v1.VolumeRef, 0, len(r.Volumes))
		res.Volumes = append(res.Volumes, r.Volumes...)
	}

	return res
}

func (r *ResourceUnit) validate() error {
	// if r.Count > uint32(validationConfig.MaxUnitCount) || r.Count < uint32(validationConfig.MinUnitCount) {
	// 	return fmt.Errorf("error: invalid unit count (%v > %v > %v fails)",
	// 		validationConfig.MaxUnitCount, r.Count, validationConfig.MinUnitCount)
	// }

	units := r.Resources

	// A unit mounting externally-leased volumes may declare no local storage
	// at all — attached volumes contribute zero storage quantity, and the
	// proto wire round-trips an empty repeated field to nil.
	if units.Storage == nil && len(r.Volumes) > 0 {
		units.Storage = make(types.Volumes, 0)
	}

	if err := validateResources(units); err != nil {
		return err
	}

	if err := r.validateVolumeRefs(); err != nil {
		return err
	}

	return nil
}

// validateVolumeRefs performs stateless validation of the unit's attach
// references (compute groups; volume groups reject them outright in
// validateVolumeUnit). Attached volumes contribute zero storage quantity to
// the compute group, so no Storage entry may correspond to a reference —
// the size/class contract is the volume group's, validated at bid time.
func (r *ResourceUnit) validateVolumeRefs() error {
	if len(r.Volumes) == 0 {
		return nil
	}

	seen := make(map[string]bool, len(r.Volumes))

	for i := range r.Volumes {
		ref := r.Volumes[i]

		if err := ref.Validate(); err != nil {
			return err
		}

		if ref.GSeq != 1 {
			return fmt.Errorf("error: invalid volume reference %q (gseq must be 1)", ref.String())
		}

		key := ref.String()
		if seen[key] {
			return fmt.Errorf("error: duplicate volume reference %q", key)
		}

		seen[key] = true
	}

	for i := range r.Storage {
		for j := range r.Volumes {
			if r.Storage[i].Name == r.Volumes[j].Name {
				return fmt.Errorf("error: storage entry %q shadows volume reference %q; attached volumes must not declare storage",
					r.Storage[i].Name, r.Volumes[j].String())
			}
		}
	}

	return nil
}

// validateVolumeUnit validates the single ResourceUnit of a storage-only
// (volume) group.
func (r *ResourceUnit) validateVolumeUnit() error {
	if r.Count != 1 {
		return fmt.Errorf("error: invalid volume group unit count (%v != 1 fails)", r.Count)
	}

	if len(r.Volumes) != 0 {
		return fmt.Errorf("error: volume group must not carry volume references")
	}

	if err := validateVolumeResources(r.Resources); err != nil {
		return err
	}

	return nil
}

// validateVolumeResources enforces the storage-only shape of a volume group's
// resources: compute legs present-but-zero (fields non-nil so every downstream
// dereference stays crash-safe, values zero so the group is storage-only) and
// exactly one persistent, non-ram Storage entry.
func validateVolumeResources(units types.Resources) error {
	if units.ID == 0 {
		return fmt.Errorf("error: invalid resources ID (> 0 fails)")
	}

	if units.CPU == nil {
		return fmt.Errorf("error: invalid unit CPU, cannot be nil")
	}

	if units.CPU.Units.Value() != 0 {
		return fmt.Errorf("error: invalid volume group CPU (%v != 0 fails)", units.CPU.Units.Value())
	}

	if units.Memory == nil {
		return fmt.Errorf("error: invalid unit memory, cannot be nil")
	}

	if units.Memory.Quantity.Value() != 0 {
		return fmt.Errorf("error: invalid volume group memory (%v != 0 fails)", units.Memory.Quantity.Value())
	}

	if units.GPU == nil {
		return fmt.Errorf("error: invalid unit GPU, cannot be nil")
	}

	if units.GPU.Units.Value() != 0 {
		return fmt.Errorf("error: invalid volume group GPU (%v != 0 fails)", units.GPU.Units.Value())
	}

	if len(units.Endpoints) != 0 {
		return fmt.Errorf("error: volume group must not declare endpoints")
	}

	if len(units.Storage) != 1 {
		return fmt.Errorf("error: invalid volume group storage (exactly one entry required, %v given)", len(units.Storage))
	}

	vol := units.Storage[0]

	if vol.Name == "" {
		return fmt.Errorf("error: volume group storage entry must be named")
	}

	if persistent, valid := vol.Attributes.Find(storageAttributePersistent).AsBool(); !valid || !persistent {
		return fmt.Errorf("error: volume group storage must set attribute %s=true", storageAttributePersistent)
	}

	class, valid := vol.Attributes.Find(storageAttributeClass).AsString()
	if !valid {
		return fmt.Errorf("error: volume group storage must set attribute %s", storageAttributeClass)
	}

	if class == storageClassRAM {
		return fmt.Errorf("error: volume group storage class cannot be %s", storageClassRAM)
	}

	// [5Mi, hard network cap]; the governance-bounded MaxVolumeSize ceiling
	// is enforced against module params via GroupSpec.ValidateVolumeBounds.
	if err := validateStorage(units.Storage); err != nil {
		return err
	}

	return nil
}

func (r *ResourceUnit) totalResources() resourceLimits {
	limits := newLimits()

	limits.cpu = limits.cpu.Add(r.CPU.Units.Val)
	limits.gpu = limits.gpu.Add(r.GPU.Units.Val)
	limits.memory = limits.memory.Add(r.Memory.Quantity.Val)

	storage := make([]sdkmath.Int, 0, len(r.Storage))

	for _, vol := range r.Storage {
		storage = append(storage, vol.Quantity.Val)
	}

	// fixme this is not actually sum for storage usecase.
	// do we really need sum here?
	limits.storage = storage

	limits.mul(r.Count)

	return limits
}

func (r *ResourceUnit) validatePricing() error {
	if !r.GetPrice().IsValid() {
		return fmt.Errorf("error: invalid price object")
	}

	if r.Price.Amount.GT(sdkmath.LegacyNewDecFromInt(sdkmath.NewIntFromUint64(validationConfig.Unit.Max.Price))) {
		return fmt.Errorf("error: invalid unit price (%v > %v fails)", validationConfig.Unit.Max.Price, r.Price)
	}

	return nil
}

func validateResources(units types.Resources) error {
	if units.ID == 0 {
		return fmt.Errorf("error: invalid resources ID (> 0 fails)")
	}

	if err := validateCPU(units.CPU); err != nil {
		return err
	}

	if err := validateGPU(units.GPU); err != nil {
		return err
	}

	if err := validateMemory(units.Memory); err != nil {
		return err
	}

	if err := validateStorage(units.Storage); err != nil {
		return err
	}

	return nil
}

func validateCPU(u *types.CPU) error {
	if u == nil {
		return fmt.Errorf("error: invalid unit CPU, cannot be nil")
	}

	if (u.Units.Value() > uint64(validationConfig.Unit.Max.CPU)) || (u.Units.Value() < uint64(validationConfig.Unit.Min.CPU)) {
		return fmt.Errorf("error: invalid unit CPU (%v > %v > %v fails)",
			validationConfig.Unit.Max.CPU, u.Units.Value(), validationConfig.Unit.Max.CPU)
	}

	if err := u.Attributes.Validate(); err != nil {
		return fmt.Errorf("error: invalid CPU attributes: %w", err)
	}

	return nil
}

func validateGPU(u *types.GPU) error {
	if u == nil {
		return fmt.Errorf("error: invalid unit GPU, cannot be nil")
	}

	if (u.Units.Value() > uint64(validationConfig.Unit.Max.GPU)) || (u.Units.Value() < uint64(validationConfig.Unit.Min.GPU)) {
		return fmt.Errorf("error: invalid unit GPU (%v > %v > %v fails)",
			validationConfig.Unit.Max.GPU, u.Units.Value(), validationConfig.Unit.Max.GPU)
	}

	if u.Units.Value() == 0 && len(u.Attributes) > 0 {
		return fmt.Errorf("error: invalid GPU state. attributes cannot be present if units == 0")
	}

	if err := u.Attributes.Validate(); err != nil {
		return fmt.Errorf("error: invalid GPU attributes: %w", err)
	}

	return nil
}

func validateMemory(u *types.Memory) error {
	if u == nil {
		return fmt.Errorf("error: invalid unit memory, cannot be nil")
	}
	if (u.Quantity.Value() > validationConfig.Unit.Max.Memory) || (u.Quantity.Value() < validationConfig.Unit.Min.Memory) {
		return fmt.Errorf("error: invalid unit memory (%v > %v > %v fails)",
			validationConfig.Unit.Max.Memory, u.Quantity.Value(), validationConfig.Unit.Max.Memory)
	}

	if err := u.Attributes.Validate(); err != nil {
		return fmt.Errorf("error: invalid Memory attributes: %w", err)
	}

	return nil
}

func validateStorage(u types.Volumes) error {
	if u == nil {
		return fmt.Errorf("error: invalid unit storage, cannot be nil")
	}

	for i := range u {
		if (u[i].Quantity.Value() > validationConfig.Unit.Max.Storage) || (u[i].Quantity.Value() < validationConfig.Unit.Min.Storage) {
			return fmt.Errorf("error: invalid unit storage (%v > %v > %v fails)",
				validationConfig.Unit.Max.Storage, u[i].Quantity.Value(), validationConfig.Unit.Min.Storage)
		}

		if err := u[i].Attributes.Validate(); err != nil {
			return fmt.Errorf("error: invalid Storage attributes: %w", err)
		}
	}

	return nil
}
