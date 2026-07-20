package v2beta4

import (
	"fmt"

	dtypes "pkg.akt.dev/go/node/deployment/v1beta5"
)

type Groups []Group

func (groups Groups) Validate() error {
	if len(groups) == 0 {
		return fmt.Errorf("%w: manifest is empty", ErrInvalidManifest)
	}

	helper := validateManifestGroupsHelper{
		hostnames: make(map[string]int),
	}

	names := make(map[string]int) // used as a set

	serviceCount := 0

	for _, group := range groups {
		if err := group.Validate(&helper); err != nil {
			return err
		}
		if _, exists := names[group.GetName()]; exists {
			return fmt.Errorf("%w: duplicate group %q", ErrInvalidManifest, group.GetName())
		}

		names[group.GetName()] = 0 // Value stored is not used

		serviceCount += len(group.Services)
	}

	// A manifest with zero services everywhere is the canonical volume
	// manifest; it pairs only with on-chain volume groups, enforced in
	// CheckAgainstGSpecs. Any manifest that carries services keeps the
	// original requirement of at least one global service.
	if serviceCount > 0 && helper.globalServiceCount == 0 {
		return fmt.Errorf("%w: zero global services", ErrInvalidManifest)
	}

	return nil
}

func (groups Groups) CheckAgainstGSpecs(gspecs dtypes.GroupSpecs) error {
	gspecs = gspecs.Dup()

	if err := groups.Validate(); err != nil {
		return err
	}

	if len(groups) != len(gspecs) {
		return fmt.Errorf("invalid manifest: group count mismatch (%v != %v)", len(groups), len(gspecs))
	}

	dgroupByName := newGroupSpecsHelper(gspecs)

	for _, mgroup := range groups {
		dgroup, dgroupExists := dgroupByName[mgroup.GetName()]

		if !dgroupExists {
			return fmt.Errorf("invalid manifest: unknown deployment group ('%v')", mgroup.GetName())
		}

		// A manifest group has zero services iff its on-chain group is a
		// volume group: volume groups run no workloads, and every compute
		// group must be backed by services.
		if len(mgroup.Services) == 0 && dgroup.gs.Volume == nil {
			return fmt.Errorf("%w: group %q contains no services", ErrInvalidManifest, mgroup.GetName())
		}

		if len(mgroup.Services) != 0 && dgroup.gs.Volume != nil {
			return fmt.Errorf("%w: group %q: volume group must not contain services", ErrManifestCrossValidation, mgroup.GetName())
		}

		if err := mgroup.checkAgainstGSpec(dgroup); err != nil {
			return err
		}
	}

	for _, gspec := range dgroupByName {
		// Volume groups have no services consuming their resources; the
		// utilization accounting below only applies to compute groups.
		if gspec.gs.Volume != nil {
			continue
		}

		for resID, eps := range gspec.endpoints {
			if eps.httpEndpoints > 0 {
				return fmt.Errorf("%w: group %q: resources ID (%d): under-utilized (%d) HTTP endpoints",
					ErrManifestCrossValidation, gspec.gs.Name, resID, eps.httpEndpoints)
			}

			if eps.portEndpoints > 0 {
				return fmt.Errorf("%w: group %q: resources ID (%d): under-utilized (%d) PORT endpoints",
					ErrManifestCrossValidation, gspec.gs.Name, resID, eps.portEndpoints)
			}

			if eps.ipEndpoints > 0 {
				return fmt.Errorf("%w: group %q: resources ID (%d): under-utilized (%d) IP endpoints",
					ErrManifestCrossValidation, gspec.gs.Name, resID, eps.ipEndpoints)
			}
		}

		for _, gRes := range gspec.gs.Resources {
			if gRes.Count > 0 {
				return fmt.Errorf("%w: group %q: resources ID (%d): under-utilized (%d) resources",
					ErrManifestCrossValidation, gspec.gs.GetName(), gRes.ID, gRes.Count)
			}
		}
	}

	return nil
}
