package controller

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
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
	originalBatchUpdateEnabled := common.BatchUpdateEnabled
	originalDataExportEnabled := common.DataExportEnabled
	originalTLSInsecureSkipVerify := common.TLSInsecureSkipVerify
	originalRelayTimeout := common.RelayTimeout
	savedConfig := config.GlobalConfig.ExportAllConfigs()
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.CryptoSecret = originalSecret
		common.PersistentCryptoSecretConfigured = originalConfigured
		common.RedisEnabled = originalRedisEnabled
		common.BatchUpdateEnabled = originalBatchUpdateEnabled
		common.DataExportEnabled = originalDataExportEnabled
		common.TLSInsecureSkipVerify = originalTLSInsecureSkipVerify
		common.RelayTimeout = originalRelayTimeout
		service.InitHttpClient()
		require.NoError(t, config.GlobalConfig.LoadFromDB(savedConfig))
	})

	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.DataExportEnabled = false
	common.TLSInsecureSkipVerify = true
	common.RelayTimeout = 0
	service.InitHttpClient()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}, &model.UserSubscription{}, &model.FusionAPIKey{}, &model.FusionConfig{}))
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

func seedFusionRelayUserAndToken(t *testing.T, userId int, userQuota int, tokenQuota int) {
	t.Helper()
	settingBytes, err := common.Marshal(dto.UserSetting{BillingPreference: "wallet_only"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userId,
		Username: fmt.Sprintf("fusion-user-%d", userId),
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    userQuota,
		Group:    "default",
		Setting:  string(settingBytes),
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             userId * 10,
		UserId:         userId,
		Key:            fmt.Sprintf("fusion-token-%d", userId),
		Name:           "fusion-token",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		RemainQuota:    tokenQuota,
		UnlimitedQuota: false,
	}).Error)
}

func fusionRelayContext(t *testing.T, body any, userId int) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	data, err := common.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/fusion/chat/completions", bytes.NewReader(data))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userId)
	ctx.Set("token_id", userId*10)
	ctx.Set("token_key", fmt.Sprintf("fusion-token-%d", userId))
	ctx.Set("token_name", "fusion-token")
	ctx.Set("token_quota", tokenQuotaForFusionRelayTest)
	ctx.Set("token_unlimited_quota", false)
	common.SetContextKey(ctx, constant.ContextKeyUserId, userId)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUserQuota, tokenQuotaForFusionRelayTest)
	common.SetContextKey(ctx, constant.ContextKeyUserEmail, "fusion@example.com")
	common.SetContextKey(ctx, constant.ContextKeyUserName, fmt.Sprintf("fusion-user-%d", userId))
	common.SetContextKey(ctx, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})
	common.SetContextKey(ctx, constant.ContextKeyTokenId, userId*10)
	common.SetContextKey(ctx, constant.ContextKeyTokenKey, fmt.Sprintf("fusion-token-%d", userId))
	common.SetContextKey(ctx, constant.ContextKeyTokenUnlimited, false)
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "")
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, false)
	return recorder, ctx
}

const tokenQuotaForFusionRelayTest = 100000

func fusionRelayJSONRequest(t *testing.T, body any, userId int) *httptest.ResponseRecorder {
	t.Helper()
	recorder, ctx := fusionRelayContext(t, body, userId)
	FusionChatCompletions(ctx)
	return recorder
}

func configureFusionRelayServer(t *testing.T, serverURL string, extra map[string]string) {
	t.Helper()
	parsedURL, err := url.Parse(serverURL)
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(parsedURL.Host)
	require.NoError(t, err)
	settings := map[string]string{
		"fusion_setting.enabled":                "true",
		"fusion_setting.allow_private_base_url": "true",
		"fusion_setting.allowed_base_url_ports": fmt.Sprintf("[%s]", port),
		"fusion_setting.billing_expr":           "cp + cc + jp + jc",
		"fusion_setting.minimum_quota":          "1",
	}
	for key, value := range extra {
		settings[key] = value
	}
	require.NoError(t, config.GlobalConfig.LoadFromDB(settings))
}

func newFusionRelayTLSServer(t *testing.T, responses map[string]dto.OpenAITextResponse) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-controller-fusion", r.Header.Get("Authorization"))
		var request dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		response, ok := responses[request.Model]
		require.True(t, ok, "unexpected upstream model %s", request.Model)
		w.Header().Set("Content-Type", "application/json")
		data, err := common.Marshal(response)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func createFusionRelayKey(t *testing.T, userId int, baseURL string) *model.FusionAPIKey {
	t.Helper()
	key := &model.FusionAPIKey{
		UserId:       userId,
		Name:         "relay-key",
		Provider:     model.FusionProviderOpenAICompatible,
		BaseURL:      baseURL,
		DefaultModel: "candidate-a",
		Status:       model.FusionKeyStatusEnabled,
	}
	require.NoError(t, key.SetPlainAPIKey("sk-controller-fusion"))
	require.NoError(t, key.SetModels([]string{"candidate-a", "judge-model"}))
	require.NoError(t, key.Insert())
	return key
}

func createFusionRelayConfig(t *testing.T, userId int, key *model.FusionAPIKey) *model.FusionConfig {
	t.Helper()
	config := &model.FusionConfig{
		UserId:       userId,
		Name:         "Research",
		ModelAlias:   "fusion:research",
		Enabled:      true,
		JudgeKeyID:   key.Id,
		JudgeModel:   "judge-model",
		Strategy:     model.FusionStrategySynthesize,
		TimeoutMS:    45000,
		MaxParallel:  1,
		MinSuccesses: 1,
	}
	require.NoError(t, config.SetCandidateKeyIDs([]int{key.Id}))
	require.NoError(t, config.SetCandidateModels(map[string]string{fmt.Sprintf("%d", key.Id): "candidate-a"}))
	require.NoError(t, config.Insert())
	return config
}

func fusionRelayResponse(modelName string, content string, promptTokens int, completionTokens int) dto.OpenAITextResponse {
	return dto.OpenAITextResponse{
		Id:      "chatcmpl-test",
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Model:   modelName,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}
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

func TestCreateFusionConfigRejectsModelOutsideKeyAllowlist(t *testing.T) {
	setupFusionControllerTestDB(t)
	key := createFusionControllerKey(t, 1, "owned")

	recorder := fusionControllerJSONRequest(t, CreateFusionConfig, http.MethodPost, "/api/fusion/configs", 1, dto.FusionConfigCreateRequest{
		Name:            "Research",
		ModelAlias:      "fusion:research",
		Enabled:         true,
		CandidateKeyIDs: []int{key.Id},
		CandidateModels: map[string]string{fmt.Sprintf("%d", key.Id): "gpt-4o"},
		JudgeKeyID:      key.Id,
		JudgeModel:      "gpt-4o-mini",
		Strategy:        model.FusionStrategySynthesize,
		TimeoutMS:       45000,
		MaxParallel:     1,
		MinSuccesses:    1,
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeFusionControllerResponse(t, recorder)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, payload["message"], "candidate model")
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

func TestFusionTestEndpointsRequireCryptoSecretBeforeLookup(t *testing.T) {
	setupFusionControllerTestDB(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"fusion_setting.enabled": "true",
	}))
	common.PersistentCryptoSecretConfigured = false

	recorder := fusionControllerRequest(t, TestFusionAPIKey, http.MethodPost, "/api/fusion/keys/999/test", 1, gin.Param{
		Key:   "id",
		Value: "999",
	})

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "CRYPTO_SECRET")
	assert.NotContains(t, recorder.Body.String(), "record not found")
}

func TestFusionRelayDisabledBlocksBeforeUpstream(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServer(t, map[string]dto.OpenAITextResponse{})
	configureFusionRelayServer(t, server.URL, map[string]string{
		"fusion_setting.enabled": "false",
	})

	recorder := fusionRelayJSONRequest(t, gin.H{
		"model": "fusion:research",
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Fusion is disabled")
	assert.Equal(t, int32(0), calls.Load())
}

func TestFusionRelayRequiresPersistentCryptoSecretBeforeUpstream(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServer(t, map[string]dto.OpenAITextResponse{})
	configureFusionRelayServer(t, server.URL, nil)
	common.PersistentCryptoSecretConfigured = false

	recorder := fusionRelayJSONRequest(t, gin.H{
		"model": "fusion:research",
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "CRYPTO_SECRET")
	assert.Equal(t, int32(0), calls.Load())
}

func TestFusionRelayRejectsDirectCredentialFieldsBeforeUpstream(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServer(t, map[string]dto.OpenAITextResponse{})
	configureFusionRelayServer(t, server.URL, nil)

	recorder := fusionRelayJSONRequest(t, gin.H{
		"model":    "fusion:research",
		"base_url": server.URL,
		"api_key":  "sk-free",
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "fusion request cannot include")
	assert.Equal(t, int32(0), calls.Load())
}

func TestFusionRelayRejectsNestedDirectCredentialFieldsBeforeUpstream(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServer(t, map[string]dto.OpenAITextResponse{})
	configureFusionRelayServer(t, server.URL, nil)

	recorder := fusionRelayJSONRequest(t, gin.H{
		"model": "fusion:research",
		"fusion": gin.H{
			"base_url": server.URL,
		},
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "fusion.base_url")
	assert.Equal(t, int32(0), calls.Load())
}

func TestFusionRelayTokenModelLimitForbidsAlias(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServer(t, map[string]dto.OpenAITextResponse{})
	configureFusionRelayServer(t, server.URL, nil)
	recorder, ctx := fusionRelayContext(t, gin.H{
		"model": "fusion:research",
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{"gpt-4o-mini": true})

	FusionChatCompletions(ctx)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "fusion:research")
	assert.Equal(t, int32(0), calls.Load())
}

func TestFusionRelayInsufficientQuotaBlocksBeforeUpstream(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServer(t, map[string]dto.OpenAITextResponse{})
	configureFusionRelayServer(t, server.URL, map[string]string{
		"fusion_setting.billing_expr": "100",
	})
	seedFusionRelayUserAndToken(t, 1, 1, 1)
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	createFusionRelayConfig(t, 1, key)

	recorder := fusionRelayJSONRequest(t, gin.H{
		"model": "fusion:research",
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "quota")
	assert.Equal(t, int32(0), calls.Load())
}

func TestFusionRelayInvalidBillingExpressionFailsBeforeUpstream(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServer(t, map[string]dto.OpenAITextResponse{})
	configureFusionRelayServer(t, server.URL, map[string]string{
		"fusion_setting.billing_expr": "p + c",
	})
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	createFusionRelayConfig(t, 1, key)

	recorder := fusionRelayJSONRequest(t, gin.H{
		"model": "fusion:research",
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "fusion billing expression")
	assert.Equal(t, int32(0), calls.Load())
}

func TestFusionRelayReturnsOpenAIResponseAndRecordsBilling(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServer(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionRelayResponse("candidate-a", "candidate answer", 10, 2),
		"judge-model": fusionRelayResponse("judge-model", "final answer", 5, 3),
	})
	configureFusionRelayServer(t, server.URL, nil)
	initialQuota := common.GetTrustQuota() + 1000
	seedFusionRelayUserAndToken(t, 1, initialQuota, initialQuota)
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	createFusionRelayConfig(t, 1, key)

	recorder, ctx := fusionRelayContext(t, gin.H{
		"model": "fusion:research",
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)
	ctx.Set("token_quota", initialQuota)
	common.SetContextKey(ctx, constant.ContextKeyUserQuota, initialQuota)
	FusionChatCompletions(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, int32(2), calls.Load())
	var response dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "chat.completion", response.Object)
	assert.Equal(t, "fusion:research", response.Model)
	require.Len(t, response.Choices, 1)
	assert.Equal(t, "final answer", response.Choices[0].Message.StringContent())
	assert.Equal(t, 15, response.Usage.PromptTokens)
	assert.Equal(t, 5, response.Usage.CompletionTokens)

	userQuota, err := model.GetUserQuota(1, true)
	require.NoError(t, err)
	assert.Equal(t, initialQuota-20, userQuota)
	token, err := model.GetTokenById(10)
	require.NoError(t, err)
	assert.Equal(t, initialQuota-20, token.RemainQuota)

	var log model.Log
	require.NoError(t, model.LOG_DB.Where("type = ? AND model_name = ?", model.LogTypeConsume, "fusion:research").First(&log).Error)
	assert.Equal(t, 0, log.ChannelId)
	assert.Equal(t, 20, log.Quota)
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	assert.Equal(t, true, other["fusion"])
	assert.NotContains(t, log.Other, "candidate answer")
	assert.NotContains(t, log.Other, "sk-controller-fusion")
}
