package v2beta4

import (
	"fmt"
	"sort"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	dtypes "pkg.akt.dev/go/node/deployment/v1beta5"
	rtypes "pkg.akt.dev/go/node/types/resources/v1beta4"
)

func (s *Service) validate(helper *validateManifestGroupsHelper) error {
	if len(s.Name) == 0 {
		return fmt.Errorf("%w: service name is empty", ErrInvalidManifest)
	}

	serviceNameValid := serviceNameValidationRegex.MatchString(s.Name)
	if !serviceNameValid {
		return fmt.Errorf("%w: service %q name is invalid", ErrInvalidManifest, s.Name)
	}

	if len(s.Image) == 0 {
		return fmt.Errorf("%w: service %q has empty image name", ErrInvalidManifest, s.Name)
	}

	// a service mounting only externally-leased volumes carries no local
	// storage entry; the empty slice the SDL emits serializes to absent on
	// the wire and decodes back to nil - normalize before validating.
	if s.Resources.Storage == nil && s.Params != nil && len(s.Params.Storage) > 0 {
		s.Resources.Storage = rtypes.Volumes{}
	}

	if err := s.Resources.Validate(); err != nil {
		return err
	}

	for _, envVar := range s.Env {
		tokens := strings.SplitN(envVar, "=", 2)
		if len(tokens) == 0 {
			return fmt.Errorf("%w: service %q defines an env. var. with an empty name", ErrInvalidManifest, s.Name)
		}

		envVarName := tokens[0]

		if len(k8svalidation.IsEnvVarName(envVarName)) != 0 {
			return fmt.Errorf("%w: service %q defines an env. var. with an invalid name %q", ErrInvalidManifest, s.Name, envVarName)
		}
	}

	if !sort.IsSorted(s.Expose) {
		return fmt.Errorf("%w: service %q: expose is not sorted", ErrInvalidManifest, s.Name)
	}

	for _, serviceExpose := range s.Expose {
		if err := serviceExpose.validate(helper); err != nil {
			return fmt.Errorf("%w: service %q: %w", ErrInvalidManifest, s.Name, err)
		}
	}

	return nil
}

func (s *Service) checkAgainstGSpec(gspec *groupSpec) error {
	// find resource units by id
	var gRes *dtypes.ResourceUnit

	for idx := range gspec.gs.Resources {
		if s.Resources.ID == gspec.gs.Resources[idx].ID {
			gRes = &gspec.gs.Resources[idx]
			break
		}
	}

	if gRes == nil {
		return fmt.Errorf("service %q: not found deployment group resources with ID = %d", s.Name, s.Resources.ID)
	}

	if s.Count > gRes.Count {
		return fmt.Errorf("service %q: over-utilized replicas (%d) > group spec resources count (%d)",
			s.Name, s.Count, gRes.Count)
	}

	// do not compare resources directly
	if !s.Resources.CPU.Equal(gRes.CPU) {
		return fmt.Errorf("service %q: CPU resources mismatch for ID %d", s.Name, s.Resources.ID)
	}

	if !s.Resources.GPU.Equal(gRes.GPU) {
		return fmt.Errorf("service %q: GPU resources mismatch for ID %d", s.Name, s.Resources.ID)
	}

	if !s.Resources.Memory.Equal(gRes.Memory) {
		return fmt.Errorf("service %q: Memory resources mismatch for ID %d", s.Name, s.Resources.ID)
	}

	if !s.Resources.Storage.Equal(gRes.Storage) {
		return fmt.Errorf("service %q: Storage resources mismatch for ID %d", s.Name, s.Resources.ID)
	}

	if err := s.checkVolumesAgainstResources(gRes); err != nil {
		return err
	}

	for _, expose := range s.Expose {
		if err := expose.checkAgainstResources(gRes, gspec.endpoints); err != nil {
			return fmt.Errorf("service %q: resource ID %d: %w", s.Name, gRes.ID, err)
		}
	}

	gRes.Count -= s.Count

	return nil
}

// checkVolumesAgainstResources requires the set of manifest
// StorageParams.volume refs to equal the on-chain ResourceUnit.Volumes
// exactly: index-wise, after canonical sort of the "owner/dseq/gseq/name"
// forms. Attached volumes carry no Storage entry on either side, so this is
// the only handshake for them.
func (s *Service) checkVolumesAgainstResources(gRes *dtypes.ResourceUnit) error {
	var mrefs []string

	if s.Params != nil {
		for _, sp := range s.Params.Storage {
			if sp.Volume != "" {
				mrefs = append(mrefs, sp.Volume)
			}
		}
	}

	grefs := make([]string, 0, len(gRes.Volumes))
	for _, ref := range gRes.Volumes {
		grefs = append(grefs, ref.String())
	}

	sort.Strings(mrefs)
	sort.Strings(grefs)

	if len(mrefs) != len(grefs) {
		return fmt.Errorf("service %q: volume refs count mismatch (%d != %d) for ID %d",
			s.Name, len(mrefs), len(grefs), s.Resources.ID)
	}

	for i := range mrefs {
		if mrefs[i] != grefs[i] {
			return fmt.Errorf("service %q: volume ref mismatch (%q != %q) for ID %d",
				s.Name, mrefs[i], grefs[i], s.Resources.ID)
		}
	}

	return nil
}
