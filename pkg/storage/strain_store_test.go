package storage

import (
	"io"
	"log"
	"os"
	"testing"
	"time"

	can "github.com/TheDonDope/wits-tui/pkg/cannabis"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain handles global test setup
func TestMain(m *testing.M) {
	// Disable log output during tests
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// testStrain generates a consistent test strain with fixed values
func testStrain() *can.Strain {
	testUUID := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	testTime := time.Date(2023, time.October, 5, 12, 0, 0, 0, time.UTC)

	return &can.Strain{
		ID:           testUUID,
		Strain:       "Test Strain",
		Cultivar:     "Test Cultivar",
		Manufacturer: "Test Manufacturer",
		Country:      "Test Country",
		Genetic:      can.Sativa,
		Radiated:     false,
		THC:          20.0,
		CBD:          0.5,
		Terpenes:     []*can.Terpene{},
		Amount:       3.5,
		CreatedAt:    testTime,
		UpdatedAt:    testTime,
	}
}

// TestInMemoryStore runs all tests for the in-memory store implementation
func TestInMemoryStore(t *testing.T) {
	t.Run("AddStrain", func(t *testing.T) {
		store := &StrainStoreInMemory{strains: make(map[string]*can.Strain)}
		testAddStrain(t, store)
	})

	t.Run("GetStrains", func(t *testing.T) {
		store := &StrainStoreInMemory{strains: make(map[string]*can.Strain)}
		testGetStrains(t, store)
	})

	t.Run("FindStrainByProduct", func(t *testing.T) {
		store := &StrainStoreInMemory{strains: make(map[string]*can.Strain)}
		testFindStrainByProduct(t, store)
	})
}

// TestYAMLFileStore runs all tests for the YAML file store implementation
func TestYAMLFileStore(t *testing.T) {
	t.Run("AddStrain", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("STORAGE_MODE", StoreYMLFile)
		t.Setenv("WITS_DIR", tempDir)
		store := NewStrainStore().(*StrainStoreYMLFile)
		testAddStrain(t, store)
	})

	t.Run("GetStrains", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("STORAGE_MODE", StoreYMLFile)
		t.Setenv("WITS_DIR", tempDir)
		store := NewStrainStore().(*StrainStoreYMLFile)
		testGetStrains(t, store)
	})

	t.Run("FindStrainByProduct", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("STORAGE_MODE", StoreYMLFile)
		t.Setenv("WITS_DIR", tempDir)
		store := NewStrainStore().(*StrainStoreYMLFile)
		testFindStrainByProduct(t, store)
	})

	t.Run("Persistence", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("STORAGE_MODE", StoreYMLFile)
		t.Setenv("WITS_DIR", tempDir)
		store := NewStrainStore().(*StrainStoreYMLFile)

		// Test implementation here
		strain := testStrain()
		require.NoError(t, store.AddStrain(strain))

		// Create new store instance to verify persistence
		newStore := NewStrainStore().(*StrainStoreYMLFile)
		persistedStrain, err := newStore.FindStrainByProduct(strain.Strain)
		require.NoError(t, err)
		assert.Equal(t, strain, persistedStrain)
	})
}

// testAddStrain tests strain addition functionality
func testAddStrain(t *testing.T, store StrainStore) {
	strain := testStrain()

	// Test initial add
	require.NoError(t, store.AddStrain(strain))

	// Test duplicate add
	err := store.AddStrain(strain)
	assert.ErrorIs(t, err, ErrStrainAlreadyExists)

	// Verify count
	strains := store.GetStrains()
	assert.Len(t, strains, 1)
}

// testGetStrains tests retrieval of all strains
func testGetStrains(t *testing.T, store StrainStore) {
	// Verify empty initial state
	assert.Empty(t, store.GetStrains())

	// Add test data
	strain1 := testStrain()
	strain1.Strain = "Strain 1"
	require.NoError(t, store.AddStrain(strain1))

	strain2 := testStrain()
	strain2.Strain = "Strain 2"
	require.NoError(t, store.AddStrain(strain2))

	// Verify retrieval
	strains := store.GetStrains()
	assert.Len(t, strains, 2)
}

// testFindStrainByProduct tests product-based strain lookup
func testFindStrainByProduct(t *testing.T, store StrainStore) {
	strain := testStrain()

	// Test not found case
	_, err := store.FindStrainByProduct(strain.Strain)
	assert.ErrorIs(t, err, ErrStrainNotFound)

	// Add and verify found case
	require.NoError(t, store.AddStrain(strain))
	found, err := store.FindStrainByProduct(strain.Strain)
	require.NoError(t, err)
	assert.Equal(t, strain, found)
}
