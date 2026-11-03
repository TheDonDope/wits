package wits

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestItems: the items of a list of the version known, and a refusal of
// any other.
func TestItems(t *testing.T) {
	var got []Product
	require.NoError(t, items([]byte(`{"apiVersion":"wits/v1","kind":"ProductList","items":[{"slug":"wcake-221","stash":1.7}]}`), &got))
	assert.Equal(t, []Product{{Slug: "wcake-221", Stash: 1.7}}, got)

	err := items([]byte(`{"apiVersion":"wits/v2","kind":"ProductList","items":[]}`), &got)
	assert.ErrorContains(t, err, `wits printed "wits/v2"`)
	assert.ErrorContains(t, items([]byte(`[]`), &got), "cannot unmarshal array", "a wits from before versions")
}
