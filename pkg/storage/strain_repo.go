package storage

import can "github.com/TheDonDope/wits/pkg/cannabis"

// StrainStore is an in-memory store for strains at runtime
type StrainStore struct {
	Strains map[string]*can.Strain
}

// NewStrainStore creates a new in-memory Strain store.
func NewStrainStore() *StrainStore {
	return &StrainStore{
		Strains: make(map[string]*can.Strain),
	}
}

// AddStrain adds a strain to the store, using its cultivar as the key.
func (s *StrainStore) AddStrain(strain *can.Strain) {
	s.Strains[strain.Cultivar] = strain
}

// GetStrains returns all strains in the store as a slice.
func (s *StrainStore) GetStrains() []*can.Strain {
	var strains []*can.Strain
	for _, s := range s.Strains {
		strains = append(strains, s)
	}
	return strains
}

// FindStrain finds a strain in the store by cultivar.
func (s *StrainStore) FindStrain(cultivar string) *can.Strain {
	return s.Strains[cultivar]
}
