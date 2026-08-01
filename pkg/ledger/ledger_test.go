package ledger

import (
	"testing"
	"time"

	"github.com/TheDonDope/wits-tui/pkg/journal"
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
		assert.Equal(t, 0.25, b.Stash, "Should leave the unconsumed remainder in the tin")
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
		assert.Equal(t, 20.0, round(b.Storage+b.Stash+b.Consumed+b.AVB), "Should account for every purchased gram")
	})

	t.Run("KeepsTinsSeparate", func(t *testing.T) {
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
		c := s.Cycles[0]
		assert.True(t, c.Open(), "Should still be running")
		assert.Equal(t, 2.0, c.Purchased, "Should count the fill")
		assert.Equal(t, 1.0, c.Remaining(), "Should subtract what was ground")
		assert.Equal(t, 0.5, c.RemainingPct(), "Should report half the cycle left")
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

	t.Run("StaysOpenUntilTheNextFill", func(t *testing.T) {
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 2, day(0)),
			event(journal.Grind, "wedding-cake", 1, day(1)),
			event(journal.Grind, "wedding-cake", 1, day(2)),
		})

		require.Len(t, s.Cycles, 1)
		assert.True(t, s.Cycles[0].Open(), "Should stay open even with storage empty, until the next fill")
		assert.Zero(t, s.Cycles[0].Remaining(), "Should report nothing left")
		assert.NotNil(t, s.CurrentCycle(), "Should still be the cycle you are in")
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
	t.Run("ALaterFillStartsANewCycleEvenWithALeftover", func(t *testing.T) {
		// 13 of 47 spreadsheet cycles ended with a remainder rather than at zero.
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(0)),
			event(journal.Grind, "wedding-cake", 15, day(10)),
			event(journal.Purchase, "lemon-cookie", 20, day(30)),
		})

		require.Len(t, s.Cycles, 2, "Should not merge the next fill into an unfinished cycle")
		assert.Equal(t, 5.0, s.Cycles[0].Leftover, "Should record what was left over")
		assert.Equal(t, day(30), s.Cycles[0].End, "Should close when the next fill arrived")
		assert.False(t, s.Cycles[0].Open(), "Should be closed by its successor")
		assert.Equal(t, 20.0, s.Cycles[1].Purchased, "Should not carry the leftover into the new cycle's total")
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
