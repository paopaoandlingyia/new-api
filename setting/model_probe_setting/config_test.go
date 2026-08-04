package model_probe_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsModelProbedUsesExplicitCaseInsensitiveAllowlist(t *testing.T) {
	original := modelProbeSetting
	original.ProbedModels = append([]string(nil), modelProbeSetting.ProbedModels...)
	t.Cleanup(func() { modelProbeSetting = original })

	modelProbeSetting.ProbedModels = []string{" gpt-4o ", "GPT-4O", ""}

	assert.True(t, IsModelProbed("GPT-4O"))
	assert.False(t, IsModelProbed("claude-sonnet"))
	assert.Equal(t, []string{"gpt-4o"}, GetProbedModels())
}

func TestEmptyProbedModelListDisablesModelProbing(t *testing.T) {
	original := modelProbeSetting
	original.ProbedModels = append([]string(nil), modelProbeSetting.ProbedModels...)
	t.Cleanup(func() { modelProbeSetting = original })

	modelProbeSetting.ProbedModels = nil

	require.Empty(t, GetProbedModels())
	assert.False(t, IsModelProbed("gpt-4o"))
}
