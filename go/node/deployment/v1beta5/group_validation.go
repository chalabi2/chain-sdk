package v1beta5

import (
	v1 "pkg.akt.dev/go/node/deployment/v1"
)

// ValidateDeploymentGroups does validation for all deployment groups
func ValidateDeploymentGroups(gspecs []GroupSpec) error {
	if len(gspecs) == 0 {
		return v1.ErrInvalidGroups
	}

	names := make(map[string]int, len(gspecs)) // Used as set
	denom := ""
	for idx, group := range gspecs {
		// a deployment containing a volume group contains only that group:
		// the single-group invariant keeps gseq == 1 for every volume and
		// makes the escrow account coterminous with the volume.
		if group.Volume != nil && len(gspecs) != 1 {
			return v1.ErrInvalidGroups.Wrapf("volume group %q must be the only group in a deployment", group.GetName())
		}

		// all must be the same denomination
		if idx == 0 {
			denom = group.Price().Denom
		} else if group.Price().Denom != denom {
			return v1.ErrInvalidPrice.Wrapf("inconsistent group denomination: %v != %v", denom, group.Price().Denom)
		}

		if err := group.ValidateBasic(); err != nil {
			return err
		}

		if _, exists := names[group.GetName()]; exists {
			return v1.ErrDuplicateGroupName.Wrapf("duplicate deployment group name %s", group.GetName())
		}

		names[group.GetName()] = 0 // Value stored does not matter
	}

	if denom == "" {
		return v1.ErrInvalidPrice.Wrapf("unsupported denomination %s", denom)
	}
	return nil
}
