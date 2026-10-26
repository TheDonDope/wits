package catalog

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStampMarksWhatAChangeChanged(t *testing.T) {
	then := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	now := then.Add(time.Hour)
	before := &Catalog{Products: []*Product{{Slug: "w", Name: "Wedding Cake", Changed: Changed{"name": then}}}}
	after := &Catalog{Products: []*Product{
		{Slug: "w", Name: "Wedding Cake", Terpenes: []string{"Myrcene"}, Changed: Changed{"name": then}},
		{Slug: "new", Name: "New", THC: 22},
		{Slug: "pulled", Name: "Pulled", Changed: Changed{"name": then}},
	}}

	StampProducts(before, after, now)

	assert.Equal(t, Changed{"name": then, "terpenes": now}, after.Products[0].Changed, "Should stamp only what changed")
	assert.Equal(t, Changed{"name": now, "thc": now}, after.Products[1].Changed, "Should stamp a new product's fields")
	assert.Equal(t, Changed{"name": then}, after.Products[2].Changed, "Should keep the stamps a product came with")
}

func TestMergeTakesTheLaterChangeFieldByField(t *testing.T) {
	early := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	late := early.Add(time.Hour)
	ours := &Product{Slug: "w", Name: "Ours", Terpenes: []string{"Myrcene"}, Changed: Changed{"name": early, "terpenes": late}}
	theirs := &Product{Slug: "w", Name: "Theirs", Changed: Changed{"name": late}}

	merged, took := MergeProduct(ours, theirs)

	assert.True(t, took)
	assert.Equal(t, "Theirs", merged.Name, "Should take their later name")
	assert.Equal(t, []string{"Myrcene"}, merged.Terpenes, "Should keep our later terpenes")
	assert.Equal(t, Changed{"name": late, "terpenes": late}, merged.Changed)
	assert.Equal(t, "Ours", ours.Name, "Should leave ours as it was")

	_, took = MergeProduct(merged, theirs)
	assert.False(t, took, "Should take nothing the second time")
}

func TestMergeWithoutStampsTakesTheirs(t *testing.T) {
	merged, took := MergeDevice(&Device{Slug: "v", Name: "Ours"}, &Device{Slug: "v", Name: "Theirs"})
	assert.True(t, took)
	assert.Equal(t, "Theirs", merged.Name, "Should do as every pull did before fields had stamps")
}
