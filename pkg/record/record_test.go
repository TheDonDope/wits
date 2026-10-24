package record

import (
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/catalog"
	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/ledger"
	"github.com/TheDonDope/wits/pkg/repo"
)

// TestMain handles global test setup
func TestMain(m *testing.M) {
	// Disable log output during tests
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// recorder returns a recorder over a fresh repository.
func recorder(t *testing.T) *Recorder {
	t.Helper()
	r, err := repo.Init(t.TempDir())
	require.NoError(t, err)
	return New(r, &catalog.Catalog{}, &catalog.Devices{}, ledger.Fold(nil))
}

func TestBuy(t *testing.T) {
	rec := recorder(t)

	e, p, added, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
	require.NoError(t, err)

	assert.True(t, added, "Should register a product it has not seen")
	assert.Equal(t, "wcake-221", p.Slug, "Should slug the name")
	assert.Equal(t, journal.Purchase, e.Type, "Should record a purchase")
	assert.Equal(t, 20.0, rec.Available("wcake-221", journal.Storage), "Should land in storage")

	_, _, added, err = rec.Buy("Enua 22/1 Wedding Cake", "", 10, time.Now())
	require.NoError(t, err)
	assert.False(t, added, "Should reuse the product the second time")
	assert.Equal(t, 30.0, rec.Available("wcake-221", journal.Storage), "Should top up storage")
}

func TestGrind(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
	require.NoError(t, err)

	t.Run("MovesStorageIntoTheStash", func(t *testing.T) {
		_, err := rec.Grind("wedding", 0.75, time.Now())
		require.NoError(t, err)

		assert.Equal(t, 19.25, rec.Available("wcake-221", journal.Storage), "Should leave storage short")
		assert.Equal(t, 0.75, rec.Available("wcake-221", journal.Stash), "Should fill the stash")
	})

	t.Run("RefusesToOverdraw", func(t *testing.T) {
		_, err := rec.Grind("wedding", 500, time.Now())

		assert.ErrorContains(t, err, "cannot take", "Should refuse rather than go negative")
		assert.Equal(t, 19.25, rec.Available("wcake-221", journal.Storage), "Should not have recorded anything")
	})

	t.Run("UnknownProduct", func(t *testing.T) {
		_, err := rec.Grind("blueberry", 1, time.Now())
		assert.Error(t, err, "Should not grind a product it has never seen")
	})
}

func TestSession(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
	require.NoError(t, err)
	_, err = rec.Grind("wedding", 1.0, time.Now())
	require.NoError(t, err)

	t.Run("DrawsOnTheStash", func(t *testing.T) {
		e, err := rec.Session("wedding", 0.4, time.Now(), "", 0, "evening")
		require.NoError(t, err)

		assert.Equal(t, journal.Sesh, e.Type, "Should record a session")
		assert.Equal(t, "evening", e.Note, "Should keep the note")
		assert.Equal(t, 0.6, rec.Available("wcake-221", journal.Stash), "Should empty the stash by that much")
	})

	t.Run("RefusesMoreThanTheStashHolds", func(t *testing.T) {
		_, err := rec.Session("wedding", 99, time.Now(), "", 0, "")
		assert.ErrorContains(t, err, "in the stash", "Should say which account is short")
	})

	t.Run("RefusesATemperatureTheDeviceCannotReach", func(t *testing.T) {
		devices := &catalog.Devices{}
		require.NoError(t, devices.Add(&catalog.Device{Name: "Volcano", MaxTemp: 230, DefaultTemp: 185}))
		rec.devices = devices

		_, err := rec.Session("wedding", 0.1, time.Now(), "volcano", 250, "")
		assert.ErrorContains(t, err, "only goes up to", "Should refuse an impossible setting")
	})

	t.Run("UsesTheDeviceDefaultTemperature", func(t *testing.T) {
		devices := &catalog.Devices{}
		require.NoError(t, devices.Add(&catalog.Device{Name: "Volcano", MaxTemp: 230, DefaultTemp: 185}))
		rec.devices = devices

		e, err := rec.Session("wedding", 0.1, time.Now(), "volcano", 0, "")
		require.NoError(t, err)
		assert.Equal(t, 185, e.Temperature, "Should fall back to the device default")
	})
}

func TestStateFollowsAlong(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
	require.NoError(t, err)

	assert.Len(t, rec.State().Events, 1, "Should fold each entry as it is recorded")
	assert.NotNil(t, rec.State().CurrentCycle(), "Should open a cycle on the first fill")
}

func TestRevert(t *testing.T) {
	t.Run("PutsTheGramsBackWithoutRemovingAnything", func(t *testing.T) {
		rec := recorder(t)
		_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
		require.NoError(t, err)
		grind, err := rec.Grind("wedding", 2, time.Now())
		require.NoError(t, err)

		fix, err := rec.Revert(grind.Hash, "")
		require.NoError(t, err)

		assert.Equal(t, 20.0, rec.Available("wcake-221", journal.Storage), "Should restore storage")
		assert.Zero(t, rec.Available("wcake-221", journal.Stash), "Should empty the stash again")
		assert.Equal(t, grind.Hash, fix.Reverts, "The correction should name what it undid")
		assert.Len(t, rec.State().Events, 3, "Should keep the original and add the correction")
	})

	t.Run("RefusesToUndoTwice", func(t *testing.T) {
		rec := recorder(t)
		_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
		require.NoError(t, err)
		grind, err := rec.Grind("wedding", 2, time.Now())
		require.NoError(t, err)
		_, err = rec.Revert(grind.Hash, "")
		require.NoError(t, err)

		_, err = rec.Revert(grind.Hash, "")
		assert.ErrorContains(t, err, "already been corrected", "Should not undo the same entry twice")
	})

	t.Run("RefusesWhenTheGramsHaveMovedOn", func(t *testing.T) {
		rec := recorder(t)
		_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
		require.NoError(t, err)
		grind, err := rec.Grind("wedding", 2, time.Now())
		require.NoError(t, err)
		_, err = rec.Session("wedding", 1.5, time.Now(), "", 0, "")
		require.NoError(t, err)

		_, err = rec.Revert(grind.Hash, "")

		assert.ErrorContains(t, err, "cannot undo",
			"Should refuse when the stash no longer holds what would have to go back")
	})

	t.Run("UnknownEntry", func(t *testing.T) {
		rec := recorder(t)
		_, err := rec.Revert("nope", "")
		assert.Error(t, err, "Should report an entry it cannot find")
	})
}

func TestAmend(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
	require.NoError(t, err)
	grind, err := rec.Grind("wedding", 7.5, time.Now())
	require.NoError(t, err)

	corrected, err := rec.Amend(grind.Hash, 0.75, "misread the scale")
	require.NoError(t, err)

	assert.Equal(t, 0.75, corrected.Grams, "Should record the corrected amount")
	assert.Equal(t, 0.75, rec.Available("wcake-221", journal.Stash), "The stash should hold the corrected amount")
	assert.Equal(t, 19.25, rec.Available("wcake-221", journal.Storage), "Storage should reflect the correction")
	assert.Len(t, rec.State().Events, 4, "Should keep the original, the undo and the replacement")
}

func TestUndoASession(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
	require.NoError(t, err)
	_, err = rec.Grind("wedding", 2, time.Now())
	require.NoError(t, err)
	sesh, err := rec.Session("wedding", 0.4, time.Now(), "", 0, "")
	require.NoError(t, err)

	_, err = rec.Revert(sesh.Hash, "")
	require.NoError(t, err, "The grams a session put through a device are in consumed, and can come back")
	assert.Equal(t, 2.0, rec.Available("wcake-221", journal.Stash), "The stash should have its grams back")
	assert.Equal(t, 0.0, rec.Available("wcake-221", journal.Consumed))

	sesh, err = rec.Session("wedding", 0.4, time.Now(), "", 0, "")
	require.NoError(t, err)
	_, err = rec.Amend(sesh.Hash, 0.3, "")
	require.NoError(t, err, "A session should be amendable, which is a revert and a re-record")
	assert.Equal(t, 1.7, rec.Available("wcake-221", journal.Stash))
	assert.Equal(t, 0.3, rec.Available("wcake-221", journal.Consumed))
}

func TestAmendAPurchaseUpward(t *testing.T) {
	rec := recorder(t)
	buy, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
	require.NoError(t, err)

	_, err = rec.Amend(buy.Hash, 25, "the label said 25")
	require.NoError(t, err, "A purchase draws on the pharmacy, which cannot be overdrawn")
	assert.Equal(t, 25.0, rec.Available("wcake-221", journal.Storage))
}

func TestAmendRefusesBeforeReverting(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
	require.NoError(t, err)
	grind, err := rec.Grind("wedding", 7.5, time.Now())
	require.NoError(t, err)

	// Amending up to more than storage can cover must fail whole: the revert
	// and the re-record are two appends, and refusing between them would leave
	// the entry silently undone.
	_, err = rec.Amend(grind.Hash, 25, "")

	assert.ErrorContains(t, err, "cannot amend", "Should refuse an amount storage cannot cover")
	assert.Len(t, rec.State().Events, 2, "Should write nothing when it refuses")
	assert.Equal(t, 7.5, rec.Available("wcake-221", journal.Stash), "The stash should be untouched")

	_, err = rec.Amend(grind.Hash, 0, "")
	assert.ErrorContains(t, err, "positive", "Should refuse a zero amount before writing anything")
	assert.Len(t, rec.State().Events, 2, "Should write nothing for a zero amount either")
}

func TestReverted(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
	require.NoError(t, err)
	grind, err := rec.Grind("wedding", 2, time.Now())
	require.NoError(t, err)
	fix, err := rec.Revert(grind.Hash, "")
	require.NoError(t, err)

	hidden := Reverted(rec.State().Events)

	assert.True(t, hidden[grind.Hash], "The corrected entry should be hideable")
	assert.True(t, hidden[fix.Hash], "and so should the correction itself")
	assert.Len(t, hidden, 2, "but nothing else")
}

// stocked returns a recorder holding 20 g of one product, 2 g of it ground.
func stocked(t *testing.T) *Recorder {
	t.Helper()
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
	require.NoError(t, err)
	_, err = rec.Grind("wedding", 2, time.Now())
	require.NoError(t, err)
	return rec
}

func TestReconcile(t *testing.T) {
	t.Run("LessOnTheScaleThanInTheLedger", func(t *testing.T) {
		rec := stocked(t)

		// 17.60 weighed against 18.00 expected: 0.40 g went somewhere unlogged.
		e, err := rec.Reconcile("wedding", journal.Storage, 17.60, "")
		require.NoError(t, err)

		assert.Equal(t, journal.Adjust, e.Type, "Should record an adjustment")
		assert.InDelta(t, 0.40, e.Grams, 0.001, "Should record the difference, not the weight")
		assert.Equal(t, journal.Storage, e.From, "Should take the grams out of storage")
		assert.Equal(t, journal.External, e.To, "and out of the system")
		assert.InDelta(t, 17.60, rec.Available("wcake-221", journal.Storage), 0.001,
			"Storage should now agree with the scale")
	})

	t.Run("MoreOnTheScaleThanInTheLedger", func(t *testing.T) {
		rec := stocked(t)

		e, err := rec.Reconcile("wedding", journal.Storage, 18.50, "")
		require.NoError(t, err)

		assert.InDelta(t, 0.50, e.Grams, 0.001, "Should record the difference")
		assert.Equal(t, journal.External, e.From, "Should bring the grams in from outside")
		assert.Equal(t, journal.Storage, e.To, "and into storage")
		assert.InDelta(t, 18.50, rec.Available("wcake-221", journal.Storage), 0.001,
			"Storage should now agree with the scale")
	})

	t.Run("TheStash", func(t *testing.T) {
		rec := stocked(t)

		_, err := rec.Reconcile("wedding", journal.Stash, 1.75, "")
		require.NoError(t, err)

		assert.InDelta(t, 1.75, rec.Available("wcake-221", journal.Stash), 0.001,
			"Should reconcile the stash as readily as storage")
		assert.InDelta(t, 18.0, rec.Available("wcake-221", journal.Storage), 0.001,
			"and should leave the other accounts alone")
	})

	t.Run("NothingToDo", func(t *testing.T) {
		rec := stocked(t)
		before := len(rec.State().Events)

		_, err := rec.Reconcile("wedding", journal.Storage, 18.00, "")

		assert.ErrorIs(t, err, ErrNothingToReconcile, "Should say so rather than record a zero")
		assert.Len(t, rec.State().Events, before, "and should record nothing")
	})

	t.Run("WeighingToNothing", func(t *testing.T) {
		rec := stocked(t)

		_, err := rec.Reconcile("wedding", journal.Storage, 0, "an empty jar")
		require.NoError(t, err)

		assert.Zero(t, rec.Available("wcake-221", journal.Storage),
			"An empty jar is a legitimate reading, and should empty the account")
	})

	t.Run("RecordsWhyByDefault", func(t *testing.T) {
		rec := stocked(t)

		e, err := rec.Reconcile("wedding", journal.Storage, 17.60, "")
		require.NoError(t, err)

		assert.Contains(t, e.Note, "17.60", "Should note what was weighed")
		assert.Contains(t, e.Note, "18.00", "and what was expected, since the entry itself only carries the difference")
	})

	t.Run("KeepsAGivenReason", func(t *testing.T) {
		rec := stocked(t)

		e, err := rec.Reconcile("wedding", journal.Storage, 17.60, "spilled on the desk")
		require.NoError(t, err)

		assert.Equal(t, "spilled on the desk", e.Note, "Should keep what was actually said")
	})

	t.Run("Refusals", func(t *testing.T) {
		rec := stocked(t)

		_, err := rec.Reconcile("wedding", journal.Storage, -1, "")
		assert.ErrorContains(t, err, "cannot be negative", "Should refuse a negative weight")

		_, err = rec.Reconcile("wedding", journal.Consumed, 1, "")
		assert.ErrorContains(t, err, "cannot be weighed",
			"Should refuse an account with no jar to put on a scale")

		_, err = rec.Reconcile("blueberry", journal.Storage, 1, "")
		assert.Error(t, err, "Should refuse a product it has never seen")
	})

	t.Run("MassStillBalancesAfterwards", func(t *testing.T) {
		rec := stocked(t)
		_, err := rec.Reconcile("wedding", journal.Storage, 17.60, "")
		require.NoError(t, err)

		// An adjustment is a transfer, so the fold still accounts for every gram
		// that came in, including the ones that left again.
		b := rec.State().Balances["wcake-221"]
		assert.InDelta(t, 19.60, b.Storage+b.Stash+b.Consumed+b.AVB, 0.001,
			"20 g bought, 0.40 g adjusted out")
	})
}

func TestDifference(t *testing.T) {
	rec := stocked(t)

	expected, difference, err := rec.Difference("wedding", journal.Storage, 17.60)
	require.NoError(t, err)

	assert.InDelta(t, 18.00, expected, 0.001, "Should report what the ledger believes")
	assert.InDelta(t, -0.40, difference, 0.001, "and the signed difference")
	assert.Len(t, rec.State().Events, 2, "without recording anything")
}

func TestBuyWithASlug(t *testing.T) {
	t.Run("MakesOneUpWhenNoneIsGiven", func(t *testing.T) {
		rec := recorder(t)

		_, p, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
		require.NoError(t, err)

		assert.Equal(t, "wcake-221", p.Slug, "Should abbreviate the cultivar")
		assert.NoError(t, catalog.CheckSlug(p.Slug), "and produce something usable")
	})

	t.Run("KeepsOneThatIsGiven", func(t *testing.T) {
		rec := recorder(t)

		_, p, _, err := rec.Buy("Enua 22/1 Wedding Cake", "cake", 20, time.Now())
		require.NoError(t, err)

		assert.Equal(t, "cake", p.Slug, "Should use what was asked for")
		_, err = rec.Grind("cake", 1, time.Now())
		assert.NoError(t, err, "and it should work as a reference")
	})

	t.Run("RefusesASlugAlreadyInUse", func(t *testing.T) {
		rec := recorder(t)
		_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "cake", 20, time.Now())
		require.NoError(t, err)

		_, _, _, err = rec.Buy("Khiron 20/1 Munson", "cake", 10, time.Now())

		assert.ErrorContains(t, err, "already in use",
			"Two products sharing a slug would make every later entry ambiguous")
	})

	t.Run("RefusesAnUnusableSlug", func(t *testing.T) {
		rec := recorder(t)

		_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "Wedding Cake", 20, time.Now())

		assert.ErrorIs(t, err, catalog.ErrBadSlug, "Should refuse something that cannot be typed as one word")
	})

	t.Run("BuyingAgainKeepsTheFirstSlug", func(t *testing.T) {
		rec := recorder(t)
		_, first, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
		require.NoError(t, err)

		_, again, added, err := rec.Buy("Enua 22/1 Wedding Cake", "", 10, time.Now())
		require.NoError(t, err)

		assert.False(t, added, "Should recognise the product")
		assert.Equal(t, first.Slug, again.Slug, "and refer to it the same way as before")
	})
}

func TestBackdating(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now())
	require.NoError(t, err)

	t.Run("AFillCanBeDatedAnywhere", func(t *testing.T) {
		_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 5, time.Now().AddDate(-1, 0, 0))
		require.NoError(t, err, "A year back")
		require.Len(t, rec.State().Cycles, 2, "It opens a cycle of its own, a year earlier")
		assert.Equal(t, 5.0, rec.State().Cycles[0].Purchased)

		_, _, _, err = rec.Buy("Enua 22/1 Wedding Cake", "", 5, time.Now().AddDate(0, 1, 0))
		require.NoError(t, err, "And ahead")
	})

	t.Run("AGrindCannotComeBeforeItsGrams", func(t *testing.T) {
		_, err := rec.Grind("wedding", 10, time.Now().AddDate(-2, 0, 0))

		assert.ErrorContains(t, err, "the grams were not there yet",
			"Today's storage holds enough, but two years ago there was none")
		assert.Empty(t, rec.State().Overdrawn, "Should have recorded nothing")
	})

	t.Run("UndoingAFillAheadOfTodayIsDatedWithIt", func(t *testing.T) {
		ahead, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 3, time.Now().AddDate(0, 2, 0))
		require.NoError(t, err)

		undo, err := rec.Revert(ahead.Hash, "")
		require.NoError(t, err, "Taking the grams back before they arrived would overdraw storage")
		assert.Equal(t, ahead.OccurredAt, undo.OccurredAt)
	})
}

func TestBuyRefusesPartOfAName(t *testing.T) {
	rec := recorder(t)
	for _, name := range []string{"Cannamedical 28/1 Lemon Cookie", "Cantourage 25/1 MAC1+"} {
		_, _, _, err := rec.Buy(name, "", 10, time.Now())
		require.NoError(t, err)
	}

	_, _, _, err := rec.Buy("MAC1", "", 3, time.Now())
	assert.ErrorIs(t, err, ErrPartialName, "Should not book onto a jar it only guessed at")
	assert.ErrorContains(t, err, "mac1-251", "Should say which product it is part of")
	assert.Equal(t, 10.0, rec.Available("mac1-251", journal.Storage), "Should have recorded nothing")

	_, _, _, err = rec.Buy("Can", "", 2, time.Now())
	assert.ErrorIs(t, err, ErrPartialName, "Should not make a third product out of two partial matches")
	assert.Len(t, rec.products.Products, 2, "Should not have added a product")

	_, _, _, err = rec.Buy("mac1-251", "", 3, time.Now())
	require.NoError(t, err, "The slug is exact")
	_, _, _, err = rec.Buy("cantourage 25/1 mac1+", "", 2, time.Now())
	require.NoError(t, err, "So is the full name, in any case")
	assert.Equal(t, 15.0, rec.Available("mac1-251", journal.Storage))

	_, p, added, err := rec.Buy("MAC1", "mac1", 5, time.Now())
	require.NoError(t, err, "A slug says a new product is meant")
	assert.True(t, added)
	assert.Equal(t, "mac1", p.Slug)
}

func TestRenameAndDescribe(t *testing.T) {
	t.Run("CorrectsTheName", func(t *testing.T) {
		rec := stocked(t)

		p, err := rec.Rename("wcake-221", "Enua 22/1 Wedding Cake (Batch 2)")
		require.NoError(t, err)

		assert.Equal(t, "Enua 22/1 Wedding Cake (Batch 2)", p.Name, "Should read as asked")
		assert.Equal(t, "wcake-221", p.Slug, "and keep the slug the journal refers to")
	})

	t.Run("RefusesAnEmptyName", func(t *testing.T) {
		rec := stocked(t)

		_, err := rec.Rename("wcake-221", "   ")

		assert.ErrorContains(t, err, "needs a name", "Should refuse to leave it nameless")
	})

	t.Run("DescribeReplacesTheParsedDetails", func(t *testing.T) {
		rec := stocked(t)

		p, err := rec.Describe("wcake-221", catalog.Product{
			Name: "Wedding Cake", Manufacturer: "Enua", Cultivar: "Wedding Cake", THC: 21.5, CBD: 0.8,
		})
		require.NoError(t, err)

		assert.Equal(t, "Enua", p.Manufacturer, "Should correct the manufacturer")
		assert.InDelta(t, 21.5, p.THC, 0.001, "and the potency the parser guessed at")
		assert.Equal(t, "wcake-221", p.Slug, "without touching the slug")
	})

	t.Run("EntriesStillResolveAfterwards", func(t *testing.T) {
		rec := stocked(t)
		_, err := rec.Rename("wcake-221", "Something Else Entirely")
		require.NoError(t, err)

		// The journal refers to the slug, so a rename cannot orphan anything.
		_, err = rec.Grind("wcake-221", 1, time.Now())
		assert.NoError(t, err, "Should still take entries against the renamed product")
		assert.InDelta(t, 17.0, rec.Available("wcake-221", journal.Storage), 0.001,
			"and its balances should be unchanged")
	})
}

// snapshot opens the repository again, the way a second process would: its own
// repository value, its own journal handle, its own fold.
func snapshot(t *testing.T, rec *Recorder) *Recorder {
	t.Helper()
	r, err := repo.Discover(rec.repo.WorkTree())
	require.NoError(t, err)
	events, err := r.Journal().Events()
	require.NoError(t, err)
	return New(r, rec.products, rec.devices, ledger.Fold(events))
}

func TestChecksTheJournalNotTheSnapshot(t *testing.T) {
	setup := func(t *testing.T) (tui, cli *Recorder) {
		rec := recorder(t)
		_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 10, time.Now().AddDate(0, 0, -1))
		require.NoError(t, err)
		return snapshot(t, rec), snapshot(t, rec)
	}

	t.Run("TwoGrindsOfOneJar", func(t *testing.T) {
		tui, cli := setup(t)
		_, err := cli.Grind("wedding", 10, time.Now())
		require.NoError(t, err)

		_, err = tui.Grind("wedding", 10, time.Now())

		assert.ErrorContains(t, err, "only 0.00g", "The TUI's snapshot still says 10 g; the journal says none")
		assert.Equal(t, 0.0, snapshot(t, cli).Available("wcake-221", journal.Storage), "Storage should not go below zero")
	})

	t.Run("TwoUndoesOfOneEntry", func(t *testing.T) {
		tui, cli := setup(t)
		grind, err := cli.Grind("wedding", 2, time.Now())
		require.NoError(t, err)
		tui = snapshot(t, tui)
		_, err = cli.Revert(grind.Hash, "")
		require.NoError(t, err)

		_, err = tui.Revert(grind.Hash, "")

		assert.ErrorContains(t, err, "already been corrected", "Undoing twice would put the grams back twice")
	})

	t.Run("AWeighingAfterAGrindElsewhere", func(t *testing.T) {
		tui, cli := setup(t)
		_, err := cli.Grind("wedding", 3, time.Now())
		require.NoError(t, err)

		// The scale reads what the jar holds: 7 g. The snapshot expects 10.
		_, err = tui.Reconcile("wedding", journal.Storage, 7, "")

		assert.ErrorIs(t, err, ErrNothingToReconcile, "The grind elsewhere already accounts for the 3 g")
	})
}

func TestAmendWritesBothEntriesTogether(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 20, time.Now().AddDate(0, 0, -1))
	require.NoError(t, err)
	grind, err := rec.Grind("wedding", 2, time.Now())
	require.NoError(t, err)

	corrected, err := rec.Amend(grind.Hash, 1.5, "")
	require.NoError(t, err)

	events, err := rec.repo.Journal().Events()
	require.NoError(t, err)
	require.Len(t, events, 4)
	assert.Equal(t, grind.Hash, events[2].Reverts, "The undo comes first")
	assert.Equal(t, corrected.Hash, events[3].Hash, "and the re-recorded entry straight after it")
	assert.Equal(t, events[2].Hash, events[3].Prev)
}
