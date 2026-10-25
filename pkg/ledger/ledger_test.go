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
		assert.Equal(t, Fill(2.0), c.Purchased, "Should count the fill")
		assert.Equal(t, Fill(1.0), s.FillOnShelf(c), "Should subtract what was ground")
	})

	t.Run("SeveralProductsInOneFillAreOneCycle", func(t *testing.T) {
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(0)),
			event(journal.Purchase, "lemon-cookie", 20, day(0)),
			event(journal.Purchase, "mac1", 20, day(0)),
		})

		require.Len(t, s.Cycles, 1, "Should not open a cycle per product")
		assert.Equal(t, Fill(60.0), s.Cycles[0].Purchased, "Should total the fill")
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
		assert.Nil(t, s.CycleAt(day(1000)), "Nothing on the shelf, no cycle in progress")
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
		assert.Equal(t, Fill(15.0), s.Cycles[0].Purchased, "Should add the top-up to the running cycle")
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
			assert.Equal(t, Fill(1.0), c.Purchased, "Cycle %d should keep its own total", i)
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
		assert.Zero(t, st.DaysWorth(60), "Should not extrapolate without a rate")
	})
}

func TestDaysWorth(t *testing.T) {
	st := Summarise([]journal.Event{
		event(journal.Grind, "wedding-cake", 2.0, day(0)),
		event(journal.Grind, "wedding-cake", 2.0, day(1)),
	})

	assert.Equal(t, 2.0, st.PerActiveDay, "Should average two grams a day")
	assert.Equal(t, 10.0, st.DaysWorth(20), "Should estimate the supply at the observed rate")
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
		assert.Equal(t, Fill(20.0), s.Cycles[1].Purchased, "Should not move the remainder onto the new cycle")
		assert.Equal(t, Shelf(5.0), s.Cycles[1].Carried, "Should note what stood on the shelf at the fill")
		assert.Equal(t, Shelf(5.0), s.Cycles[1].Opening["wedding-cake"], "Should know which jar held it")

		c := &s.Cycles[1]
		assert.Equal(t, Fill(20.0), s.FillOnShelf(c), "The new fill is untouched")
		carried, jars, open := s.CarriedOnShelf(c)
		assert.Equal(t, Shelf(5.0), carried, "The remainder stands on the old cycle's account")
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
		assert.Equal(t, Fill(4.0), s.FillOnShelf(c), "6 of the new 10 went after the old 6")
		assert.Equal(t, Fill(4.0), s.ShareOf(c, "wedding-cake"), "and the share never reads over its fill")
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
		assert.Equal(t, Shelf(51.0), c.Held(), "The shelf at the fill: 40 dispensed + 11 standing")
		assert.Equal(t, Fill(40.0), c.Purchased, "The fill is what it dispensed, nothing more")

		// The day-36 grind of 4 g took the old jar-share of 3 g first, then
		// 1 g of the new fill: 20 - 1 = 19 for wedding-cake, 15 for lemon.
		assert.Equal(t, Fill(19.0), s.ShareOf(c, "wedding-cake"), "The shared jar drains oldest first")
		assert.Equal(t, Fill(34.0), s.FillOnShelf(c), "19 + 15 of the fill still standing")

		carried, jars, open := s.CarriedOnShelf(c)
		assert.Equal(t, Shelf(7.0), carried, "old-strain's remainder stays the earlier cycle's business")
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
		assert.Equal(t, Fill(60.0), s.Cycles[0].Purchased, "Should total the fill")
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

func TestFoldsByDate(t *testing.T) {
	t.Run("ABackdatedFillOpensItsOwnCycle", func(t *testing.T) {
		// Recorded last, dated first: the fill typed in a month late.
		s := Fold([]journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(30)),
			event(journal.Grind, "wedding-cake", 1, day(31)),
			event(journal.Purchase, "lemon-cookie", 10, day(0)),
		})

		require.Len(t, s.Cycles, 2, "A fill dated a month earlier is a cycle of its own")
		assert.Equal(t, day(0), s.Cycles[0].Start)
		assert.Equal(t, Fill(10.0), s.Cycles[0].Purchased, "The early fill should not join the later cycle")
		assert.Equal(t, Fill(20.0), s.Cycles[1].Purchased)
		assert.Equal(t, Shelf(10.0), s.Cycles[1].Carried, "The later cycle arrived to the early fill still standing")
	})

	t.Run("EventsReadInTheOrderTheyHappened", func(t *testing.T) {
		late := event(journal.Purchase, "lemon-cookie", 10, day(0))
		late.Seq = 3
		first := event(journal.Purchase, "wedding-cake", 20, day(5))
		first.Seq = 1
		same := event(journal.Grind, "wedding-cake", 1, day(5))
		same.Seq = 2

		s := Fold([]journal.Event{first, same, late})

		var seqs []int
		for _, e := range s.Events {
			seqs = append(seqs, e.Seq)
		}
		assert.Equal(t, []int{3, 1, 2}, seqs, "By date, and in recorded order within the same moment")
	})

	t.Run("NamesAnEntryWhoseGramsWereNotThereYet", func(t *testing.T) {
		events := []journal.Event{
			event(journal.Purchase, "wedding-cake", 20, day(5)),
			event(journal.Grind, "wedding-cake", 2, day(1)),
		}

		assert.Equal(t, 18.0, Fold(events).Balances["wedding-cake"].Storage, "Today's balance is fine")
		short := Shortfalls(events)
		require.Len(t, short, 1, "But on day 1 the grams had not arrived")
		assert.Equal(t, journal.Storage, short[0].Account)
		assert.Equal(t, 0.0, short[0].Held, "The jar held nothing then — not less than nothing")
	})
}

func TestCycleAt(t *testing.T) {
	s := Fold([]journal.Event{
		event(journal.Purchase, "wedding-cake", 20, day(0)),
		event(journal.Grind, "wedding-cake", 1, day(1)),
		event(journal.Purchase, "lemon-cookie", 10, day(60)),
	})

	require.Len(t, s.Cycles, 2)
	assert.Nil(t, s.CycleAt(day(-1)), "Nothing had been dispensed yet")
	assert.Equal(t, 0, s.CycleAt(day(5)).Seq, "A later fill is not in progress before it was dispensed")
	assert.Equal(t, 1, s.CycleAt(day(60)).Seq, "and is from the day it does")

	emptied := Fold([]journal.Event{
		event(journal.Purchase, "wedding-cake", 2, day(0)),
		event(journal.Grind, "wedding-cake", 2, day(1)),
		event(journal.Purchase, "lemon-cookie", 10, day(60)),
	})
	assert.Nil(t, emptied.CycleAt(day(5)), "With the shelf empty and the next fill not here yet, nothing is in progress")
}

func TestParseGrams(t *testing.T) {
	for in, want := range map[string]float64{"0.75": 0.75, "0.75g": 0.75, "0,75 g": 0.75, " 20G ": 20, "0": 0} {
		got, err := ParseGrams(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	_, err := ParseGrams("0.333")
	assert.ErrorContains(t, err, "finer than a scale reads")
	for _, in := range []string{"lots", "", "NaN", "Inf"} {
		_, err := ParseGrams(in)
		assert.ErrorContains(t, err, "is not an amount in grams", in)
	}
}

func TestFoldAddsInCentigrams(t *testing.T) {
	// Thirty fills of 0.1 g add up to 3.0000000000000013 in floating point.
	var events []journal.Event
	for i := 0; i < 30; i++ {
		events = append(events, event(journal.Purchase, "wedding-cake", 0.1, day(0)))
	}
	events = append(events, event(journal.Grind, "wedding-cake", 3, day(1)))

	s := Fold(events)

	assert.Equal(t, 0.0, s.Balances["wedding-cake"].Storage, "Exactly nothing should be left")
	assert.Equal(t, Fill(3.0), s.Cycles[0].Purchased)
	assert.False(t, s.Cycles[0].Open(), "and the fill should close, not hang open on a rounding remainder")
}

func TestFillGround(t *testing.T) {
	s := Fold([]journal.Event{
		event(journal.Purchase, "wedding-cake", 10, day(0)),
		event(journal.Grind, "wedding-cake", 6, day(1)),
		event(journal.Purchase, "wedding-cake", 20, day(30)),
		event(journal.Grind, "wedding-cake", 5, day(31)), // 4 from the old fill, 1 from the new
	})
	fill := &s.Cycles[1]

	assert.Equal(t, Fill(19), s.ShareOf(fill, "wedding-cake"), "Grams drain oldest first")
	assert.Equal(t, 11.0, s.Balances["wedding-cake"].Stash, "The stash holds both fills' ground grams")
	stash, avb := s.FillGround(fill, "wedding-cake")
	assert.Equal(t, Fill(1), stash, "Only 1 g of it can be this fill's — it used to read all 11, past the fill's 20")
	assert.Equal(t, Fill(0), avb)
	assert.Equal(t, Fill(20), s.ShareOf(fill, "wedding-cake")+stash+avb, "and the row adds up to what the fill dispensed")
}

func TestShortfallSaysWhatTheJarHeld(t *testing.T) {
	short := Shortfalls([]journal.Event{
		event(journal.Purchase, "wedding-cake", 1, day(0)),
		event(journal.Grind, "wedding-cake", 5, day(1)),
	})
	require.Len(t, short, 1)
	assert.Equal(t, "on 2026-07-02, grind 5.00 g of wedding-cake took more than storage held — 1.00 g", short[0].String())
}

// clash is a 2 g fill ground to nothing twice, as two devices pulled into one
// journal record it: the second grind took grams that were not there.
func clash() []journal.Event {
	fill := event(journal.Purchase, "wedding-cake", 2, day(0))
	deck := event(journal.Grind, "wedding-cake", 2, day(1))
	phone := event(journal.Grind, "wedding-cake", 1.5, day(1))
	fill.Hash, deck.Hash, phone.Hash = "fill", "deck", "phone"
	return []journal.Event{fill, deck, phone}
}

func TestAClashIsUndone(t *testing.T) {
	events := clash()
	require.Len(t, Shortfalls(events), 1, "The second grind takes grams that were not there")
	undo := journal.Event{Type: journal.Adjust, Product: "wedding-cake", Grams: 1.5,
		From: journal.Stash, To: journal.Storage, OccurredAt: day(2), Reverts: "phone"}
	events = append(events, undo)

	assert.Empty(t, Shortfalls(events), "An undone entry and its undo moved nothing between them")
	s := Fold(events)
	assert.Equal(t, 0.0, s.Balances["wedding-cake"].Storage, "The jar is empty, as both grinds left it")
	assert.Equal(t, Fill(0), s.FillOnShelf(&s.Cycles[0]),
		"The undo puts back only what the grind took from the fill — it took nothing, and the fill read 1.5 g")
}

func TestPlanRefill(t *testing.T) {
	s := Fold([]journal.Event{
		event(journal.Purchase, "wedding-cake", 20, day(0)),
		event(journal.Grind, "wedding-cake", 2, day(1)),
		event(journal.Grind, "wedding-cake", 1, day(2)),
	})
	c := &s.Cycles[0]

	plan, ok := PlanRefill(c, day(2), 3)

	require.True(t, ok)
	assert.Equal(t, Shelf(17), plan.Shelf, "The shelf is what the fill brought, less every grind")
	assert.Equal(t, 1.5, plan.Rate, "At the fill's rate, per day with grinding")
	assert.Equal(t, midnight(day(14)), plan.EmptyOn, "17 g at 1.5 g a day runs out on the 12th day from now")
	assert.Equal(t, midnight(day(11)), plan.OrderBy, "Three days before that")
	assert.False(t, plan.Overdue(day(11)), "The day to order by itself is not late")
	assert.True(t, plan.Overdue(day(12)), "The day after it is")

	_, ok = PlanRefill(nil, day(2), 3)
	assert.False(t, ok, "There is nothing to plan without a fill")
	fresh := Fold([]journal.Event{event(journal.Purchase, "wedding-cake", 20, day(0))})
	_, ok = PlanRefill(&fresh.Cycles[0], day(0), 3)
	assert.False(t, ok, "Nor without a rate")
}

func TestSplit(t *testing.T) {
	shares := Split(21, map[string]journal.Centigrams{"morning": 30, "papaya": 45, "none": 0})
	assert.Equal(t, map[string]journal.Centigrams{"morning": 8, "papaya": 13}, shares,
		"21 cg in proportion to 30 and 45, the centigram left over to the larger remainder")
	assert.Equal(t, map[string]journal.Centigrams{"a": 1, "b": 1, "c": 1},
		Split(3, map[string]journal.Centigrams{"a": 1, "b": 1, "c": 1}), "all of it, when it all comes back")
	assert.Empty(t, Split(5, nil), "nothing to share between")
}

func TestYields(t *testing.T) {
	sesh := func(product string, g float64, n int, device string) journal.Event {
		e := event(journal.Sesh, product, g, day(n))
		e.Device = device
		return e
	}
	collect := func(product string, g float64, n int, device string) journal.Event {
		e := event(journal.AVBCollect, product, g, day(n))
		e.Device = device
		return e
	}
	events := []journal.Event{
		sesh("morning", 0.3, 1, "mighty"), sesh("papaya", 0.45, 1, "mighty"), sesh("papaya", 0.2, 1, "volcano"),
		collect("morning", 0.08, 2, "mighty"), collect("papaya", 0.13, 2, "mighty"),
		sesh("morning", 0.5, 3, "mighty"),
	}

	waiting := Uncollected(events, day(3))
	assert.Len(t, waiting["mighty"], 1, "only the session since the mighty was emptied")
	assert.Len(t, waiting["volcano"], 1, "the volcano has not been")

	y := Yields(events)
	require.Len(t, y, 1, "two entries at one moment from one device are one collection")
	assert.Equal(t, Yield{At: day(2), Device: "mighty", Collected: 0.21, From: 0.75, Sessions: 2}, y[0])
	assert.InDelta(t, 0.28, y[0].Ratio(), 1e-9)
}

// undo is the correction wits revert writes for an entry: the grams moved
// back, dated when the undo was made.
func undo(e journal.Event, at time.Time) journal.Event {
	return journal.Event{Type: journal.Adjust, Product: e.Product, Grams: e.Grams, From: e.To, To: e.From,
		OccurredAt: at, Reverts: e.Hash, Hash: "undo-" + e.Hash}
}

// An undone grind counted in the rate, the shelf and the refill date — every
// undo moved the day to order earlier — though the jar itself was right.
func TestUndoneCountsInNoTotal(t *testing.T) {
	mistake := event(journal.Grind, "wedding-cake", 5, day(3))
	mistake.Hash = "mistake"
	s := Fold([]journal.Event{
		event(journal.Purchase, "wedding-cake", 20, day(0)),
		event(journal.Grind, "wedding-cake", 2, day(1)),
		event(journal.Grind, "wedding-cake", 2, day(2)),
		mistake,
		undo(mistake, day(3).Add(time.Minute)),
	})
	c := &s.Cycles[0]

	assert.Equal(t, 16.0, s.Balances["wedding-cake"].Storage, "The jar was always right")
	assert.Len(t, s.Events, 5, "Every entry is still in the fold")
	assert.Len(t, s.Standing, 3, "but neither half of the undo stands")
	assert.Equal(t, Shelf(4), c.Ground, "The fill's grinding leaves out the undone 5 g")
	assert.Equal(t, 2.0, Summarise(c.Standing).PerActiveDay, "2 g a day on 2 days, not 3 g on 3")

	plan, ok := PlanRefill(c, day(3), 3)
	require.True(t, ok)
	assert.Equal(t, Shelf(16), plan.Shelf, "The shelf the refill counts is the jar's 16 g, not 11 g")
	assert.Equal(t, midnight(day(11)), plan.EmptyOn, "16 g at 2 g a day: eight days on")

	p := Project(c, day(3))
	assert.Equal(t, Shelf(16), p.History[len(p.History)-1].Shelf, "The curve starts from the same 16 g")
	assert.Equal(t, plan.EmptyOn, p.EmptyOn, "and ends where the refill planner says")
	assert.Equal(t, Shelf(0), p.Projected[len(p.Projected)-1].Shelf)
}

// An undo is dated when it was made, which can be inside another fill's
// tenure: the fill the undone entry fell in must still leave it out.
func TestAnUndoInALaterFill(t *testing.T) {
	mistake := event(journal.Grind, "wedding-cake", 5, day(2))
	mistake.Hash = "mistake"
	s := Fold([]journal.Event{
		event(journal.Purchase, "wedding-cake", 10, day(0)),
		event(journal.Grind, "wedding-cake", 1, day(1)),
		mistake,
		event(journal.Purchase, "lemon-tartz", 10, day(20)),
		undo(mistake, day(21)),
	})
	require.Len(t, s.Cycles, 2)

	assert.Equal(t, Shelf(1), s.Cycles[0].Ground, "The first fill's grinding, without the grind undone later")
	assert.Equal(t, 1.0, Summarise(s.Cycles[0].Standing).Ground)
	assert.Empty(t, Summarise(s.Cycles[1].Standing).Ground, "and the undo grinds nothing in the second")
}
