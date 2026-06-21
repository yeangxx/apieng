package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFusionControllerTestDB(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalSecret := common.CryptoSecret
	originalConfigured := common.PersistentCryptoSecretConfigured
	originalRedisEnabled := common.RedisEnabled
	savedConfig := config.GlobalConfig.ExportAllConfigs()
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.CryptoSecret = originalSecret
		common.PersistentCryptoSecretConfigured = originalConfigured
		common.RedisEnabled = originalRedisEnabled
		require.NoError(t, config.GlobalConfig.LoadFromDB(savedConfig))
	})

	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.FusionAPIKey{}, &model.FusionConfig{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	common.CryptoSecret = "test-secret-with-enough-entropy"
	common.PersistentCryptoSecretConfigured = true
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"fusion_setting.enabled":                "false",
		"fusion_setting.allow_private_base_url": "true",
		"fusion_setting.allowed_base_url_ports": `[443]`,
	}))
}

func fusionControllerJSONRequest(t *testing.T, handler gin.HandlerFunc, method string, path string, userId int, body any, params ...gin.Param) *httptest.ResponseRecorder {
	t.Helper()
	data, err := common.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, bytes.NewReader(data))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userId)
	ctx.Params = params
	handler(ctx)
	return recorder
}

func fusionControllerRequest(t *testing.T, handler gin.HandlerFunc, method string, path string, userId int, params ...gin.Param) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, nil)
	ctx.Set("id", userId)
	ctx.Params = params
	handler(ctx)
	return recorder
}

func decodeFusionControllerResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload
}

func createFusionControllerKey(t *testing.T, userId int, name string) *model.FusionAPIKey {
	t.Helper()
	key := &model.FusionAPIKey{
		UserId:       userId,
		Name:         name,
		Provider:     model.FusionProviderOpenAICompatible,
		BaseURL:      "https://127.0.0.1/v1",
		DefaultModel: "gpt-4o-mini",
		Status:       model.FusionKeyStatusEnabled,
	}
	require.NoError(t, key.SetPlainAPIKey("sk-"+name))
	require.NoError(t, key.SetModels([]string{"gpt-4o-mini"}))
	require.NoError(t, key.Insert())
	return key
}

func TestCreateFusionAPIKeyRequiresPersistentCryptoSecret(t *testing.T) {
	setupFusionControllerTestDB(t)
	common.PersistentCryptoSecretConfigured = false

	recorder := fusionControllerJSONRequest(t, CreateFusionAPIKey, http.MethodPost, "/api/fusion/keys", 1, dto.FusionAPIKeyCreateRequest{
		Name:         "Primary",
		Provider:     model.FusionProviderOpenAICompatible,
		BaseURL:      "https://127.0.0.1/v1",
		APIKey:       "sk-secret",
		DefaultModel: "gpt-4o-mini",
		Models:       []string{"gpt-4o-mini"},
	})

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	payload := decodeFusionControllerResponse(t, recorder)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, payload["message"], "CRYPTO_SECRET")

	total, err := model.CountFusionAPIKeysByUserId(1)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestFusionAPIKeyResponseDoesNotExposeSecrets(t *testing.T) {
	setupFusionControllerTestDB(t)

	recorder := fusionControllerJSONRequest(t, CreateFusionAPIKey, http.MethodPost, "/api/fusion/keys", 1, dto.FusionAPIKeyCreateRequest{
		Name:         "Primary",
		Provider:     model.FusionProviderOpenAICompatible,
		BaseURL:      "https://127.0.0.1/v1",
		APIKey:       "sk-visible-secret",
		DefaultModel: "gpt-4o-mini",
		Models:       []string{"gpt-4o-mini"},
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.NotContains(t, body, "sk-visible-secret")
	assert.NotContains(t, body, "api_key_ciphertext")
	assert.NotContains(t, body, "key_fingerprint")
	assert.Contains(t, body, "api_key_hint")

	listRecorder := fusionControllerRequest(t, GetFusionAPIKeys, http.MethodGet, "/api/fusion/keys", 1)
	require.Equal(t, http.StatusOK, listRecorder.Code)
	listBody := listRecorder.Body.String()
	assert.NotContains(t, listBody, "sk-visible-secret")
	assert.NotContains(t, listBody, "api_key_ciphertext")
	assert.NotContains(t, listBody, "key_fingerprint")
}

func TestCreateFusionConfigRejectsForeignKeys(t *testing.T) {
	setupFusionControllerTestDB(t)
	foreignKey := createFusionControllerKey(t, 2, "foreign")

	recorder := fusionControllerJSONRequest(t, CreateFusionConfig, http.MethodPost, "/api/fusion/configs", 1, dto.FusionConfigCreateRequest{
		Name:            "Research",
		ModelAlias:      "fusion:research",
		Enabled:         true,
		CandidateKeyIDs: []int{foreignKey.Id},
		JudgeKeyID:      foreignKey.Id,
		JudgeModel:      "gpt-4o-mini",
		Strategy:        model.FusionStrategySynthesize,
		TimeoutMS:       45000,
		MaxParallel:     1,
		MinSuccesses:    1,
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeFusionControllerResponse(t, recorder)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, payload["message"], "candidate key")
}

func TestCreateFusionConfigReturnsSavedShape(t *testing.T) {
	setupFusionControllerTestDB(t)
	key := createFusionControllerKey(t, 1, "owned")

	recorder := fusionControllerJSONRequest(t, CreateFusionConfig, http.MethodPost, "/api/fusion/configs", 1, dto.FusionConfigCreateRequest{
		Name:            "Research",
		ModelAlias:      "fusion:research",
		Enabled:         true,
		CandidateKeyIDs: []int{key.Id},
		CandidateModels: map[string]string{fmt.Sprintf("%d", key.Id): "gpt-4o-mini"},
		JudgeKeyID:      key.Id,
		JudgeModel:      "gpt-4o-mini",
		Strategy:        model.FusionStrategySynthesize,
		TimeoutMS:       45000,
		MaxParallel:     1,
		MinSuccesses:    1,
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeFusionControllerResponse(t, recorder)
	assert.Equal(t, true, payload["success"])
	body := recorder.Body.String()
	assert.Contains(t, body, "fusion:research")
	assert.Contains(t, body, "candidate_key_ids")
}

func TestDeleteFusionAPIKeyRejectsConfigReferences(t *testing.T) {
	setupFusionControllerTestDB(t)
	key := createFusionControllerKey(t, 1, "referenced")
	config := &model.FusionConfig{
		UserId:       1,
		Name:         "Research",
		ModelAlias:   "fusion:research",
		Enabled:      true,
		JudgeKeyID:   key.Id,
		JudgeModel:   "gpt-4o-mini",
		Strategy:     model.FusionStrategySynthesize,
		TimeoutMS:    45000,
		MaxParallel:  1,
		MinSuccesses: 1,
	}
	require.NoError(t, config.SetCandidateKeyIDs([]int{key.Id}))
	require.NoError(t, config.SetCandidateModels(nil))
	require.NoError(t, config.Insert())

	recorder := fusionControllerRequest(t, DeleteFusionAPIKey, http.MethodDelete, "/api/fusion/keys/1", 1, gin.Param{
		Key:   "id",
		Value: fmt.Sprintf("%d", key.Id),
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeFusionControllerResponse(t, recorder)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, payload["message"], "used by a fusion config")
}

func TestFusionTestEndpointsFailClosedUntilEngineExists(t *testing.T) {
	setupFusionControllerTestDB(t)
	key := createFusionControllerKey(t, 1, "test")
	fusionConfig := &model.FusionConfig{
		UserId:       1,
		Name:         "Research",
		ModelAlias:   "fusion:research",
		Enabled:      true,
		JudgeKeyID:   key.Id,
		JudgeModel:   "gpt-4o-mini",
		Strategy:     model.FusionStrategySynthesize,
		TimeoutMS:    45000,
		MaxParallel:  1,
		MinSuccesses: 1,
	}
	require.NoError(t, fusionConfig.SetCandidateKeyIDs([]int{key.Id}))
	require.NoError(t, fusionConfig.SetCandidateModels(nil))
	require.NoError(t, fusionConfig.Insert())

	disabledRecorder := fusionControllerRequest(t, TestFusionAPIKey, http.MethodPost, "/api/fusion/keys/1/test", 1, gin.Param{
		Key:   "id",
		Value: fmt.Sprintf("%d", key.Id),
	})
	require.Equal(t, http.StatusForbidden, disabledRecorder.Code)
	assert.Contains(t, disabledRecorder.Body.String(), "Fusion is disabled")

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"fusion_setting.enabled": "true",
	}))
	notImplementedRecorder := fusionControllerRequest(t, TestFusionAPIKey, http.MethodPost, "/api/fusion/keys/1/test", 1, gin.Param{
		Key:   "id",
		Value: fmt.Sprintf("%d", key.Id),
	})
	require.Equal(t, http.StatusNotImplemented, notImplementedRecorder.Code)
	assert.Contains(t, notImplementedRecorder.Body.String(), "not implemented")

	configRecorder := fusionControllerRequest(t, TestFusionConfig, http.MethodPost, "/api/fusion/configs/1/test", 1, gin.Param{
		Key:   "id",
		Value: fmt.Sprintf("%d", fusionConfig.Id),
	})
	require.Equal(t, http.StatusNotImplemented, configRecorder.Code)
	assert.Contains(t, configRecorder.Body.String(), "not implemented")
}
