package cannabis

import (
	"time"

	"github.com/google/uuid"
)

// Strain is the type for a cannabis strain.
type Strain struct {
	ID           uuid.UUID   // The unique identifier
	Strain       string      // The product name
	Cultivar     string      // The breed
	Manufacturer string      // The producer
	Country      string      // The country of origin
	Genetic      GeneticType // The genetic type
	Radiated     bool        // If the strain was radiation treated
	THC          float64     // The THC content in %
	CBD          float64     // The CBD content in %
	Terpenes     []*Terpene  // The terpenes in the strain
	Amount       float64     // The amount in grams
	CreatedAt    time.Time   // The creation timestamp
	UpdatedAt    time.Time   // The last update timestamp
}

// GeneticType is the enum for the genetic types
type GeneticType int

const (
	// Sativa is the phenotype of cannabis that is tall and thin with narrow leaves
	Sativa GeneticType = iota
	// Indica is the phenotype of cannabis that is short and bushy with wide leaves
	Indica
	// Hybrid is a mix of both sativa and indica phenotypes
	Hybrid
	// SativaHybrid is a mix of sativa and another phenotype
	SativaHybrid
	// IndicaHybrid is a mix of indica and another phenotype
	IndicaHybrid
)

// Genetics is a collection of all known genetic types.
var Genetics = map[GeneticType]string{
	Sativa:       "Sativa",
	Indica:       "Indica",
	Hybrid:       "Hybrid",
	SativaHybrid: "Sativa-hybrid",
	IndicaHybrid: "Indica-hybrid"}

// CannabinoidType is the enum for the cannnabinoid keys.
type CannabinoidType int

const (
	// THCA is Tetrahydrocannabinolic acid, acid conversion to THC when heated
	THCA CannabinoidType = iota
	// CBDA is Cannabidiolic acid, acid conversion to CBD when heated
	CBDA
	// CBCA is Cannabichromene acid, acid conversion to CBG when heated
	CBCA
	// Delta9THC is Tetrahydrocannabinol, the psychoactive compound in cannabis
	Delta9THC
	// CBD is Cannabidiol, a non-psychoactive compound in cannabis
	CBD
	// Delta8THC is Tetrahydrocannabinol, a psychoactive compound in cannabis
	Delta8THC
	// CBN is Cannabinol, a mildly psychoactive compound in cannabis
	CBN
	// CBE is Cannabielsoin, a psychoactive compound in cannabis
	CBE
	// Benzene are harmful toxic vapours that can occur above a certain temperature (200°C)
	Benzene
	// THCV is Tetrahydrocannabivarin, a psychoactive compound in cannabis
	THCV
	// CBC is Cannabichromene, a non-psychoactive compound in cannabis
	CBC
)

// Cannabinoid is the type for a cannabinoid, which is a compound found in cannabis.
type Cannabinoid struct {
	ShortName    string   // The cannabinoids short name
	Name         string   // The cannabinoids full name
	Effects      []string // The cannabinoids subjective effects
	Notes        string   // Additional notes
	BoilingPoint int      // The cannabinoids boiling point in degrees Celsius
}

// Cannabinoids is a collection of all known cannabinoids.
var Cannabinoids = map[CannabinoidType]Cannabinoid{
	THCA: {
		ShortName:    "THCA",
		Name:         "Tetrahydrocannabinolic acid",
		Effects:      []string{"anti-inflammatory", "anti-epileptic", "anti-proliferic"},
		Notes:        "Acid Conversion. Requires 30 mins. in the oven",
		BoilingPoint: 120},
	CBDA: {
		ShortName:    "CBDA",
		Name:         "Cannabidiolic acid",
		Effects:      []string{"anti-inflammatory", "anti-proliferic"},
		Notes:        "Acid Conversion. Requires 60 mins. in the oven",
		BoilingPoint: 130},
	CBCA: {
		ShortName:    "CBCA",
		Name:         "Cannabichromene acid",
		Effects:      []string{"anti-bacterial", "anti-fungal"},
		Notes:        "Acid Conversion. Requires 60 mins. in the oven",
		BoilingPoint: 140},
	Delta9THC: {
		ShortName:    "Δ-9-THC",
		Name:         "Tetrahydrocannabinol",
		Effects:      []string{"psychoactive", "anti-inflammatory", "anti-emetic", "appetite stimulant", "anti-proliferic", "anti-oxidant"},
		Notes:        "Delta 9 (Δ-9)",
		BoilingPoint: 157},
	CBD: {
		ShortName:    "CBD",
		Name:         "Cannabidiol",
		Effects:      []string{"non-psychoactive", "anti-inflammatory", "anti-anxiety"},
		Notes:        "Excludes Δ-8",
		BoilingPoint: 165},
	Delta8THC: {
		ShortName:    "Δ-8-THC",
		Name:         "Tetrahydrocannabinol",
		Effects:      []string{"non-psychoactive", "neuroprotective", "anti-emetic"},
		Notes:        "Delta 8 (Δ-8)",
		BoilingPoint: 175},
	CBN: {
		ShortName:    "CBN",
		Name:         "Cannabinol",
		Effects:      []string{"mildly psychoactive", "anti-spasmodic", "anti-insomnia", "analgesic"},
		Notes:        "THC degredation",
		BoilingPoint: 185},
	CBE: {
		ShortName:    "CBE",
		Name:         "Cannabielsoin",
		Effects:      []string{"sedative", "anti-depressant", "anxiolytic"},
		Notes:        "CBD degredation",
		BoilingPoint: 195},
	Benzene: {
		ShortName:    "Benzene",
		Name:         "Benzene",
		Effects:      []string{"toxic", "carcinogenic"},
		Notes:        "Avoid harmful toxic vapours",
		BoilingPoint: 205},
	THCV: {
		ShortName:    "THCV",
		Name:         "Tetrahydrocannabivarin",
		Effects:      []string{"psychoactive", "euphoriant", "anti-thc", "analgesic", "anti-diabetic", "anorectic", "bone stimulant"},
		Notes:        "Blocks THC",
		BoilingPoint: 220},
	CBC: {
		ShortName:    "CBC",
		Name:         "Cannabichromene",
		Effects:      []string{"non-psychoactive", "anti-proliferative", "anti-bacterial", "bone stimulant", "anti-inflammatory", "analgesic"},
		Notes:        "Includes THCV",
		BoilingPoint: 220}}

// TerpeneType is the enum for the terpene keys.
type TerpeneType int

const (
	// BetaCaryophyllene is β-Caryophyllene, with anti-malarial, cytoprotective and anti-inflammatory treatments
	BetaCaryophyllene TerpeneType = iota
	// BetaSitosterol is β-Sitosterol, with anti-inflammatory properties and acting as a 5-α-reductase inhibitor
	BetaSitosterol
	// AlphaPinene is α-Pinene, with anti-inflammatory, bone stimulant, anti-biotic, bronchodilator and anti-neoplatic properties
	AlphaPinene
	// BetaMyrcene is β-Myrcene, with analgesic, anti-biotic, anti-mutagenic and anti-inflammatory properties
	BetaMyrcene
	// Delta3Carene is Δ-3-Carene, with anti-inflammatory properties
	Delta3Carene
	// Eucalyptol has blood flow stimulant properties
	Eucalyptol
	// Limonene has anti-depressant and agonist properties
	Limonene
	// PeCymene (P-Cymene) has anti-biotic and anti-candidal properties
	PeCymene
	// Apigenin has estrogenic and anxiolytic properties
	Apigenin
	// CannaflavinA is a COX inhibtor and LO inhibtor
	CannaflavinA
	// Linalool has sedative, anti-depressant, anxiolytic and immune ptentiator properties (like limonene)
	Linalool
	// Terpinen4Ol (terpinen-4-ol) has antibiotic properties and acts as a AChE inhibitor (like p-cymene)
	Terpinen4Ol
	// Borneol has antibiotic properties
	Borneol
	// AlphaTerpineol is α-Terpineol, with sedative, anti-biotic, anti-oxidant and anti-malarial properties
	AlphaTerpineol
	// Pulegone has sedative and anti-pyretic properties
	Pulegone
	// Quercetin has anti-mutagenic, anti viral, anti-oxidant and anti-neoplastic properties
	Quercetin
)

// Terpene is the type for a terpene, which is a compound found in cannabis.
type Terpene struct {
	Name         string   // The terpenes name
	Effects      []string // The terpenes subjective effects
	Flavors      []string // The terpenes subjective flavors
	BoilingPoint int      // The terpenes boiling point in degrees Celsius
}

// Terpenes is a collection of all known terpenes.
var Terpenes = map[TerpeneType]*Terpene{
	BetaCaryophyllene: {
		Name:         "β-Caryophyllene",
		Effects:      []string{"anti-malarial", "cytoprotective", "anti-inflammatory"},
		Flavors:      []string{"pepper", "spicy", "wood"},
		BoilingPoint: 130},
	BetaSitosterol: {
		Name:         "β-Sitosterol",
		Effects:      []string{"anti-inflammatory", "5-α-reductase inhibitor"},
		Flavors:      []string{"herbal", "earthy"},
		BoilingPoint: 140},
	AlphaPinene: {
		Name:         "α-Pinene",
		Effects:      []string{"anti-inflammatory", "bone stimulant", "anti-biotic", "bronchodilator", "anti-neoplatic"},
		Flavors:      []string{"pine", "rosemary", "sage"},
		BoilingPoint: 157},
	BetaMyrcene: {
		Name:         "β-Myrcene",
		Effects:      []string{"analgesic", "anti-biotic", "anti-mutagenic", "anti-inflammatory"},
		Flavors:      []string{"musk", "earth", "herbal"},
		BoilingPoint: 165},
	Delta3Carene: {
		Name:         "Δ-3-Carene",
		Effects:      []string{"anti-inflammatory"},
		Flavors:      []string{"sweet", "pine", "cedar"},
		BoilingPoint: 165},
	Eucalyptol: {
		Name:         "Eucalyptol",
		Effects:      []string{"blood flow stimulant"},
		Flavors:      []string{"mint", "spicy", "cool"},
		BoilingPoint: 175},
	Limonene: {
		Name:         "Limonene",
		Effects:      []string{"anti-depressant", "agonist"},
		Flavors:      []string{"citrus", "lemon", "orange"},
		BoilingPoint: 175},
	PeCymene: {
		Name:         "P-Cymene",
		Effects:      []string{"anti-biotic", "anti-candidal"},
		Flavors:      []string{"citrus", "herbal", "spicy"},
		BoilingPoint: 175},
	Apigenin: {
		Name:         "Apigenin",
		Effects:      []string{"estrogenic", "anxiolytic"},
		Flavors:      []string{"herbal", "spicy", "sweet"},
		BoilingPoint: 175},
	CannaflavinA: {
		Name:         "Cannaflavin A",
		Effects:      []string{"COX inhibitor", "LO inhibitor"},
		Flavors:      []string{"herbal", "spicy", "sweet"},
		BoilingPoint: 185},
	Linalool: {
		Name:         "Linalool",
		Effects:      []string{"sedative", "anti-depressant", "anxiolytic", "immune potentiator"},
		Flavors:      []string{"floral", "lavender", "citrus"},
		BoilingPoint: 195},
	Terpinen4Ol: {
		Name:         "Terpinen-4-ol",
		Effects:      []string{"anti-biotic", "AChE inhibitor"},
		Flavors:      []string{"herbal", "spicy", "sweet"},
		BoilingPoint: 205},
	Borneol: {
		Name:         "Borneol",
		Effects:      []string{"anti-biotic"},
		Flavors:      []string{"mint", "camphor", "spicy"},
		BoilingPoint: 205},
	AlphaTerpineol: {
		Name:         "α-Terpineol",
		Effects:      []string{"sedative", "anti-biotic", "anti-oxidant", "anti-malarial"},
		Flavors:      []string{"floral", "citrus", "apple"},
		BoilingPoint: 220},
	Pulegone: {
		Name:         "Pulegone",
		Effects:      []string{"sedative", "anti-pyretic"},
		Flavors:      []string{"mint", "camphor", "spicy"},
		BoilingPoint: 220},
	Quercetin: {
		Name:         "Quercetin",
		Effects:      []string{"anti-mutagenic", "anti-viral", "anti-oxidant", "anti-neoplastic"},
		Flavors:      []string{"herbal", "spicy", "sweet"},
		BoilingPoint: 220}}
