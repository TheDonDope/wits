package api

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheGeneratedTypesAreCurrent(t *testing.T) {
	written, err := os.ReadFile("../../wits-ui/src/app/data/api.gen.ts")
	require.NoError(t, err)
	assert.Equal(t, TypeScript(), string(written), "api.gen.ts is out of date with pkg/api: run make types")
}
