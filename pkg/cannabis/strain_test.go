package cannabis

import "testing"

// TestTerpeneData guards the invariants the rest of the application reads this
// table through: every compound has a name and a boiling point above the floor
// the temperature feature tests assume, and none of the aromatic compounds is
// mislabelled with the effects reserved for a hazard.
func TestTerpeneData(t *testing.T) {
	for typ, terp := range Terpenes {
		if terp.Name == "" {
			t.Errorf("terpene %d has no name", typ)
		}
		// catalog.ReleasedAt is tested against the promise that nothing comes
		// off below 100°C; a terpene under it would break that.
		if terp.BoilingPoint <= 100 {
			t.Errorf("%s boils at %d°C, below the 100°C floor the temperature feature assumes",
				terp.Name, terp.BoilingPoint)
		}
		for _, e := range terp.Effects {
			if e == "toxic" || e == "carcinogenic" {
				t.Errorf("%s is marked %q, which is reserved for hazards like benzene", terp.Name, e)
			}
		}
	}
}

// TestMajorTerpenesPresent checks that the six terpenes a cannabis result is
// most likely to lead with are all in the set, so a real certificate of
// analysis can be described without gaps.
func TestMajorTerpenesPresent(t *testing.T) {
	for _, typ := range []TerpeneType{
		BetaMyrcene, Limonene, BetaCaryophyllene, AlphaPinene, Linalool, Terpinolene, Humulene,
	} {
		if Terpenes[typ] == nil {
			t.Errorf("major terpene %d is missing from the set", typ)
		}
	}
}
