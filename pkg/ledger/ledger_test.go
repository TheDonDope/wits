package ledger

import (
	"testing"
	"time"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// day returns a timestamp n days after the first of the month, so that tests
// read as a sequence of days rather than a wall of dates.
func day(n int) time.Time {
	return time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, n)
}

// event builds an event of the given type with its accounts filled in, the way
// the journal would fill them on append.
func event(typ journal.Type, product string, grams float64, at time.Time) journal.Event {
	from, to, _ := journal.Flow(typ)
	return journal.Event{Type: typ, Product: product, Grams: grams, From: from, To: to, OccurredAt: at}
}

func TestFold(t *testing.T) {
	t.Run("MovesGramsBetweenAccounts", func(t *testing.T) {
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(0)),
			event(journal.Grind, "wedding-cake", 0.75, day(1)),
			event(journal.Sesh, "wedding-cake", 0.5, day(1)),
			event(journal.AVBCollect, "wedding-cake", 0.35, day(1)),
		})

		b := s.Balances["wedding-cake"]
		require.NotNil(t, b)
		assert.Equal(t, 19.25, b.Storage, "Should take the ground amount out of storage")
		assert.Equal(t, 0.25, b.Stash, "Should leave the unconsumed remainder in the stash")
		assert.Equal(t, 0.15, b.Consumed, "Should hold what has not been collected as AVB yet")
		assert.Equal(t, 0.35, b.AVB, "Should credit the weighed AVB")
	})

	t.Run("ConservesMass", func(t *testing.T) {
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(0)),
			event(journal.Grind, "wedding-cake", 1.5, day(1)),
			event(journal.Sesh, "wedding-cake", 1.0, day(1)),
			event(journal.AVBCollect, "wedding-cake", 0.7, day(2)),
		})

		b := s.Balances["wedding-cake"]
		assert.Equal(t, 20.0, Round(b.Storage+b.Stash+b.Consumed+b.AVB), "Should account for every purchased gram")
	})

	t.Run("KeepsStashesSeparate", func(t *testing.T) {
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(0)),
			event(journal.Purchase, "lemon-cookie", 20, day(0)),
			event(journal.Grind, "wedding-cake", 0.75, day(1)),
			event(journal.Grind, "lemon-cookie", 1.25, day(1)),
		})

		assert.Equal(t, 0.75, s.Balances["wedding-cake"].Stash, "Should stash each product on its own")
		assert.Equal(t, 1.25, s.Balances["lemon-cookie"].Stash, "Should stash each product on its own")
		assert.Equal(t, 19.25, s.Balances["wedding-cake"].Storage, "Should not cross products")
		assert.Equal(t, 18.75, s.Balances["lemon-cookie"].Storage, "Should not cross products")
	})

	t.Run("ProductsAreSorted", func(t *testing.T) {
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(0)),
			event(journal.Purchase, "amnesia", 20, day(0)),
			event(journal.Purchase, "lemon-cookie", 20, day(0)),
		})

		assert.Equal(t, []string{"amnesia", "lemon-cookie", "wedding-cake"}, s.Products(),
			"Should be stable rather than following map order")
	})
}

func TestCycles(t *testing.T) {
	t.Run("APurchaseIntoAnEmptyStorageOpensACycle", func(t *testing.T) {
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 2, day(0)),
			event(journal.Grind, "wedding-cake", 1, day(1)),
		})

		require.Len(t, s.Cycles, 1)
		c := &s.Cycles[0]
		assert.True(t, c.Open(), "Should still be running")
		assert.Equal(t, 2.0, c.Purchased, "Should count the fill")
		assert.Equal(t, 1.0, s.FillOnShelf(c), "Should subtract what was ground")
	})

	t.Run("SeveralProductsInOneFillAreOneCycle", func(t *testing.T) {
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(0)),
			event(journal.Purchase, "lemon-cookie", 20, day(0)),
			event(journal.Purchase, "mac1", 20, day(0)),
		})

		require.Len(t, s.Cycles, 1, "Should not open a cycle per product")
		assert.Equal(t, 60.0, s.Cycles[0].Purchased, "Should total the fill")
		assert.Len(t, s.Cycles[0].Products, 3, "Should list every product in the fill")
	})

	t.Run("ClosesWhenItsOwnJarsAreEmpty", func(t *testing.T) {
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 2, day(0)),
			event(journal.Grind, "wedding-cake", 1, day(1)),
			event(journal.Grind, "wedding-cake", 1, day(2)),
		})

		require.Len(t, s.Cycles, 1)
		assert.False(t, s.Cycles[0].Open(), "Should close itself when the last of its grams is ground")
		assert.Equal(t, day(2), s.Cycles[0].End, "Should date the close to the grind that emptied it")
		assert.Nil(t, s.CurrentCycle(), "Nothing on the shelf, no cycle in progress")
	})

	t.Run("TheNextFillOpensTheNextCycle", func(t *testing.T) {
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 1, day(0)),
			event(journal.Grind, "wedding-cake", 1, day(1)),
			event(journal.Purchase, "lemon-cookie", 1, day(30)),
		})

		require.Len(t, s.Cycles, 2, "Should start a new cycle after the previous one ran to zero")
		assert.Equal(t, day(30), s.Cycles[1].Start, "Should start on the day of the fill")
		assert.True(t, s.Cycles[1].Open(), "Should be the cycle in progress")
	})

	t.Run("ATopUpJoinsTheRunningCycle", func(t *testing.T) {
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 10, day(0)),
			event(journal.Grind, "wedding-cake", 1, day(1)),
			event(journal.Purchase, "lemon-cookie", 5, day(2)),
		})

		require.Len(t, s.Cycles, 1, "Should not split a cycle that still has stock")
		assert.Equal(t, 15.0, s.Cycles[0].Purchased, "Should add the top-up to the running cycle")
	})

	t.Run("SurvivesManyCycles", func(t *testing.T) {
		// Appending to the cycle slice must not strand the cursor into it.
		var events []journal.Event
		for i := 0; i < 64; i++ {
			events = append(events,
				event(journal.Purchase, "wedding-cake", 1, day(i*30)),
				event(journal.Grind, "wedding-cake", 1, day(i*30+1)),
			)
		}

		s := Fold(events)

		require.Len(t, s.Cycles, 64, "Should record every cycle")
		for i, c := range s.Cycles {
			assert.Equal(t, 1.0, c.Purchased, "Cycle %d should keep its own total", i)
		}
		for i, c := range s.Cycles[:len(s.Cycles)-1] {
			assert.False(t, c.Open(), "Cycle %d should have been closed by its successor", i)
		}
	})
}

func TestSummarise(t *testing.T) {
	t.Run("AveragesOverDaysWithAnAmount", func(t *testing.T) {
		st := Summarise([]journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(0)),
			event(journal.Grind, "wedding-cake", 1.0, day(0)),
			event(journal.Grind, "wedding-cake", 0.5, day(0)),
			event(journal.Grind, "wedding-cake", 1.5, day(2)),
		})

		assert.Equal(t, 3.0, st.Ground, "Should total the ground amount only")
		assert.Equal(t, 2, st.ActiveDays, "Should count days with an amount, not events")
		assert.Equal(t, 3, st.ElapsedDays, "Should count calendar days from first to last")
		assert.Equal(t, 1.5, st.PerActiveDay, "Should average over active days")
		assert.Equal(t, 1.0, st.PerElapsedDay, "Should average over elapsed days")
	})

	t.Run("IgnoresNonGrindEvents", func(t *testing.T) {
		st := Summarise([]journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(0)),
			event(journal.Sesh, "wedding-cake", 5, day(0)),
		})

		assert.Zero(t, st.Ground, "Should not count a purchase or a consume as grinding")
		assert.Zero(t, st.ActiveDays, "Should have no active days")
	})

	t.Run("Median", func(t *testing.T) {
		st := Summarise([]journal.Event{
			event(journal.Grind, "wedding-cake", 1.0, day(0)),
			event(journal.Grind, "wedding-cake", 5.0, day(1)),
			event(journal.Grind, "wedding-cake", 2.0, day(2)),
		})

		assert.Equal(t, 2.0, st.MedianPerDay, "Should not be skewed by the outlier the mean follows")
	})

	t.Run("EmptyJournal", func(t *testing.T) {
		st := Summarise(nil)

		assert.Zero(t, st.ActiveDays, "Should have no active days")
		assert.Zero(t, st.DaysLeft(60), "Should not extrapolate without a rate")
	})
}

func TestDaysLeft(t *testing.T) {
	st := Summarise([]journal.Event{
		event(journal.Grind, "wedding-cake", 2.0, day(0)),
		event(journal.Grind, "wedding-cake", 2.0, day(1)),
	})

	assert.Equal(t, 2.0, st.PerActiveDay, "Should average two grams a day")
	assert.Equal(t, 10.0, st.DaysLeft(20), "Should estimate the supply at the observed rate")
}

func TestCycleGap(t *testing.T) {
	t.Run("ALaterFillLeavesTheUnfinishedCycleOpen", func(t *testing.T) {
		// 13 of 47 spreadsheet cycles ended with a remainder rather than at
		// zero. The remainder stays on its own cycle's account, and that
		// cycle stays open beside the new one.
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(0)),
			event(journal.Grind, "wedding-cake", 15, day(10)),
			event(journal.Purchase, "lemon-cookie", 20, day(30)),
		})

		require.Len(t, s.Cycles, 2, "Should not merge the next fill into an unfinished cycle")
		assert.True(t, s.Cycles[0].Open(), "Should stay open while its own jar holds something")
		assert.Equal(t, 20.0, s.Cycles[1].Purchased, "Should not move the remainder onto the new cycle")
		assert.Equal(t, 5.0, s.Cycles[1].Carried, "Should note what stood on the shelf at the fill")
		assert.Equal(t, 5.0, s.Cycles[1].Opening["wedding-cake"], "Should know which jar held it")

		c := &s.Cycles[1]
		assert.Equal(t, 20.0, s.FillOnShelf(c), "The new fill is untouched")
		carried, jars, open := s.CarriedOnShelf(c)
		assert.Equal(t, 5.0, carried, "The remainder stands on the old cycle's account")
		assert.Equal(t, 1, jars)
		assert.Equal(t, 1, open, "and keeps that one cycle open")
	})

	t.Run("GrindingASharedJarDrawsTheOldestGramsFirst", func(t *testing.T) {
		// The same product refilled before its jar was empty: the jar holds
		// two cycles' grams, and no scale can say whose leave first — so the
		// oldest do. The grind below takes the first cycle's 6 g remainder,
		// closing it, then 6 g of the new fill.
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 10, day(0)),
			event(journal.Grind, "wedding-cake", 4, day(5)),
			event(journal.Purchase, "wedding-cake", 10, day(30)),
			event(journal.Grind, "wedding-cake", 12, day(35)),
		})

		require.Len(t, s.Cycles, 2)
		assert.False(t, s.Cycles[0].Open(), "The old cycle's grams are gone")
		assert.Equal(t, day(35), s.Cycles[0].End, "closed by the grind that finished them")
		c := &s.Cycles[1]
		assert.True(t, c.Open())
		assert.Equal(t, 4.0, s.FillOnShelf(c), "6 of the new 10 went after the old 6")
		assert.Equal(t, 4.0, s.ShareOf(c, "wedding-cake"), "and the share never reads over its fill")
	})

	t.Run("TheFillIsNotTheShelf", func(t *testing.T) {
		// The dashboard once billed the whole shelf to the current cycle: a
		// fill of 40 read as "of 51" because eleven grams of older jars were
		// still on the shelf, and the headline summed jars the card never
		// listed. Every gram stands on the account of the cycle that
		// dispensed it: the fill counts its own, the older cycles keep
		// theirs, and a shared jar splits along its fills.
		s := Fold([]journal.Event{
			event(journal.Purchase, "old-strain", 10, day(0)),
			event(journal.Purchase, "wedding-cake", 10, day(0)),
			event(journal.Grind, "old-strain", 2, day(5)),
			event(journal.Grind, "wedding-cake", 7, day(6)),
			// The next fill: a new product, and wedding-cake again with 3 g
			// still in its jar. old-strain is not refilled.
			event(journal.Purchase, "lemon-cookie", 20, day(30)),
			event(journal.Purchase, "wedding-cake", 20, day(30)),
			event(journal.Grind, "lemon-cookie", 5, day(35)),
			event(journal.Grind, "wedding-cake", 4, day(36)),
			event(journal.Grind, "old-strain", 1, day(37)),
		})

		require.Len(t, s.Cycles, 2)
		c := &s.Cycles[1]
		assert.Equal(t, 51.0, c.Held(), "The shelf at the fill: 40 dispensed + 11 standing")
		assert.Equal(t, 40.0, c.Purchased, "The fill is what it dispensed, nothing more")

		// The day-36 grind of 4 g took the old jar-share of 3 g first, then
		// 1 g of the new fill: 20 - 1 = 19 for wedding-cake, 15 for lemon.
		assert.Equal(t, 19.0, s.ShareOf(c, "wedding-cake"), "The shared jar drains oldest first")
		assert.Equal(t, 34.0, s.FillOnShelf(c), "19 + 15 of the fill still standing")

		carried, jars, open := s.CarriedOnShelf(c)
		assert.Equal(t, 7.0, carried, "old-strain's remainder stays the earlier cycle's business")
		assert.Equal(t, 1, jars, "and it sits in one jar")
		assert.Equal(t, 1, open, "keeping that first cycle open")
		assert.True(t, s.Cycles[0].Open(), "the cycle with grams standing is not finished")
	})

	t.Run("SameDayFillsStayTogether", func(t *testing.T) {
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(0)),
			event(journal.Purchase, "lemon-cookie", 20, day(0)),
			event(journal.Purchase, "mac1", 20, day(1)),
		})

		require.Len(t, s.Cycles, 1, "Should keep one prescription's products in one cycle")
		assert.Equal(t, 60.0, s.Cycles[0].Purchased, "Should total the fill")
	})

	t.Run("TheTightestRealBoundaryStillSplits", func(t *testing.T) {
		// 2024-12-1 and 2024-12-2 were eight days apart.
		s := Fold([]journal.Event{
			event(journal.Purchase, "mac1", 10, day(0)),
			event(journal.Purchase, "white-widow", 15, day(8)),
		})

		assert.Len(t, s.Cycles, 2, "Should split two fills eight days apart")
	})
}
