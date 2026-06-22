package model

import (
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
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
	require.NoError(t, DB.AutoMigrate(&FusionUpstreamTemplate{}, &FusionAPIKey{}, &FusionConfig{}, &FusionResponseState{}))

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

func TestFusionResponseStateSaveLoadAndScope(t *testing.T) {
	setupFusionModelTestDB(t)
	message := dto.Message{
		Role:    "user",
		Content: "read file",
	}
	payload := FusionResponseStatePayload{
		Messages: []dto.Message{message},
		Outputs: []dto.ResponsesOutput{
			{
				Type:   "function_call",
				ID:     "fc_read",
				CallId: "call_read",
				Name:   "read_file",
			},
		},
		CallIDByItemID: map[string]string{"fc_read": "call_read"},
	}
	state := &FusionResponseState{
		ResponseID:       "resp_test",
		UserID:           1,
		TokenID:          10,
		ModelAlias:       "fusion:research",
		ParentResponseID: "",
		ExpiresAt:        time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, state.SetStatePayload(payload))
	require.NoError(t, state.Insert())
	assert.NotContains(t, state.StateCiphertext, "read file")

	loaded, err := GetFusionResponseState("resp_test", 1, 10)
	require.NoError(t, err)
	loadedPayload, err := loaded.GetStatePayload()
	require.NoError(t, err)
	require.Len(t, loadedPayload.Messages, 1)
	assert.Equal(t, "read file", loadedPayload.Messages[0].StringContent())
	assert.Equal(t, "call_read", loadedPayload.CallIDByItemID["fc_read"])

	_, err = GetFusionResponseState("resp_test", 1, 11)
	require.Error(t, err)
}

func TestFusionResponseStateExpiryCleanup(t *testing.T) {
	setupFusionModelTestDB(t)
	expired := &FusionResponseState{
		ResponseID: "resp_expired",
		UserID:     1,
		TokenID:    10,
		ModelAlias: "fusion:research",
		ExpiresAt:  time.Now().Add(-time.Hour).Unix(),
	}
	require.NoError(t, expired.SetStatePayload(FusionResponseStatePayload{Messages: []dto.Message{{Role: "user", Content: "old"}}}))
	require.NoError(t, expired.Insert())
	active := &FusionResponseState{
		ResponseID: "resp_active",
		UserID:     1,
		TokenID:    10,
		ModelAlias: "fusion:research",
		ExpiresAt:  time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, active.SetStatePayload(FusionResponseStatePayload{Messages: []dto.Message{{Role: "user", Content: "new"}}}))
	require.NoError(t, active.Insert())

	_, err := GetFusionResponseState("resp_expired", 1, 10)
	require.Error(t, err)
	deleted, err := DeleteExpiredFusionResponseStates(time.Now().Unix())
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	_, err = GetFusionResponseState("resp_active", 1, 10)
	require.NoError(t, err)
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
