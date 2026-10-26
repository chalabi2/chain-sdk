package v1beta4

import (
	"fmt"
	"sort"

	v1 "pkg.akt.dev/go/node/deployment/v1"
	attr "pkg.akt.dev/go/node/types/attributes/v1"
	types "pkg.akt.dev/go/node/types/resources/v1beta4"
)

// Resize returns g with its resource units replaced by resources, after
// checking that the change is one an in-place lease resize can apply:
//
//   - the same resource unit IDs, each keeping its attributes, storage names
//     and classes, endpoints and volume references
//   - CPU, memory, ephemeral storage, count and price may change
//   - GPU units may change only on a unit that already has GPUs
//   - persistent storage keeps its size, except the single entry of a volume
//     group, which may only grow
//   - a unit that mounts volumes keeps its count
//
// Placement requirements and the volume policy are carried over unchanged.
// The result must differ from g and is validated as a whole, so every
// create-time rule still holds.
func (g GroupSpec) Resize(resources ResourceUnits) (GroupSpec, error) {
	if len(resources) != len(g.Resources) {
		return GroupSpec{}, fmt.Errorf("%w: resource unit count changes (%d != %d)", v1.ErrInvalidResize, len(resources), len(g.Resources))
	}

	byID := make(map[uint32]ResourceUnit, len(resources))
	for _, ru := range resources {
		if _, dup := byID[ru.ID]; dup {
			return GroupSpec{}, fmt.Errorf("%w: duplicate resource unit %d", v1.ErrInvalidResize, ru.ID)
		}
		byID[ru.ID] = ru
	}

	next := g.Dup()
	changed := false

	for i := range g.Resources {
		cur := g.Resources[i]

		nru, found := byID[cur.ID]
		if !found {
			return GroupSpec{}, fmt.Errorf("%w: resource unit %d missing", v1.ErrInvalidResize, cur.ID)
		}

		if err := checkUnitResize(g.Volume != nil, cur, nru); err != nil {
			return GroupSpec{}, fmt.Errorf("%w: resource unit %d: %w", v1.ErrInvalidResize, cur.ID, err)
		}

		if !cur.Equal(&nru) {
			changed = true
		}

		next.Resources[i] = nru.Dup()
	}

	if !changed {
		return GroupSpec{}, fmt.Errorf("%w: resources unchanged", v1.ErrInvalidResize)
	}

	if err := next.ValidateBasic(); err != nil {
		return GroupSpec{}, fmt.Errorf("%w: %w", v1.ErrInvalidResize, err)
	}

	return next, nil
}

func checkUnitResize(volumeGroup bool, cur, next ResourceUnit) error {
	if next.Price.Denom != cur.Price.Denom {
		return fmt.Errorf("price denom changes (%s != %s)", next.Price.Denom, cur.Price.Denom)
	}

	if len(cur.Volumes) > 0 && next.Count != cur.Count {
		return fmt.Errorf("count of a unit that mounts volumes changes (%d != %d)", next.Count, cur.Count)
	}

	if len(next.Volumes) != len(cur.Volumes) {
		return fmt.Errorf("volume references change")
	}

	for i := range cur.Volumes {
		if !cur.Volumes[i].Equal(&next.Volumes[i]) {
			return fmt.Errorf("volume reference %q changes", cur.Volumes[i].String())
		}
	}

	if err := checkComputeResize(cur.Resources, next.Resources); err != nil {
		return err
	}

	if err := checkStorageResize(volumeGroup, cur.Storage, next.Storage); err != nil {
		return err
	}

	if len(next.Endpoints) != len(cur.Endpoints) {
		return fmt.Errorf("endpoints change")
	}

	for i := range cur.Endpoints {
		if !cur.Endpoints[i].Equal(&next.Endpoints[i]) {
			return fmt.Errorf("endpoints change")
		}
	}

	return nil
}

func checkComputeResize(cur, next types.Resources) error {
	if (cur.CPU == nil) != (next.CPU == nil) || (cur.Memory == nil) != (next.Memory == nil) {
		return fmt.Errorf("compute resources must stay present")
	}

	if cur.CPU != nil && !sameAttributes(cur.CPU.Attributes, next.CPU.Attributes) {
		return fmt.Errorf("cpu attributes change")
	}

	if cur.Memory != nil && !sameAttributes(cur.Memory.Attributes, next.Memory.Attributes) {
		return fmt.Errorf("memory attributes change")
	}

	curGPU, nextGPU := gpuUnits(cur.GPU), gpuUnits(next.GPU)
	if (curGPU == 0) != (nextGPU == 0) {
		return fmt.Errorf("gpus cannot be added or removed (%d -> %d)", curGPU, nextGPU)
	}

	if curGPU > 0 && !sameAttributes(cur.GPU.Attributes, next.GPU.Attributes) {
		return fmt.Errorf("gpu attributes change")
	}

	return nil
}

func checkStorageResize(volumeGroup bool, cur, next types.Volumes) error {
	if len(next) != len(cur) {
		return fmt.Errorf("storage entries change (%d != %d)", len(next), len(cur))
	}

	for i := range cur {
		if next[i].Name != cur[i].Name {
			return fmt.Errorf("storage %q renamed to %q", cur[i].Name, next[i].Name)
		}

		if !sameAttributes(cur[i].Attributes, next[i].Attributes) {
			return fmt.Errorf("storage %q attributes change", cur[i].Name)
		}

		persistent, _ := cur[i].Attributes.Find(storageAttributePersistent).AsBool()
		if !persistent {
			continue
		}

		curSize, nextSize := cur[i].Quantity.Value(), next[i].Quantity.Value()

		switch {
		case volumeGroup && nextSize < curSize:
			return fmt.Errorf("volume %q cannot shrink (%d < %d)", cur[i].Name, nextSize, curSize)
		case !volumeGroup && nextSize != curSize:
			return fmt.Errorf("persistent storage %q keeps its size (%d != %d)", cur[i].Name, nextSize, curSize)
		}
	}

	return nil
}

func gpuUnits(gpu *types.GPU) uint64 {
	if gpu == nil {
		return 0
	}

	return gpu.Units.Value()
}

// sameAttributes compares attributes as key/value sets.
func sameAttributes(a, b attr.Attributes) bool {
	if len(a) != len(b) {
		return false
	}

	sa, sb := a.Dup(), b.Dup()
	sort.Sort(sa)
	sort.Sort(sb)

	for i := range sa {
		if sa[i].Key != sb[i].Key || sa[i].Value != sb[i].Value {
			return false
		}
	}

	return true
}
