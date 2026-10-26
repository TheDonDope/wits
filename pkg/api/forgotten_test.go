package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheDonDope/wits/pkg/record"
)

func TestWeighingShortIsPutDownToSomethingForgotten(t *testing.T) {
	s, _ := server(t, "")

	w := call(t, s, "GET", "/api/weigh/forgotten?product=wcake&account=storage&weighed=17", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	plan := decodeInto[forgottenPlan](t, w)
	assert.Equal(t, 18.0, plan.Expected)
	assert.Equal(t, 1.0, plan.Short)
	require.Len(t, plan.Entries, 1, "Should plan a grind for a light jar")
	assert.Equal(t, "grind", plan.Entries[0].Type)

	w = call(t, s, "POST", "/api/weigh/forgotten", `{"product":"wcake","account":"storage","weighed":17}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, "a forgotten grind of 1.00 g of Enua 22/1 Wedding Cake", decodeInto[flushed](t, w).Message)

	// The stash held 1.5 g and now 2.5 g; 1.5 g on the scale is two sessions of the one 0.5 g on record.
	w = call(t, s, "POST", "/api/weigh/forgotten", `{"product":"wcake","account":"stash","weighed":1.5}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	sessions := decodeInto[flushed](t, w)
	assert.Equal(t, "2 forgotten sessions, 1.00 g of Enua 22/1 Wedding Cake", sessions.Message)
	assert.Equal(t, "volcano", sessions.Entries[0].Device, "Should have them as the last session was had")

	effects := decodeInto[effectLog](t, call(t, s, "GET", "/api/effects", ""))
	for _, e := range effects.Unrated {
		assert.NotEqual(t, record.ForgottenNote, e.Note, "Should not ask how a predicted session went")
	}

	w = call(t, s, "GET", "/api/weigh/forgotten?product=wcake&account=stash&weighed=1.5", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Empty(t, decodeInto[forgottenPlan](t, w).Entries, "Should find nothing forgotten once the scale agrees")
}
