package model

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFusionModelTestDB(t *testing.T) {
	originalDB := DB
	originalSecret := common.CryptoSecret
	originalConfigured := common.PersistentCryptoSecretConfigured
	t.Cleanup(func() {
		DB = originalDB
		common.CryptoSecret = originalSecret
		common.PersistentCryptoSecretConfigured = originalConfigured
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, DB.AutoMigrate(&FusionUpstreamTemplate{}, &FusionAPIKey{}, &FusionConfig{}))

	common.CryptoSecret = "test-secret-with-enough-entropy"
	common.PersistentCryptoSecretConfigured = true
}

func createFusionTestKey(t *testing.T, userId int, name string, models []string) *FusionAPIKey {
	t.Helper()
	key := &FusionAPIKey{
		UserId:       userId,
		Name:         name,
		Provider:     FusionProviderOpenAICompatible,
		BaseURL:      "https://example.com/v1",
		DefaultModel: "gpt-4o-mini",
		Status:       FusionKeyStatusEnabled,
	}
	require.NoError(t, key.SetPlainAPIKey("sk-"+name))
	require.NoError(t, key.SetModels(models))
	key.Normalize()
	require.NoError(t, DB.Create(key).Error)
	return key
}

func createFusionTestConfig(t *testing.T, userId int, candidateKeyIDs []int, judgeKeyID int, candidateModels map[string]string, judgeModel string) *FusionConfig {
	t.Helper()
	config := &FusionConfig{
		UserId:       userId,
		Name:         "Research Fusion",
		ModelAlias:   "fusion:research",
		Enabled:      true,
		JudgeKeyID:   judgeKeyID,
		JudgeModel:   judgeModel,
		Strategy:     FusionStrategySynthesize,
		TimeoutMS:    45000,
		MaxParallel:  2,
		MinSuccesses: 1,
	}
	require.NoError(t, config.SetCandidateKeyIDs(candidateKeyIDs))
	require.NoError(t, config.SetCandidateModels(candidateModels))
	return config
}

func TestFusionAPIKeyOwnership(t *testing.T) {
	setupFusionModelTestDB(t)
	userOneKey := createFusionTestKey(t, 1, "user-one", []string{"gpt-4o-mini"})
	userTwoKey := createFusionTestKey(t, 2, "user-two", []string{"gpt-4o-mini"})

	got, err := GetFusionAPIKeyByUserAndId(1, userOneKey.Id)
	require.NoError(t, err)
	assert.Equal(t, userOneKey.Id, got.Id)
	assert.Equal(t, "https://example.com/v1", got.BaseURL)

	_, err = GetFusionAPIKeyByUserAndId(1, userTwoKey.Id)
	require.Error(t, err)
}

func TestFusionAPIKeyStoresNormalizedBaseURL(t *testing.T) {
	setupFusionModelTestDB(t)
	key := createFusionTestKey(t, 1, "normalized", []string{"gpt-4o-mini"})

	got, err := GetFusionAPIKeyByUserAndId(1, key.Id)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/v1", got.BaseURL)
}

func TestFusionUpstreamConfigRejectsSensitiveOverrides(t *testing.T) {
	_, err := ParseFusionUpstreamConfigJSON(`{"headers":{"Authorization":"Bearer bad"}}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "headers")

	_, err = ParseFusionUpstreamConfigJSON(`{"query":{"api_key":"bad"}}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query")

	_, err = ParseFusionUpstreamConfigJSON(`{"body_overrides":{"model":"gpt-4o"}}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "body_overrides")
}

func TestFusionConfigValidationRejectsForeignKeys(t *testing.T) {
	setupFusionModelTestDB(t)
	userOneKey := createFusionTestKey(t, 1, "user-one", []string{"gpt-4o-mini"})
	userTwoKey := createFusionTestKey(t, 2, "user-two", []string{"gpt-4o-mini"})

	foreignCandidate := createFusionTestConfig(t, 1, []int{userTwoKey.Id}, userOneKey.Id, nil, "gpt-4o-mini")
	err := ValidateFusionConfigKeyOwnership(1, foreignCandidate)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "candidate key")

	foreignJudge := createFusionTestConfig(t, 1, []int{userOneKey.Id}, userTwoKey.Id, nil, "gpt-4o-mini")
	err = ValidateFusionConfigKeyOwnership(1, foreignJudge)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "judge key")
}

func TestFusionConfigAliasValidation(t *testing.T) {
	assert.NoError(t, ValidateFusionModelAlias("fusion:research"))
	assert.NoError(t, ValidateFusionModelAlias("fusion:research.v1"))
	assert.Error(t, ValidateFusionModelAlias("research"))
	assert.Error(t, ValidateFusionModelAlias("fusion:"))
	assert.Error(t, ValidateFusionModelAlias("fusion:-bad"))
}

func TestFusionConfigModelOverrideRespectsKeyAllowlist(t *testing.T) {
	setupFusionModelTestDB(t)
	key := createFusionTestKey(t, 1, "allowlist", []string{"gpt-4o-mini"})

	config := createFusionTestConfig(t, 1, []int{key.Id}, key.Id, map[string]string{
		strconv.Itoa(key.Id): "gpt-4o",
	}, "gpt-4o-mini")
	err := ValidateFusionConfigKeyOwnership(1, config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "candidate model")

	config = createFusionTestConfig(t, 1, []int{key.Id}, key.Id, nil, "gpt-4o")
	err = ValidateFusionConfigKeyOwnership(1, config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "judge model")

	config = createFusionTestConfig(t, 1, []int{key.Id}, key.Id, nil, "gpt-4o-mini")
	require.NoError(t, ValidateFusionConfigKeyOwnership(1, config))
}
