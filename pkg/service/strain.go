package service

import (
	can "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/TheDonDope/wits/pkg/storage"
)

// StrainService provides operations on strains.
type StrainService interface {
	AddStrain(s *can.Strain) error
	GetStrains() []*can.Strain
	FindStrainByProduct(p string) (*can.Strain, error)
}

// StrainServiceType provides operations on strains, accessing a store.
type StrainServiceType struct {
	store storage.StrainStore
}

// NewStrainService creates a new service layer for strains.
func NewStrainService(s storage.StrainStore) *StrainServiceType {
	return &StrainServiceType{store: s}
}

// AddStrain adds a strain to the store.
func (svc *StrainServiceType) AddStrain(s *can.Strain) error {
	return svc.store.AddStrain(s)
}

// GetStrains retrieves all strains from the store.
func (svc *StrainServiceType) GetStrains() []*can.Strain {
	return svc.store.GetStrains()
}

// FindStrainByProduct looks up a strain by its prodcut name.
func (svc *StrainServiceType) FindStrainByProduct(p string) (*can.Strain, error) {
	return svc.store.FindStrainByProduct(p)
}
