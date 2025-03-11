package service

import (
	can "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/TheDonDope/wits/pkg/storage"
)

// StrainService provides operations on strains.
type StrainService interface {
	AddStrain(strain *can.Strain)
	GetStrains() []*can.Strain
	FindStrainByCultivar(cultivar string) (*can.Strain, error)
}

// StrainServiceType provides operations on strains.
type StrainServiceType struct {
	store storage.StrainStore
}

// NewStrainService creates a new service layer for strains.
func NewStrainService(store storage.StrainStore) *StrainServiceType {
	return &StrainServiceType{store: store}
}

// AddStrain adds a strain to the store.
func (s *StrainServiceType) AddStrain(strain *can.Strain) {
	s.store.AddStrain(strain)
}

// GetStrains retrieves all strains from the store.
func (s *StrainServiceType) GetStrains() []*can.Strain {
	return s.store.GetStrains()
}

// FindStrainByCultivar looks up a strain by its cultivar.
func (s *StrainServiceType) FindStrainByCultivar(cultivar string) (*can.Strain, error) {
	return s.store.FindStrain(cultivar)
}
