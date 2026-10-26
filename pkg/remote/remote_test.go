package remote

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheDonDope/wits/pkg/bundle"
	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/effects"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/prices"
	"github.com/TheDonDope/wits/pkg/repo"
	"github.com/TheDonDope/wits/pkg/seal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pair is two repositories sharing one remote folder, the deck's key
// brought to the phone.
func pair(t *testing.T) (deck, phone *repo.Repo) {
	t.Helper()
	share := t.TempDir()
	var err error
	deck, err = repo.Init(t.TempDir())
	require.NoError(t, err)
	phone, err = repo.Init(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, Add(deck, share, ""))
	require.NoError(t, Add(phone, share, deck.RemoteKeyPath()))
	return deck, phone
}

var july = time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC)

// record appends entries to a repository's journal, the product they name
// added to its catalog.
func record(t *testing.T, r *repo.Repo, events ...journal.Event) []journal.Event {
	t.Helper()
	_, err := r.UpdateProducts(func(c *catalog.Catalog) error {
		if findProduct(c, "wcake") == nil {
			c.Products = append(c.Products, &catalog.Product{Slug: "wcake", Name: "Enua 22/1 Wedding Cake", AddedAt: july})
		}
		return nil
	})
	require.NoError(t, err)
	var stored []journal.Event
	for _, e := range events {
		e.Product = "wcake"
		s, err := r.Journal().Append(e)
		require.NoError(t, err)
		stored = append(stored, s)
	}
	return stored
}

func buy(g float64, day int) journal.Event {
	return journal.Event{Type: journal.Purchase, Grams: g, OccurredAt: july.AddDate(0, 0, day), RecordedAt: july.AddDate(0, 0, day)}
}

func grind(g float64, day int) journal.Event {
	return journal.Event{Type: journal.Grind, Grams: g, OccurredAt: july.AddDate(0, 0, day), RecordedAt: july.AddDate(0, 0, day)}
}

func sesh(g float64, day int) journal.Event {
	return journal.Event{Type: journal.Sesh, Grams: g, OccurredAt: july.AddDate(0, 0, day), RecordedAt: july.AddDate(0, 0, day)}
}

// same reports whether two repositories hold the same journal and effect log,
// byte for byte.
func same(t *testing.T, a, b *repo.Repo) {
	t.Helper()
	for _, path := range []func(*repo.Repo) string{(*repo.Repo).JournalPath, (*repo.Repo).EffectsPath} {
		x, _ := os.ReadFile(path(a))
		y, _ := os.ReadFile(path(b))
		assert.Equal(t, string(x), string(y), "Should hold the same %s", filepath.Base(path(a)))
	}
}

func TestPushAndPull(t *testing.T) {
	t.Run("BringsEverythingAcross", func(t *testing.T) {
		deck, phone := pair(t)
		stored := record(t, deck, buy(20, 0), grind(1, 1), sesh(0.5, 1))
		_, err := deck.Effects().Append(effects.Rating{Session: stored[2].Hash, At: july, Scores: map[effects.Dimension]int{effects.Calm: 4}})
		require.NoError(t, err)

		pushed, err := Push(deck)
		require.NoError(t, err)
		pulled, err := Pull(phone)

		require.NoError(t, err)
		assert.Equal(t, Pushed{Events: 3, Ratings: 1}, pushed)
		assert.Equal(t, 3, pulled.Fetched, "Should bring every entry")
		assert.Equal(t, 1, pulled.Ratings, "Should bring every rating")
		assert.Equal(t, []string{"wcake"}, pulled.Products, "Should bring the product the entries name")
		same(t, deck, phone)
	})

	t.Run("ConvergesAfterBothRecorded", func(t *testing.T) {
		deck, phone := pair(t)
		record(t, deck, buy(20, 0))
		_, err := Push(deck)
		require.NoError(t, err)
		_, err = Pull(phone)
		require.NoError(t, err)
		record(t, phone, grind(1, 1))
		mine := record(t, deck, sesh(0.5, 2))
		_, err = deck.Effects().Append(effects.Rating{Session: mine[0].Hash, At: july, Scores: map[effects.Dimension]int{effects.Calm: 4}})
		require.NoError(t, err)

		_, err = Push(phone)
		require.NoError(t, err)
		_, err = Push(deck)
		require.ErrorIs(t, err, ErrBehind, "Should refuse to write over the phone's grind")
		pulled, err := Pull(deck)
		require.NoError(t, err)
		_, err = Push(deck)
		require.NoError(t, err)
		_, err = Pull(phone)
		require.NoError(t, err)

		assert.Equal(t, 1, pulled.Rechained, "Should chain the deck's session again on top")
		same(t, deck, phone)
		ratings, err := phone.Effects().All()
		require.NoError(t, err)
		events, err := phone.Journal().Events()
		require.NoError(t, err)
		assert.Equal(t, events[2].Hash, ratings[0].Session, "Should rate the session by its new hash")
	})

	t.Run("NamesAClash", func(t *testing.T) {
		deck, phone := pair(t)
		record(t, deck, buy(2, 0))
		_, err := Push(deck)
		require.NoError(t, err)
		_, err = Pull(phone)
		require.NoError(t, err)
		record(t, phone, grind(1.5, 1))
		record(t, deck, grind(2, 1))
		_, err = Push(deck)
		require.NoError(t, err)

		pulled, err := Pull(phone)

		require.NoError(t, err)
		require.Len(t, pulled.Clashes, 1, "Should name the grind that took grams that were not there")
		assert.Equal(t, 1.5, pulled.Clashes[0].Event.Grams)
	})

	t.Run("TakesTheRemotesLaterChange", func(t *testing.T) {
		deck, phone := pair(t)
		record(t, deck, buy(2, 0))
		_, err := Push(deck)
		require.NoError(t, err)
		// Named on the phone before the deck's copy was: the deck's is later.
		_, err = phone.UpdateProducts(func(c *catalog.Catalog) error {
			c.Products = []*catalog.Product{{Slug: "wcake", Name: "Wedding Cake", AddedAt: july,
				Changed: catalog.Changed{"name": july}}}
			return nil
		})
		require.NoError(t, err)

		pulled, err := Pull(phone)

		require.NoError(t, err)
		assert.Equal(t, []string{"wcake"}, pulled.Products, "Should say which product it changed")
		c, err := catalog.Load(phone.ProductsPath())
		require.NoError(t, err)
		assert.Equal(t, "Enua 22/1 Wedding Cake", c.Products[0].Name, "Should take the remote's later name")
	})

	t.Run("KeepsWhatWasChangedHereSince", func(t *testing.T) {
		deck, phone := pair(t)
		record(t, deck, buy(2, 0))
		_, err := Push(deck)
		require.NoError(t, err)
		_, err = Pull(phone)
		require.NoError(t, err)
		// Terpenes written down on the phone, and not pushed: a later change than the remote's.
		later := time.Now().Add(time.Hour)
		_, err = phone.UpdateProducts(func(c *catalog.Catalog) error {
			c.Products[0].Terpenes = []string{"Myrcene", "Limonene"}
			c.Products[0].Changed["terpenes"] = later
			return nil
		})
		require.NoError(t, err)

		_, err = Pull(phone)

		require.NoError(t, err)
		c, err := catalog.Load(phone.ProductsPath())
		require.NoError(t, err)
		assert.Equal(t, []string{"Myrcene", "Limonene"}, c.Products[0].Terpenes, "Should keep what was written here since")
		assert.Equal(t, "Enua 22/1 Wedding Cake", c.Products[0].Name, "and the rest as it was")
	})

	t.Run("NothingToPullIsNotAnError", func(t *testing.T) {
		_, phone := pair(t)

		pulled, err := Pull(phone)

		require.NoError(t, err)
		assert.Zero(t, pulled.Fetched)
	})
}

func TestAdd(t *testing.T) {
	t.Run("RefusesAFolderItCannotRead", func(t *testing.T) {
		deck, _ := pair(t)
		record(t, deck, buy(2, 0))
		_, err := Push(deck)
		require.NoError(t, err)
		stranger, err := repo.Init(t.TempDir())
		require.NoError(t, err)

		err = Add(stranger, deck.Config.Remote, "")

		assert.ErrorIs(t, err, ErrNoKey, "Should not take a remote encrypted to another key")
		assert.Empty(t, stranger.Config.Remote, "Should set nothing")
		assert.NoFileExists(t, stranger.RemoteKeyPath(), "Should keep no key it made for nothing")
	})

	t.Run("RefusesWhatIsNotAFolder", func(t *testing.T) {
		r, err := repo.Init(t.TempDir())
		require.NoError(t, err)

		assert.Error(t, Add(r, filepath.Join(t.TempDir(), "nowhere"), ""))
	})

	t.Run("KeepsTheKeyOutOfTheFolder", func(t *testing.T) {
		deck, _ := pair(t)
		record(t, deck, buy(2, 0))
		_, err := Push(deck)
		require.NoError(t, err)

		entries, err := os.ReadDir(deck.Config.Remote)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, File, entries[0].Name(), "Should hold only the encrypted repository")
		info, err := os.Stat(deck.RemoteKeyPath())
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "Should keep the key to its owner")
	})
}

func TestPushWithoutARemote(t *testing.T) {
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)

	_, err = Push(r)

	assert.ErrorIs(t, err, ErrNoRemote)
}

func TestPricesTravelAndFollowTheirPurchase(t *testing.T) {
	deck, phone := pair(t)
	record(t, deck, buy(20, 0))
	_, err := Push(deck)
	require.NoError(t, err)
	_, err = Pull(phone)
	require.NoError(t, err)
	record(t, phone, buy(10, 5))
	mine := record(t, deck, buy(15, 6))
	_, err = deck.Prices().Append(prices.Price{Purchase: mine[0].Hash, At: july, Cents: 12000})
	require.NoError(t, err)

	_, err = Push(phone)
	require.NoError(t, err)
	pulled, err := Pull(deck)
	require.NoError(t, err)
	pushed, err := Push(deck)
	require.NoError(t, err)
	_, err = Pull(phone)
	require.NoError(t, err)

	assert.Equal(t, 1, pulled.Rechained)
	assert.Equal(t, 1, pushed.Prices, "Should push the price")
	paid, err := phone.Prices().All()
	require.NoError(t, err)
	events, err := phone.Journal().Events()
	require.NoError(t, err)
	require.Len(t, paid, 1)
	assert.Equal(t, events[2].Hash, paid[0].Purchase, "Should price the purchase by its new hash")
	x, _ := os.ReadFile(deck.PricesPath())
	y, _ := os.ReadFile(phone.PricesPath())
	assert.Equal(t, string(x), string(y), "Should end with the same price log")
}

func TestSealsTravel(t *testing.T) {
	deck, phone := pair(t)
	record(t, deck, buy(20, 0))
	require.NoError(t, (seal.Store{Dir: deck.SealsDir()}).Save(seal.File{Name: "a", Text: "wits-seal 1\n"}))

	pushed, err := Push(deck)
	require.NoError(t, err)
	pulled, err := Pull(phone)
	require.NoError(t, err)

	assert.Equal(t, 1, pushed.Seals)
	assert.Equal(t, 1, pulled.Seals)
	kept, err := seal.Store{Dir: phone.SealsDir()}.All()
	require.NoError(t, err)
	require.Len(t, kept, 1)
	assert.Equal(t, "wits-seal 1\n", kept[0].Text)
}

// TestTakeProductsKeepsTheNewerRating: a strain rated here after the remote's
// rating of it keeps this one; rated there later, it takes theirs; and a
// product that differs in nothing else is not taken at all.
func TestTakeProductsKeepsTheNewerRating(t *testing.T) {
	earlier := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	later := earlier.Add(48 * time.Hour)
	added := earlier.Add(-time.Hour)
	ours := &catalog.Catalog{Products: []*catalog.Product{
		{Slug: "a", Name: "A", AddedAt: added, Rating: &catalog.StrainRating{Overall: 5, At: later}},
		{Slug: "b", Name: "B", AddedAt: added, Rating: &catalog.StrainRating{Overall: 2, At: earlier}},
	}}
	theirs := bundle.Contents{Products: &catalog.Catalog{Products: []*catalog.Product{
		{Slug: "a", Name: "A", AddedAt: added, Rating: &catalog.StrainRating{Overall: 1, At: earlier}},
		{Slug: "b", Name: "B", AddedAt: added, Rating: &catalog.StrainRating{Overall: 4, At: later}},
	}}}

	took := takeProducts(ours, theirs)

	assert.Equal(t, []string{"b"}, took, "a is ours and newer, so nothing of it is taken")
	assert.Equal(t, 5, ours.Products[0].Rating.Overall)
	assert.Equal(t, 4, ours.Products[1].Rating.Overall)
}
