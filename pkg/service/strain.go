package service

import (
	can "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/TheDonDope/wits/pkg/storage"
)

// StrainService provides operations on strains.
type StrainService struct {
	store *storage.StrainStore
}

// NewStrainService creates a new service layer for strains.
func NewStrainService(store *storage.StrainStore) *StrainService {
	return &StrainService{store: store}
}

// AddStrain adds a strain to the store.
func (s *StrainService) AddStrain(strain *can.Strain) {
	s.store.AddStrain(strain)
}

// GetStrains retrieves all strains from the store.
func (s *StrainService) GetStrains() []*can.Strain {
	return s.store.GetStrains()
}

// FindStrainByCultivar looks up a strain by its cultivar.
func (s *StrainService) FindStrainByCultivar(cultivar string) *can.Strain {
	return s.store.FindStrain(cultivar)
}
