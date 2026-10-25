package record

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/journal"
	"github.com/TheDonDope/wits/pkg/prices"
)

func TestFill(t *testing.T) {
	rec := recorder(t)
	_, _, _, err := rec.Buy("Enua 22/1 Wedding Cake", "", 5, time.Now().AddDate(0, 0, -40))
	require.NoError(t, err)

	filled, err := rec.Fill([]FillLine{
		{Name: "Enua 22/1 Wedding Cake", Grams: 20, Cents: 11680},
		{Name: "NICE 32/1 Morning Sun", Grams: 20, Cents: 11600, Note: "invoice 2609"},
		{Name: "Sumo 25/1 Papaya Bomb", Grams: 10},
	}, time.Now(), "invoice 2608")

	require.NoError(t, err)
	require.Len(t, filled, 3)
	assert.False(t, filled[0].Added, "A product it knows")
	assert.True(t, filled[1].Added, "and ones it does not")
	assert.Equal(t, 25.0, rec.Available("wcake-221", journal.Storage))
	assert.Equal(t, 20.0, rec.Available(filled[1].Product.Slug, journal.Storage))
	paid, err := rec.repo.Prices().All()
	require.NoError(t, err)
	latest := prices.Latest(paid)
	assert.Len(t, latest, 2, "A price for each line that has one")
	assert.Equal(t, int64(11680), latest[filled[0].Event.Hash].Cents)
	assert.Equal(t, "invoice 2608", latest[filled[0].Event.Hash].Note, "The fill's note")
	assert.Equal(t, "invoice 2609", latest[filled[1].Event.Hash].Note, "unless the line has its own")
}

func TestFillIsAllOrNothing(t *testing.T) {
	rec := recorder(t)

	_, err := rec.Fill([]FillLine{
		{Name: "NICE 32/1 Morning Sun", Grams: 20},
		{Name: "Sumo 25/1 Papaya Bomb", Grams: 0},
	}, time.Now(), "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "line 2")
	events, err := rec.repo.Journal().Events()
	require.NoError(t, err)
	assert.Empty(t, events, "Nothing recorded")
	assert.Empty(t, rec.products.Products, "and no product added for the line that was fine")

	_, err = rec.Fill(nil, time.Now(), "")
	assert.ErrorIs(t, err, ErrEmptyFill)
	_, err = rec.Fill([]FillLine{{Name: "X 22/1 Y", Grams: 1}}, time.Now().Add(time.Hour), "")
	assert.ErrorIs(t, err, ErrAhead)
}
