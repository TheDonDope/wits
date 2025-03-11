package service

import (
	can "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/TheDonDope/wits/pkg/storage"
)

// StrainService provides operations on strains.
type StrainService interface {
	AddStrain(strain *can.Strain) error
	GetStrains() []*can.Strain
	FindStrainByProduct(product string) (*can.Strain, error)
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
func (s *StrainServiceType) AddStrain(strain *can.Strain) error {
	return s.store.AddStrain(strain)
}

// GetStrains retrieves all strains from the store.
func (s *StrainServiceType) GetStrains() []*can.Strain {
	return s.store.GetStrains()
}

// FindStrainByProduct looks up a strain by its prodcut name.
func (s *StrainServiceType) FindStrainByProduct(product string) (*can.Strain, error) {
	return s.store.FindStrain(product)
}
