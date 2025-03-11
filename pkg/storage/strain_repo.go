package storage

import (
	"errors"
	"time"

	can "github.com/TheDonDope/wits/pkg/cannabis"
	"github.com/google/uuid"
)

var (
	// ErrStrainNotFound is returned when a strain is not found in the store.
	ErrStrainNotFound = errors.New("Strain with that product name not found")
	// ErrStrainAlreadyExists is returned when a strain with the same product name already exists in the store.
	ErrStrainAlreadyExists = errors.New("Strain with that product name already exists")
)

// StrainStore is an interface for storing strains.
type StrainStore interface {
	AddStrain(strain *can.Strain) error
	GetStrains() []*can.Strain
	FindStrain(cultivar string) (*can.Strain, error)
}

// StrainStoreInMemory is an in-memory store for strains at runtime
type StrainStoreInMemory struct {
	Strains map[string]*can.Strain
}

// NewStrainStoreInMemory creates a new in-memory Strain store.
func NewStrainStoreInMemory() *StrainStoreInMemory {
	return &StrainStoreInMemory{
		Strains: make(map[string]*can.Strain),
	}
}

// AddStrain adds a strain to the store, using its product name as the key.
func (s *StrainStoreInMemory) AddStrain(strain *can.Strain) error {
	if _, exists := s.Strains[strain.Strain]; exists {
		return ErrStrainAlreadyExists
	}
	s.Strains[strain.Strain] = strain
	return nil
}

// GetStrains returns all strains in the store as a slice.
func (s *StrainStoreInMemory) GetStrains() []*can.Strain {
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

// FindStrain finds a strain in the store by product name.
func (s *StrainStoreInMemory) FindStrain(product string) (*can.Strain, error) {
	strain, exists := s.Strains[product]
	if !exists {
		return nil, ErrStrainNotFound
	}
	return strain, nil
}
