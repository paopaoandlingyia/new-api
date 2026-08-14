package model_availability_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSourcesValidatesOperatorConfiguration(t *testing.T) {
	sources, err := ParseSources(`[{"group":" 通用 ","url":"http://claude-relay:8080/availability","token":"secret"}]`)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, "通用", sources[0].Group)
	assert.Equal(t, "http://claude-relay:8080/availability", sources[0].URL)

	for _, value := range []string{
		`{"group":"通用"}`,
		`[{"group":"","url":"https://example.com/availability"}]`,
		`[{"group":"通用","url":"file:///tmp/status"}]`,
	} {
		assert.Error(t, ValidateSources(value))
	}
}
