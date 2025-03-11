package storage

import (
	"time"

	can "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/google/uuid"
)

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
	if len(strains) == 0 {
		// Add dummy strain for testing
		strains = append(strains, &can.Strain{
			ID:           uuid.New(),
			Strain:       "Barongo 27/1 MAC3",
			Cultivar:     "MAC 3",
			Manufacturer: "WMG Pharma",
			Genetic:      can.Hybrid,
			THC:          27.0,
			CBD:          1.0,
			Terpenes:     []*can.Terpene{can.Terpenes[can.BetaMyrcene], can.Terpenes[can.Limonene], can.Terpenes[can.Linalool], can.Terpenes[can.BetaCaryophyllene]},
			Amount:       5.0,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		})
	}
	return strains
}

// FindStrain finds a strain in the store by cultivar.
func (s *StrainStore) FindStrain(cultivar string) *can.Strain {
	return s.Strains[cultivar]
}
