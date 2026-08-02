package cannabis

// GeneticType is the enum for the genetic types
type GeneticType int

const (
	// Sativa is the phenotype of cannabis that is tall and thin with narrow leaves
	Sativa GeneticType = iota
	// Indica is the phenotype of cannabis that is short and bushy with wide leaves
	Indica
	// Hybrid is a mix of both sativa and indica phenotypes
	Hybrid
)

// Genetics is a collection of all known genetic types.
var Genetics = map[GeneticType]string{
	Sativa: "Sativa",
	Indica: "Indica",
	Hybrid: "Hybrid"}

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
//
// The set is not strictly terpenes: it carries the aromatic compounds a
// cannabis certificate of analysis tends to report together, which includes a
// few flavonoids (Apigenin, Cannaflavin A, Quercetin) and a phytosterol
// (β-Sitosterol). They are kept here because they are what a lab result lists
// beside the terpenes, and because they share the one thing this package is
// for: a boiling point, so a temperature on a dial can be read as what it
// releases.
type TerpeneType int

const (
	// BetaCaryophyllene is β-Caryophyllene, the one terpene that binds a
	// cannabinoid receptor (CB2), with anti-inflammatory, anti-malarial and
	// cytoprotective properties
	BetaCaryophyllene TerpeneType = iota
	// BetaSitosterol is β-Sitosterol, a plant sterol with anti-inflammatory properties, acting as a 5-α-reductase inhibitor
	BetaSitosterol
	// AlphaPinene is α-Pinene, with anti-inflammatory, bone stimulant, anti-biotic, bronchodilator and anti-neoplastic properties
	AlphaPinene
	// BetaMyrcene is β-Myrcene, with analgesic, anti-biotic, anti-mutagenic and anti-inflammatory properties
	BetaMyrcene
	// Delta3Carene is Δ-3-Carene, with anti-inflammatory properties
	Delta3Carene
	// Eucalyptol has blood flow stimulant and anti-inflammatory properties
	Eucalyptol
	// Limonene has anti-depressant, anxiolytic and anti-fungal properties
	Limonene
	// PeCymene (P-Cymene) has anti-biotic and anti-candidal properties
	PeCymene
	// Apigenin is a flavonoid with estrogenic and anxiolytic properties
	Apigenin
	// CannaflavinA is a cannabis flavonoid acting as a COX inhibitor and LO inhibitor
	CannaflavinA
	// Linalool has sedative, anti-depressant, anxiolytic and immune potentiator properties
	Linalool
	// Terpinen4Ol (terpinen-4-ol) has antibiotic properties and acts as a AChE inhibitor
	Terpinen4Ol
	// Borneol has antibiotic and anti-inflammatory properties
	Borneol
	// AlphaTerpineol is α-Terpineol, with sedative, anti-biotic, anti-oxidant and anti-malarial properties
	AlphaTerpineol
	// Pulegone has sedative and anti-pyretic properties
	Pulegone
	// Quercetin is a flavonoid with anti-mutagenic, anti-viral, anti-oxidant and anti-neoplastic properties
	Quercetin
	// Humulene is α-Humulene, an isomer of caryophyllene, with anti-inflammatory, anti-biotic, appetite suppressant and analgesic properties
	Humulene
	// Terpinolene has antioxidant, sedative, anti-biotic and anti-fungal properties
	Terpinolene
	// Nerolidol has sedative, anti-fungal, anti-malarial and anti-parasitic properties, and enhances skin penetration
	Nerolidol
	// Bisabolol is α-Bisabolol, the soothing compound of chamomile, with anti-inflammatory, anti-microbial, analgesic and anti-irritant properties
	Bisabolol
	// Camphene has antioxidant, anti-inflammatory and cardioprotective properties
	Camphene
	// Sabinene has antioxidant, anti-inflammatory and anti-microbial properties
	Sabinene
	// Phytol is a chlorophyll degradation product with sedative, relaxant, antioxidant and anxiolytic properties
	Phytol
	// Geraniol has antioxidant, neuroprotectant, anti-biotic and anti-fungal properties
	Geraniol
)

// Terpene is the type for a terpene, which is a compound found in cannabis.
type Terpene struct {
	Name         string   // The terpenes name
	Effects      []string // The terpenes subjective effects
	Flavors      []string // The terpenes subjective flavors
	Notes        string   // Where it is found, and what it is
	BoilingPoint int      // The terpenes boiling point in degrees Celsius
}

// Terpenes is a collection of all known terpenes, ordered by the temperature
// at which each begins to volatilise, which is the order they come off a
// vaporizer as it warms.
var Terpenes = map[TerpeneType]*Terpene{
	Nerolidol: {
		Name:         "Nerolidol",
		Effects:      []string{"sedative", "anti-fungal", "anti-malarial", "anti-parasitic", "skin penetration enhancer"},
		Flavors:      []string{"floral", "wood", "citrus", "apple"},
		Notes:        "A sesquiterpene alcohol also found in jasmine, ginger and tea tree",
		BoilingPoint: 122},
	BetaCaryophyllene: {
		Name:         "β-Caryophyllene",
		Effects:      []string{"anti-inflammatory", "anti-malarial", "cytoprotective", "analgesic"},
		Flavors:      []string{"pepper", "spicy", "wood"},
		Notes:        "A dietary cannabinoid: the only terpene that binds the CB2 receptor; also in black pepper and cloves",
		BoilingPoint: 130},
	BetaSitosterol: {
		Name:         "β-Sitosterol",
		Effects:      []string{"anti-inflammatory", "5-α-reductase inhibitor"},
		Flavors:      []string{"herbal", "earthy"},
		Notes:        "A plant sterol rather than a terpene; also in avocado and nuts",
		BoilingPoint: 140},
	Bisabolol: {
		Name:         "α-Bisabolol",
		Effects:      []string{"anti-inflammatory", "anti-microbial", "analgesic", "anti-irritant"},
		Flavors:      []string{"floral", "chamomile", "sweet", "pepper"},
		Notes:        "The soothing active of German chamomile",
		BoilingPoint: 153},
	AlphaPinene: {
		Name:         "α-Pinene",
		Effects:      []string{"anti-inflammatory", "bone stimulant", "anti-biotic", "bronchodilator", "anti-neoplastic"},
		Flavors:      []string{"pine", "rosemary", "sage"},
		Notes:        "The most common terpene in nature; the scent of pine and rosemary",
		BoilingPoint: 157},
	Camphene: {
		Name:         "Camphene",
		Effects:      []string{"antioxidant", "anti-inflammatory", "cardioprotective"},
		Flavors:      []string{"pine", "fir", "earthy", "damp"},
		Notes:        "Smells of fir needles; studied for cholesterol and triglycerides",
		BoilingPoint: 159},
	Sabinene: {
		Name:         "Sabinene",
		Effects:      []string{"antioxidant", "anti-inflammatory", "anti-microbial"},
		Flavors:      []string{"pepper", "pine", "citrus", "spicy"},
		Notes:        "Also in black pepper, nutmeg and tea tree",
		BoilingPoint: 163},
	BetaMyrcene: {
		Name:         "β-Myrcene",
		Effects:      []string{"analgesic", "anti-biotic", "anti-mutagenic", "anti-inflammatory", "sedative"},
		Flavors:      []string{"musk", "earth", "herbal"},
		Notes:        "Often the most abundant terpene in cannabis; also in mango and hops",
		BoilingPoint: 165},
	Delta3Carene: {
		Name:         "Δ-3-Carene",
		Effects:      []string{"anti-inflammatory"},
		Flavors:      []string{"sweet", "pine", "cedar"},
		Notes:        "Also in pine, cedar and rosemary; may dry the eyes and mouth",
		BoilingPoint: 165},
	Eucalyptol: {
		Name:         "Eucalyptol",
		Effects:      []string{"blood flow stimulant", "anti-inflammatory"},
		Flavors:      []string{"mint", "spicy", "cool"},
		Notes:        "Also called 1,8-cineole; the cooling note of eucalyptus",
		BoilingPoint: 175},
	Limonene: {
		Name:         "Limonene",
		Effects:      []string{"anti-depressant", "anxiolytic", "anti-fungal"},
		Flavors:      []string{"citrus", "lemon", "orange"},
		Notes:        "The scent of citrus peel; also in juniper and peppermint",
		BoilingPoint: 175},
	PeCymene: {
		Name:         "P-Cymene",
		Effects:      []string{"anti-biotic", "anti-candidal"},
		Flavors:      []string{"citrus", "herbal", "spicy"},
		Notes:        "A precursor to carvacrol; also in cumin and thyme",
		BoilingPoint: 175},
	Apigenin: {
		Name:         "Apigenin",
		Effects:      []string{"estrogenic", "anxiolytic"},
		Flavors:      []string{"herbal", "spicy", "sweet"},
		Notes:        "A flavonoid rather than a terpene; also in chamomile and parsley",
		BoilingPoint: 175},
	CannaflavinA: {
		Name:         "Cannaflavin A",
		Effects:      []string{"COX inhibitor", "LO inhibitor"},
		Flavors:      []string{"herbal", "spicy", "sweet"},
		Notes:        "A cannabis-specific flavonoid; a potent anti-inflammatory",
		BoilingPoint: 185},
	Terpinolene: {
		Name:         "Terpinolene",
		Effects:      []string{"antioxidant", "sedative", "anti-biotic", "anti-fungal"},
		Flavors:      []string{"pine", "floral", "herbal", "citrus"},
		Notes:        "One of the six major cannabis terpenes; also in nutmeg, apples and tea tree",
		BoilingPoint: 186},
	Linalool: {
		Name:         "Linalool",
		Effects:      []string{"sedative", "anti-depressant", "anxiolytic", "immune potentiator"},
		Flavors:      []string{"floral", "lavender", "citrus"},
		Notes:        "The floral note of lavender; also in coriander",
		BoilingPoint: 195},
	Humulene: {
		Name:         "α-Humulene",
		Effects:      []string{"anti-inflammatory", "anti-biotic", "appetite suppressant", "analgesic"},
		Flavors:      []string{"hops", "earthy", "wood", "herbal"},
		Notes:        "An isomer of caryophyllene; the earthy bite of hops and cloves",
		BoilingPoint: 198},
	Phytol: {
		Name:         "Phytol",
		Effects:      []string{"sedative", "relaxant", "antioxidant", "anxiolytic"},
		Flavors:      []string{"floral", "balsamic", "grassy"},
		Notes:        "A chlorophyll degradation product and precursor to vitamins E and K",
		BoilingPoint: 204},
	Terpinen4Ol: {
		Name:         "Terpinen-4-ol",
		Effects:      []string{"anti-biotic", "AChE inhibitor"},
		Flavors:      []string{"herbal", "spicy", "sweet"},
		Notes:        "The main active of tea tree oil",
		BoilingPoint: 205},
	Borneol: {
		Name:         "Borneol",
		Effects:      []string{"anti-biotic", "anti-inflammatory"},
		Flavors:      []string{"mint", "camphor", "spicy"},
		Notes:        "A camphor-like crystal used in traditional Chinese medicine",
		BoilingPoint: 205},
	AlphaTerpineol: {
		Name:         "α-Terpineol",
		Effects:      []string{"sedative", "anti-biotic", "anti-oxidant", "anti-malarial"},
		Flavors:      []string{"floral", "citrus", "apple"},
		Notes:        "Also in pine and lilac; a common note in cosmetics",
		BoilingPoint: 220},
	Pulegone: {
		Name:         "Pulegone",
		Effects:      []string{"sedative", "anti-pyretic"},
		Flavors:      []string{"mint", "camphor", "spicy"},
		Notes:        "The scent of pennyroyal; sedative, but a liver irritant in quantity",
		BoilingPoint: 220},
	Quercetin: {
		Name:         "Quercetin",
		Effects:      []string{"anti-mutagenic", "anti-viral", "anti-oxidant", "anti-neoplastic"},
		Flavors:      []string{"herbal", "spicy", "sweet"},
		Notes:        "A flavonoid rather than a terpene; widespread in fruit and vegetables",
		BoilingPoint: 220},
	Geraniol: {
		Name:         "Geraniol",
		Effects:      []string{"antioxidant", "neuroprotectant", "anti-biotic", "anti-fungal"},
		Flavors:      []string{"rose", "floral", "citrus", "sweet"},
		Notes:        "The scent of rose oil and citronella; a natural mosquito repellent",
		BoilingPoint: 230}}
