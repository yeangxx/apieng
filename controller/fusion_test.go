package controller

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"

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
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}, &model.UserSubscription{}, &model.FusionUpstreamTemplate{}, &model.FusionAPIKey{}, &model.FusionConfig{}, &model.FusionResponseState{}))
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

func createFusionControllerChatTemplate(t *testing.T) *model.FusionUpstreamTemplate {
	t.Helper()
	template := &model.FusionUpstreamTemplate{
		Name:          "Test OpenAI Chat Completions",
		ProviderLabel: "OpenAI Compatible",
		Protocol:      model.FusionProtocolOpenAIChatCompatible,
		EndpointPath:  "/v1/chat/completions",
		AuthType:      model.FusionAuthTypeBearer,
		AuthHeader:    "Authorization",
		DetectRules:   `[]`,
		Enabled:       true,
	}
	require.NoError(t, template.Insert())
	return template
}

func createFusionControllerKey(t *testing.T, userId int, name string) *model.FusionAPIKey {
	t.Helper()
	template := createFusionControllerChatTemplate(t)
	key := &model.FusionAPIKey{
		UserId:       userId,
		Name:         name,
		Provider:     model.FusionProviderOpenAICompatible,
		TemplateID:   template.Id,
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
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{"fusion:research": true})
	return recorder, ctx
}

const tokenQuotaForFusionRelayTest = 100000

func fusionRelayJSONRequest(t *testing.T, body any, userId int) *httptest.ResponseRecorder {
	t.Helper()
	recorder, ctx := fusionRelayContext(t, body, userId)
	FusionChatCompletions(ctx)
	return recorder
}

func fusionStandardChatRelayJSONRequest(t *testing.T, body any, userId int) *httptest.ResponseRecorder {
	t.Helper()
	data, err := common.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(data))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userId)
	ctx.Set("token_id", userId*10)
	ctx.Set("token_key", fmt.Sprintf("fusion-token-%d", userId))
	ctx.Set("token_name", "fusion-token")
	ctx.Set("token_quota", tokenQuotaForFusionRelayTest)
	ctx.Set("token_unlimited_quota", true)
	ctx.Set("user_quota", 1000000)
	ctx.Set("group", "default")
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUserQuota, tokenQuotaForFusionRelayTest)
	common.SetContextKey(ctx, constant.ContextKeyTokenUnlimited, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{"fusion:research": true})
	common.SetContextKey(ctx, constant.ContextKeyIsFusionRequest, true)
	Relay(ctx, types.RelayFormatOpenAI)
	return recorder
}

func fusionStandardResponsesRelayJSONRequest(t *testing.T, body any, userId int) *httptest.ResponseRecorder {
	t.Helper()
	data, err := common.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(data))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userId)
	ctx.Set("token_id", userId*10)
	ctx.Set("token_key", fmt.Sprintf("fusion-token-%d", userId))
	ctx.Set("token_name", "fusion-token")
	ctx.Set("token_quota", tokenQuotaForFusionRelayTest)
	ctx.Set("token_unlimited_quota", true)
	ctx.Set("user_quota", 1000000)
	ctx.Set("group", "default")
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUserQuota, tokenQuotaForFusionRelayTest)
	common.SetContextKey(ctx, constant.ContextKeyTokenUnlimited, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{"fusion:research": true})
	common.SetContextKey(ctx, constant.ContextKeyIsFusionRequest, true)
	Relay(ctx, types.RelayFormatOpenAIResponses)
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
		"fusion_setting.allowed_token_groups":   `["default"]`,
		"fusion_setting.billing_expr":           "cp + cc + jp + jc",
		"fusion_setting.minimum_quota":          "1",
	}
	for key, value := range extra {
		settings[key] = value
	}
	require.NoError(t, config.GlobalConfig.LoadFromDB(settings))
}

func newFusionRelayTLSServer(t *testing.T, responses map[string]dto.OpenAITextResponse) (*httptest.Server, *atomic.Int32) {
	return newFusionRelayTLSServerWithRequestCheck(t, responses, nil)
}

func newFusionRelayTLSServerWithRequestCheck(t *testing.T, responses map[string]dto.OpenAITextResponse, check func(*testing.T, dto.GeneralOpenAIRequest)) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-controller-fusion", r.Header.Get("Authorization"))
		var request dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		if check != nil {
			check(t, request)
		}
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

func newDelayedFusionRelayTLSServer(t *testing.T, delay time.Duration, responses map[string]dto.OpenAITextResponse) (*httptest.Server, *atomic.Int32) {
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
		time.Sleep(delay)
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
	template := createFusionControllerChatTemplate(t)
	key := &model.FusionAPIKey{
		UserId:       userId,
		Name:         "relay-key",
		Provider:     model.FusionProviderOpenAICompatible,
		TemplateID:   template.Id,
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

func writeFusionRelayStreamChunk(t *testing.T, w http.ResponseWriter, chunk gin.H) {
	t.Helper()
	data, err := common.Marshal(chunk)
	require.NoError(t, err)
	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	require.NoError(t, err)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func writeFusionRelayStreamDone(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	_, err := fmt.Fprint(w, "data: [DONE]\n\n")
	require.NoError(t, err)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func fusionRelayToolCallResponse(modelName string, callID string, toolName string, arguments string, promptTokens int, completionTokens int) dto.OpenAITextResponse {
	message := dto.Message{
		Role:    "assistant",
		Content: "",
	}
	message.SetToolCalls([]dto.ToolCallResponse{
		{
			ID:   callID,
			Type: "function",
			Function: dto.FunctionResponse{
				Name:      toolName,
				Arguments: arguments,
			},
		},
	})
	return dto.OpenAITextResponse{
		Id:      "chatcmpl-tool-test",
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Model:   modelName,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index:        0,
				Message:      message,
				FinishReason: "tool_calls",
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

func TestUnsavedFusionAPIKeyTestReturnsDetectedConfigWithoutPersistingKey(t *testing.T) {
	setupFusionControllerTestDB(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-detect-secret", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"error":{"message":"missing provider option for sk-detect-secret"}}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	configureFusionRelayServer(t, server.URL, nil)
	template := &model.FusionUpstreamTemplate{
		Name:          "Detectable",
		ProviderLabel: "Detectable",
		Protocol:      model.FusionProtocolOpenAIChatCompatible,
		EndpointPath:  "/v1/chat/completions",
		AuthType:      model.FusionAuthTypeBearer,
		AuthHeader:    "Authorization",
		DetectRules: `[{
			"status_codes": [400],
			"error_contains": ["missing provider option"],
			"suggested_config": {
				"body_overrides": {"provider_option": "required"}
			}
		}]`,
		Enabled: true,
	}
	require.NoError(t, template.Insert())

	recorder := fusionControllerJSONRequest(t, TestUnsavedFusionAPIKey, http.MethodPost, "/api/fusion/keys/test", 1, dto.FusionAPIKeyTestRequest{
		FusionAPIKeyCreateRequest: dto.FusionAPIKeyCreateRequest{
			Name:         "Probe",
			Provider:     model.FusionProviderOpenAICompatible,
			TemplateID:   template.Id,
			BaseURL:      server.URL,
			APIKey:       "sk-detect-secret",
			DefaultModel: "gpt-4o-mini",
			Models:       []string{"gpt-4o-mini"},
		},
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "sk-detect-secret")
	payload := decodeFusionControllerResponse(t, recorder)
	assert.Equal(t, true, payload["success"])
	data, ok := payload["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, false, data["ok"])
	detected, ok := data["detected_config"].(map[string]interface{})
	require.True(t, ok)
	overrides, ok := detected["body_overrides"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "required", overrides["provider_option"])
	total, err := model.CountFusionAPIKeysByUserId(1)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestSavedFusionAPIKeyTestUsesPayloadWithoutPersistingConfig(t *testing.T) {
	setupFusionControllerTestDB(t)
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-saved-probe", r.Header.Get("Authorization"))
		var request map[string]interface{}
		require.NoError(t, common.DecodeJson(r.Body, &request))
		assert.Equal(t, "enabled", request["gateway_flag"])
		assert.Equal(t, "gpt-4o-mini", request["model"])
		_, err := w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	configureFusionRelayServer(t, server.URL, nil)
	key := createFusionControllerKey(t, 1, "saved-probe")
	originalBaseURL := key.BaseURL

	recorder := fusionControllerJSONRequest(t, TestFusionAPIKey, http.MethodPost, "/api/fusion/keys/1/test", 1, dto.FusionAPIKeyUpdateRequest{
		Name:           key.Name,
		Provider:       key.Provider,
		TemplateID:     key.TemplateID,
		BaseURL:        server.URL,
		DefaultModel:   "gpt-4o-mini",
		Models:         []string{"gpt-4o-mini"},
		UpstreamConfig: `{"body_overrides":{"gateway_flag":"enabled"}}`,
		Status:         model.FusionKeyStatusEnabled,
	}, gin.Param{Key: "id", Value: fmt.Sprintf("%d", key.Id)})

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeFusionControllerResponse(t, recorder)
	data, ok := payload["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, data["ok"])
	assert.Equal(t, int32(1), calls.Load())
	reloaded, err := model.GetFusionAPIKeyByUserAndId(1, key.Id)
	require.NoError(t, err)
	assert.Equal(t, originalBaseURL, reloaded.BaseURL)
	assert.JSONEq(t, "{}", reloaded.UpstreamConfig)
}

func TestAdminFusionUpstreamTemplateCRUD(t *testing.T) {
	setupFusionControllerTestDB(t)
	createRecorder := fusionControllerJSONRequest(t, AdminCreateFusionUpstreamTemplate, http.MethodPost, "/api/fusion/admin/upstream-templates", 1, dto.FusionUpstreamTemplateRequest{
		Name:                 "Gateway",
		ProviderLabel:        "Gateway",
		Protocol:             model.FusionProtocolOpenAIChatCompatible,
		EndpointPath:         "/gateway/chat",
		AuthType:             model.FusionAuthTypeBearer,
		AuthHeader:           "Authorization",
		DefaultHeaders:       `{"X-Gateway":"1"}`,
		DefaultQuery:         `{}`,
		DefaultBodyOverrides: `{}`,
		DetectRules:          `[]`,
		Enabled:              true,
		Sort:                 10,
	})
	require.Equal(t, http.StatusOK, createRecorder.Code)
	payload := decodeFusionControllerResponse(t, createRecorder)
	data, ok := payload["data"].(map[string]interface{})
	require.True(t, ok)
	id := int(data["id"].(float64))

	listRecorder := fusionControllerRequest(t, AdminGetFusionUpstreamTemplates, http.MethodGet, "/api/fusion/admin/upstream-templates", 1)
	require.Equal(t, http.StatusOK, listRecorder.Code)
	assert.Contains(t, listRecorder.Body.String(), "Gateway")

	deleteRecorder := fusionControllerRequest(t, AdminDeleteFusionUpstreamTemplate, http.MethodDelete, "/api/fusion/admin/upstream-templates/1", 1, gin.Param{
		Key:   "id",
		Value: fmt.Sprintf("%d", id),
	})
	require.Equal(t, http.StatusOK, deleteRecorder.Code)
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

func TestCreateFusionConfigRejectsForeignDirectKey(t *testing.T) {
	setupFusionControllerTestDB(t)
	key := createFusionControllerKey(t, 1, "owned")
	foreignKey := createFusionControllerKey(t, 2, "foreign")

	recorder := fusionControllerJSONRequest(t, CreateFusionConfig, http.MethodPost, "/api/fusion/configs", 1, dto.FusionConfigCreateRequest{
		Name:            "Research",
		ModelAlias:      "fusion:research",
		Enabled:         true,
		CandidateKeyIDs: []int{key.Id},
		CandidateModels: map[string]string{fmt.Sprintf("%d", key.Id): "gpt-4o-mini"},
		JudgeKeyID:      key.Id,
		JudgeModel:      "gpt-4o-mini",
		RoutingMode:     model.FusionRoutingModeAutoSimple,
		DirectKeyID:     foreignKey.Id,
		DirectModel:     "gpt-4o-mini",
		Strategy:        model.FusionStrategySynthesize,
		TimeoutMS:       45000,
		MaxParallel:     1,
		MinSuccesses:    1,
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeFusionControllerResponse(t, recorder)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, payload["message"], "direct key")
}

func TestCreateFusionConfigRejectsForeignRankerKey(t *testing.T) {
	setupFusionControllerTestDB(t)
	key := createFusionControllerKey(t, 1, "owned")
	foreignKey := createFusionControllerKey(t, 2, "foreign")

	recorder := fusionControllerJSONRequest(t, CreateFusionConfig, http.MethodPost, "/api/fusion/configs", 1, dto.FusionConfigCreateRequest{
		Name:                  "Research",
		ModelAlias:            "fusion:research",
		Enabled:               true,
		CandidateKeyIDs:       []int{key.Id},
		CandidateModels:       map[string]string{fmt.Sprintf("%d", key.Id): "gpt-4o-mini"},
		JudgeKeyID:            key.Id,
		JudgeModel:            "gpt-4o-mini",
		QualityMode:           model.FusionQualityModeRanked,
		RankerKeyID:           foreignKey.Id,
		RankerModel:           "gpt-4o-mini",
		QualityThreshold:      0.65,
		RankerTopK:            1,
		CandidateSamplingMode: model.FusionCandidateSamplingModeConfigured,
		Strategy:              model.FusionStrategySynthesize,
		TimeoutMS:             45000,
		MaxParallel:           1,
		MinSuccesses:          1,
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeFusionControllerResponse(t, recorder)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, payload["message"], "ranker key")
}

func TestCreateFusionConfigReturnsSavedShape(t *testing.T) {
	setupFusionControllerTestDB(t)
	key := createFusionControllerKey(t, 1, "owned")

	recorder := fusionControllerJSONRequest(t, CreateFusionConfig, http.MethodPost, "/api/fusion/configs", 1, dto.FusionConfigCreateRequest{
		Name:                  "Research",
		ModelAlias:            "fusion:research",
		Enabled:               true,
		CandidateKeyIDs:       []int{key.Id},
		CandidateModels:       map[string]string{fmt.Sprintf("%d", key.Id): "gpt-4o-mini"},
		JudgeKeyID:            key.Id,
		JudgeModel:            "gpt-4o-mini",
		RoutingMode:           model.FusionRoutingModeAutoSimple,
		DirectKeyID:           key.Id,
		DirectModel:           "gpt-4o-mini",
		QualityMode:           model.FusionQualityModeRanked,
		RankerKeyID:           key.Id,
		RankerModel:           "gpt-4o-mini",
		QualityThreshold:      0.7,
		RankerTopK:            1,
		CandidateSamplingMode: model.FusionCandidateSamplingModeSelfSample,
		Strategy:              model.FusionStrategySynthesize,
		TimeoutMS:             45000,
		MaxParallel:           1,
		MinSuccesses:          1,
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeFusionControllerResponse(t, recorder)
	assert.Equal(t, true, payload["success"])
	body := recorder.Body.String()
	assert.Contains(t, body, "fusion:research")
	assert.Contains(t, body, "candidate_key_ids")
	assert.Contains(t, body, "routing_mode")
	assert.Contains(t, body, model.FusionRoutingModeAutoSimple)
	assert.Contains(t, body, "direct_key_id")
	assert.Contains(t, body, "direct_model")
	assert.Contains(t, body, "quality_mode")
	assert.Contains(t, body, model.FusionQualityModeRanked)
	assert.Contains(t, body, "ranker_key_id")
	assert.Contains(t, body, "quality_threshold")
	assert.Contains(t, body, "ranker_top_k")
	assert.Contains(t, body, "candidate_sampling_mode")
	assert.Contains(t, body, model.FusionCandidateSamplingModeSelfSample)
}

func TestFusionLogOtherIncludesExecutionModeAndCanceledCandidates(t *testing.T) {
	result := &service.FusionEngineResult{
		ExecutionMode: service.FusionExecutionModeCache,
		RouteReason:   "result_cache_hit",
		CacheHit:      true,
		Candidates: []service.FusionCandidateResult{
			{KeyID: 1, Model: "candidate-a", Success: true, Usage: dto.Usage{PromptTokens: 10, CompletionTokens: 2}},
			{KeyID: 2, Model: "candidate-b", Canceled: true},
		},
	}

	other := buildFusionLogOther(nil, result, 100, 1, service.FusionBillingPolicy{MinimumQuota: 1, GroupRatio: 1})

	assert.Equal(t, service.FusionExecutionModeCache, other["execution_mode"])
	assert.Equal(t, "result_cache_hit", other["route_reason"])
	assert.Equal(t, true, other["cache_hit"])
	assert.Equal(t, 2, other["candidate_count"])
	assert.Equal(t, 1, other["candidate_success_count"])
	assert.Equal(t, 1, other["candidate_canceled_count"])
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

func TestFusionTestEndpointsRunKeyProbeWithoutSavingDetectedConfig(t *testing.T) {
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

	recorder := fusionControllerRequest(t, TestFusionAPIKey, http.MethodPost, "/api/fusion/keys/1/test", 1, gin.Param{
		Key:   "id",
		Value: fmt.Sprintf("%d", key.Id),
	})
	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeFusionControllerResponse(t, recorder)
	assert.Equal(t, true, payload["success"])
	body := recorder.Body.String()
	assert.Contains(t, body, `"ok":false`)
	assert.NotContains(t, body, "not implemented")

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"fusion_setting.enabled": "true",
	}))
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

func TestFusionRelayRequiresFusionTokenGroup(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServer(t, map[string]dto.OpenAITextResponse{})
	configureFusionRelayServer(t, server.URL, map[string]string{
		"fusion_setting.allowed_token_groups": `["fusion-basic"]`,
	})
	recorder, ctx := fusionRelayContext(t, gin.H{
		"model": "fusion:research",
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "default")

	FusionChatCompletions(ctx)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Fusion models require a Fusion token group")
	assert.Equal(t, int32(0), calls.Load())
}

func TestFusionRelayRequiresBoundFusionModelLimit(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServer(t, map[string]dto.OpenAITextResponse{})
	configureFusionRelayServer(t, server.URL, nil)
	recorder, ctx := fusionRelayContext(t, gin.H{
		"model": "fusion:research",
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, false)

	FusionChatCompletions(ctx)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "exactly one enabled Fusion model limit")
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

func TestStandardChatCompletionsFusionAliasUsesFusionRelay(t *testing.T) {
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

	recorder := fusionStandardChatRelayJSONRequest(t, gin.H{
		"model": "fusion:research",
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, int32(2), calls.Load())
	var response dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "fusion:research", response.Model)
	require.Len(t, response.Choices, 1)
	assert.Equal(t, "final answer", response.Choices[0].Message.StringContent())
}

func TestStandardChatCompletionsFusionStreamReturnsCompatibleSSE(t *testing.T) {
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

	recorder := fusionStandardChatRelayJSONRequest(t, gin.H{
		"model":  "fusion:research",
		"stream": true,
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	assert.Equal(t, int32(2), calls.Load())
	body := recorder.Body.String()
	assert.Contains(t, body, "chat.completion.chunk")
	assert.Contains(t, body, "fusion:research")
	assert.Contains(t, body, "final answer")
	assert.Contains(t, body, "[DONE]")
}

func TestStandardChatCompletionsFusionStreamWritesJudgeDeltas(t *testing.T) {
	setupFusionControllerTestDB(t)
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-controller-fusion", r.Header.Get("Authorization"))
		var request dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		require.NotNil(t, request.Stream)
		require.True(t, *request.Stream)
		w.Header().Set("Content-Type", "text/event-stream")
		contentParts := []string{"candidate answer"}
		if request.Model == "judge-model" {
			contentParts = []string{"streamed ", "final"}
		}
		for _, part := range contentParts {
			writeFusionRelayStreamChunk(t, w, gin.H{
				"id":      "chatcmpl-stream-test",
				"object":  "chat.completion.chunk",
				"created": common.GetTimestamp(),
				"model":   request.Model,
				"choices": []gin.H{{"index": 0, "delta": gin.H{"role": "assistant", "content": part}, "finish_reason": nil}},
			})
		}
		writeFusionRelayStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-stream-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   request.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{}, "finish_reason": "stop"}},
			"usage": gin.H{
				"prompt_tokens":     10,
				"completion_tokens": len(contentParts),
				"total_tokens":      10 + len(contentParts),
			},
		})
		writeFusionRelayStreamDone(t, w)
	}))
	t.Cleanup(server.Close)
	configureFusionRelayServer(t, server.URL, nil)
	initialQuota := common.GetTrustQuota() + 1000
	seedFusionRelayUserAndToken(t, 1, initialQuota, initialQuota)
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	createFusionRelayConfig(t, 1, key)

	recorder := fusionStandardChatRelayJSONRequest(t, gin.H{
		"model":  "fusion:research",
		"stream": true,
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	}, 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	assert.Equal(t, int32(2), calls.Load())
	body := recorder.Body.String()
	assert.Contains(t, body, `"content":"streamed "`)
	assert.Contains(t, body, `"content":"final"`)
	assert.Less(t, strings.Index(body, `"content":"streamed "`), strings.Index(body, `"content":"final"`))
	assert.NotContains(t, body, `"content":"streamed final"`)
	assert.Contains(t, body, "[DONE]")
}

func TestFusionChatCompletionsStreamSendsHeartbeatWhileFusionRuns(t *testing.T) {
	setupFusionControllerTestDB(t)
	originalPingInterval := fusionStreamPingInterval
	fusionStreamPingInterval = 20 * time.Millisecond
	t.Cleanup(func() {
		fusionStreamPingInterval = originalPingInterval
	})

	server, calls := newDelayedFusionRelayTLSServer(t, 120*time.Millisecond, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionRelayResponse("candidate-a", "candidate answer", 10, 2),
		"judge-model": fusionRelayResponse("judge-model", "final answer", 5, 3),
	})
	configureFusionRelayServer(t, server.URL, nil)
	initialQuota := common.GetTrustQuota() + 1000
	seedFusionRelayUserAndToken(t, 1, initialQuota, initialQuota)
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	createFusionRelayConfig(t, 1, key)

	router := gin.New()
	router.POST("/v1/chat/completions", func(ctx *gin.Context) {
		ctx.Set("id", 1)
		ctx.Set("token_id", 10)
		ctx.Set("token_key", "fusion-token-1")
		ctx.Set("token_name", "fusion-token")
		ctx.Set("token_quota", initialQuota)
		ctx.Set("token_unlimited_quota", true)
		common.SetContextKey(ctx, constant.ContextKeyUserId, 1)
		common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(ctx, constant.ContextKeyUserQuota, initialQuota)
		common.SetContextKey(ctx, constant.ContextKeyUserEmail, "fusion@example.com")
		common.SetContextKey(ctx, constant.ContextKeyUserName, "fusion-user-1")
		common.SetContextKey(ctx, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})
		common.SetContextKey(ctx, constant.ContextKeyTokenId, 10)
		common.SetContextKey(ctx, constant.ContextKeyTokenKey, "fusion-token-1")
		common.SetContextKey(ctx, constant.ContextKeyTokenUnlimited, true)
		common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "default")
		common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
		common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{"fusion:research": true})
		FusionChatCompletions(ctx)
	})
	appServer := httptest.NewServer(router)
	t.Cleanup(appServer.Close)

	payload, err := common.Marshal(gin.H{
		"model":  "fusion:research",
		"stream": true,
		"messages": []gin.H{
			{"role": "user", "content": "hello"},
		},
	})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, appServer.URL+"/v1/chat/completions", bytes.NewReader(payload))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response, err := appServer.Client().Do(request)
	require.NoError(t, err)
	defer response.Body.Close()

	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.Contains(t, response.Header.Get("Content-Type"), "text/event-stream")
	reader := bufio.NewReader(response.Body)
	firstLine, err := reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, ": PING\n", firstLine)
	rest, err := io.ReadAll(reader)
	require.NoError(t, err)
	body := string(rest)
	assert.Contains(t, body, "chat.completion.chunk")
	assert.Contains(t, body, "final answer")
	assert.Contains(t, body, "[DONE]")
	assert.Equal(t, int32(2), calls.Load())
}

func TestStandardChatCompletionsFusionToolsReturnsToolCall(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServerWithRequestCheck(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionRelayToolCallResponse("candidate-a", "call_read", "read_file", `{"path":"main.go"}`, 10, 1),
		"judge-model": fusionRelayResponse("judge-model", "should not run", 5, 3),
	}, func(t *testing.T, request dto.GeneralOpenAIRequest) {
		if request.Model != "candidate-a" {
			return
		}
		require.Len(t, request.Tools, 1)
		assert.Equal(t, "read_file", request.Tools[0].Function.Name)
		assert.NotNil(t, request.Tools[0].Function.Parameters)
		require.NotNil(t, request.ToolChoice)
	})
	configureFusionRelayServer(t, server.URL, nil)
	initialQuota := common.GetTrustQuota() + 1000
	seedFusionRelayUserAndToken(t, 1, initialQuota, initialQuota)
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	createFusionRelayConfig(t, 1, key)

	recorder := fusionStandardChatRelayJSONRequest(t, gin.H{
		"model": "fusion:research",
		"messages": []gin.H{
			{"role": "user", "content": "read main.go"},
		},
		"tools": []gin.H{
			{
				"type": "function",
				"function": gin.H{
					"name":        "read_file",
					"description": "Read a file",
					"parameters": gin.H{
						"type": "object",
					},
				},
			},
		},
		"tool_choice": gin.H{
			"type": "function",
			"function": gin.H{
				"name": "read_file",
			},
		},
	}, 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, int32(1), calls.Load())
	var response dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Choices, 1)
	assert.Equal(t, "tool_calls", response.Choices[0].FinishReason)
	toolCalls := response.Choices[0].Message.ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call_read", toolCalls[0].ID)
	assert.Equal(t, "read_file", toolCalls[0].Function.Name)
	assert.Equal(t, `{"path":"main.go"}`, toolCalls[0].Function.Arguments)
}

func TestStandardChatCompletionsFusionToolCallStreamReturnsSSE(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServer(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionRelayToolCallResponse("candidate-a", "call_read", "read_file", `{"path":"main.go"}`, 10, 1),
		"judge-model": fusionRelayResponse("judge-model", "should not run", 5, 3),
	})
	configureFusionRelayServer(t, server.URL, nil)
	initialQuota := common.GetTrustQuota() + 1000
	seedFusionRelayUserAndToken(t, 1, initialQuota, initialQuota)
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	createFusionRelayConfig(t, 1, key)

	recorder := fusionStandardChatRelayJSONRequest(t, gin.H{
		"model":  "fusion:research",
		"stream": true,
		"messages": []gin.H{
			{"role": "user", "content": "read main.go"},
		},
		"tools": []gin.H{
			{
				"type": "function",
				"function": gin.H{
					"name": "read_file",
				},
			},
		},
	}, 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	assert.Equal(t, int32(1), calls.Load())
	body := recorder.Body.String()
	assert.Contains(t, body, "chat.completion.chunk")
	assert.Contains(t, body, "tool_calls")
	assert.Contains(t, body, "call_read")
	assert.Contains(t, body, "read_file")
	assert.Contains(t, body, "[DONE]")
}

func TestResponsesFusionAliasUsesFusionRelay(t *testing.T) {
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

	recorder := fusionStandardResponsesRelayJSONRequest(t, gin.H{
		"model":             "fusion:research",
		"input":             "hello",
		"max_output_tokens": 64,
	}, 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, int32(2), calls.Load())
	var response dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "response", response.Object)
	assert.Equal(t, "fusion:research", response.Model)
	assert.Contains(t, string(response.Status), "completed")
	require.Len(t, response.Output, 1)
	require.Len(t, response.Output[0].Content, 1)
	assert.Equal(t, "output_text", response.Output[0].Content[0].Type)
	assert.Equal(t, "final answer", response.Output[0].Content[0].Text)
	require.NotNil(t, response.Usage)
	assert.Equal(t, 15, response.Usage.InputTokens)
	assert.Equal(t, 5, response.Usage.OutputTokens)
}

func TestResponsesFusionStreamReturnsCompatibleSSE(t *testing.T) {
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

	recorder := fusionStandardResponsesRelayJSONRequest(t, gin.H{
		"model":  "fusion:research",
		"input":  "hello",
		"stream": true,
	}, 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	assert.Equal(t, int32(2), calls.Load())
	body := recorder.Body.String()
	expectedEvents := []string{
		"response.created",
		"response.in_progress",
		"response.output_item.added",
		"response.content_part.added",
		"response.output_text.delta",
		"response.output_text.done",
		"response.content_part.done",
		"response.output_item.done",
		"response.completed",
	}
	lastEventIndex := -1
	for _, eventType := range expectedEvents {
		eventIndex := strings.Index(body, "event: "+eventType)
		require.NotEqual(t, -1, eventIndex, eventType)
		assert.Greater(t, eventIndex, lastEventIndex, eventType)
		lastEventIndex = eventIndex
	}
	assert.Contains(t, body, "fusion:research")
	assert.Contains(t, body, "final answer")
}

func TestResponsesFusionStreamWritesJudgeDeltas(t *testing.T) {
	setupFusionControllerTestDB(t)
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-controller-fusion", r.Header.Get("Authorization"))
		var request dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		require.NotNil(t, request.Stream)
		require.True(t, *request.Stream)
		w.Header().Set("Content-Type", "text/event-stream")
		contentParts := []string{"candidate answer"}
		if request.Model == "judge-model" {
			contentParts = []string{"streamed ", "final"}
		}
		for _, part := range contentParts {
			writeFusionRelayStreamChunk(t, w, gin.H{
				"id":      "chatcmpl-stream-test",
				"object":  "chat.completion.chunk",
				"created": common.GetTimestamp(),
				"model":   request.Model,
				"choices": []gin.H{{"index": 0, "delta": gin.H{"role": "assistant", "content": part}, "finish_reason": nil}},
			})
		}
		writeFusionRelayStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-stream-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   request.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{}, "finish_reason": "stop"}},
			"usage": gin.H{
				"prompt_tokens":     10,
				"completion_tokens": len(contentParts),
				"total_tokens":      10 + len(contentParts),
			},
		})
		writeFusionRelayStreamDone(t, w)
	}))
	t.Cleanup(server.Close)
	configureFusionRelayServer(t, server.URL, nil)
	initialQuota := common.GetTrustQuota() + 1000
	seedFusionRelayUserAndToken(t, 1, initialQuota, initialQuota)
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	createFusionRelayConfig(t, 1, key)

	recorder := fusionStandardResponsesRelayJSONRequest(t, gin.H{
		"model":  "fusion:research",
		"input":  "hello",
		"stream": true,
	}, 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	assert.Equal(t, int32(2), calls.Load())
	body := recorder.Body.String()
	assert.GreaterOrEqual(t, strings.Count(body, "event: response.output_text.delta"), 2)
	assert.Contains(t, body, `"delta":"streamed "`)
	assert.Contains(t, body, `"delta":"final"`)
	assert.Less(t, strings.Index(body, `"delta":"streamed "`), strings.Index(body, `"delta":"final"`))
	assert.Contains(t, body, `"text":"streamed final"`)
	assert.Contains(t, body, "event: response.completed")
}

func TestResponsesFusionStreamTimeoutReturnsTimeoutErrorCode(t *testing.T) {
	setupFusionControllerTestDB(t)
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-controller-fusion", r.Header.Get("Authorization"))
		var request dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		require.NotNil(t, request.Stream)
		require.True(t, *request.Stream)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(50 * time.Millisecond)
	}))
	t.Cleanup(server.Close)
	configureFusionRelayServer(t, server.URL, nil)
	initialQuota := common.GetTrustQuota() + 1000
	seedFusionRelayUserAndToken(t, 1, initialQuota, initialQuota)
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	fusionConfig := createFusionRelayConfig(t, 1, key)
	fusionConfig.TimeoutMS = 10
	require.NoError(t, model.DB.Model(fusionConfig).Update("timeout_ms", fusionConfig.TimeoutMS).Error)

	recorder := fusionStandardResponsesRelayJSONRequest(t, gin.H{
		"model":  "fusion:research",
		"input":  "hello",
		"stream": true,
	}, 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, int32(1), calls.Load())
	body := recorder.Body.String()
	assert.Contains(t, body, "event: response.failed")
	assert.Contains(t, body, `"status":"failed"`)
	assert.Contains(t, body, string(types.ErrorCodeChannelResponseTimeExceeded))
	assert.Contains(t, body, "upstream request timeout")
	assert.NotContains(t, body, "context deadline exceeded")
	assert.NotContains(t, body, string(types.ErrorCodeBadResponse))
}

func TestResponsesFusionToolsReturnsFunctionCall(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServerWithRequestCheck(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionRelayToolCallResponse("candidate-a", "call_read", "read_file", `{"path":"main.go"}`, 10, 1),
		"judge-model": fusionRelayResponse("judge-model", "should not run", 5, 3),
	}, func(t *testing.T, request dto.GeneralOpenAIRequest) {
		if request.Model != "candidate-a" {
			return
		}
		require.Len(t, request.Tools, 1)
		assert.Equal(t, "read_file", request.Tools[0].Function.Name)
		require.NotNil(t, request.ToolChoice)
	})
	configureFusionRelayServer(t, server.URL, nil)
	initialQuota := common.GetTrustQuota() + 1000
	seedFusionRelayUserAndToken(t, 1, initialQuota, initialQuota)
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	createFusionRelayConfig(t, 1, key)

	recorder := fusionStandardResponsesRelayJSONRequest(t, gin.H{
		"model": "fusion:research",
		"input": "hello",
		"tools": []gin.H{
			{
				"type":        "function",
				"name":        "read_file",
				"description": "Read a file",
				"parameters": gin.H{
					"type": "object",
				},
			},
		},
		"tool_choice": gin.H{
			"type": "function",
			"name": "read_file",
		},
	}, 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, int32(1), calls.Load())
	var response dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Output, 1)
	assert.Equal(t, "function_call", response.Output[0].Type)
	assert.Equal(t, "call_read", response.Output[0].CallId)
	assert.Equal(t, "read_file", response.Output[0].Name)
	assert.Equal(t, `{"path":"main.go"}`, response.Output[0].ArgumentsString())
	_, err := model.GetFusionResponseState(response.ID, 1, 10)
	require.NoError(t, err)
}

func TestResponsesFusionPreviousResponseIDRestoresToolCallContext(t *testing.T) {
	setupFusionControllerTestDB(t)
	var seenSecondRound atomic.Bool
	var calls atomic.Int32
	var candidateCalls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-controller-fusion", r.Header.Get("Authorization"))
		var request dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		var response dto.OpenAITextResponse
		switch request.Model {
		case "candidate-a":
			if candidateCalls.Add(1) == 1 {
				response = fusionRelayToolCallResponse("candidate-a", "call_read", "read_file", `{"path":"main.go"}`, 10, 1)
				break
			}
			seenSecondRound.Store(true)
			require.Len(t, request.Messages, 3)
			assert.Equal(t, "user", request.Messages[0].Role)
			assert.Equal(t, "hello", request.Messages[0].StringContent())
			assert.Equal(t, "assistant", request.Messages[1].Role)
			toolCalls := request.Messages[1].ParseToolCalls()
			require.Len(t, toolCalls, 1)
			assert.Equal(t, "call_read", toolCalls[0].ID)
			assert.Equal(t, "read_file", toolCalls[0].Function.Name)
			assert.Equal(t, "tool", request.Messages[2].Role)
			assert.Equal(t, "call_read", request.Messages[2].ToolCallId)
			assert.Contains(t, request.Messages[2].StringContent(), "package main")
			response = fusionRelayResponse("candidate-a", "candidate saw tool output", 10, 2)
		case "judge-model":
			response = fusionRelayResponse("judge-model", "final answer", 5, 3)
		default:
			require.FailNow(t, "unexpected upstream model "+request.Model)
		}
		w.Header().Set("Content-Type", "application/json")
		data, err := common.Marshal(response)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	configureFusionRelayServer(t, server.URL, nil)
	initialQuota := common.GetTrustQuota() + 1000
	seedFusionRelayUserAndToken(t, 1, initialQuota, initialQuota)
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	createFusionRelayConfig(t, 1, key)

	firstRecorder := fusionStandardResponsesRelayJSONRequest(t, gin.H{
		"model": "fusion:research",
		"input": "hello",
		"tools": []gin.H{
			{
				"type": "function",
				"name": "read_file",
				"parameters": gin.H{
					"type": "object",
				},
			},
		},
	}, 1)

	require.Equal(t, http.StatusOK, firstRecorder.Code)
	var firstResponse dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(firstRecorder.Body.Bytes(), &firstResponse))
	require.NotEmpty(t, firstResponse.ID)
	require.Len(t, firstResponse.Output, 1)
	assert.Equal(t, "function_call", firstResponse.Output[0].Type)

	secondRecorder := fusionStandardResponsesRelayJSONRequest(t, gin.H{
		"model":                "fusion:research",
		"previous_response_id": firstResponse.ID,
		"input": []gin.H{
			{
				"type":    "function_call_output",
				"call_id": "fc_read",
				"output":  "package main\nfunc main() {}",
			},
		},
	}, 1)

	require.Equal(t, http.StatusOK, secondRecorder.Code)
	assert.True(t, seenSecondRound.Load())
	assert.Equal(t, int32(3), calls.Load())
	var secondResponse dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(secondRecorder.Body.Bytes(), &secondResponse))
	assert.True(t, strings.HasPrefix(secondResponse.ID, "resp_fusion_"))
	require.Len(t, secondResponse.Output, 1)
	assert.Equal(t, "message", secondResponse.Output[0].Type)
}

func TestResponsesFusionPreviousResponseIDRejectsMissingState(t *testing.T) {
	setupFusionControllerTestDB(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"fusion_setting.enabled":              "true",
		"fusion_setting.allowed_token_groups": `["default"]`,
	}))
	recorder := fusionStandardResponsesRelayJSONRequest(t, gin.H{
		"model":                "fusion:research",
		"previous_response_id": "resp_missing",
		"input": []gin.H{
			{
				"type":    "function_call_output",
				"call_id": "call_read",
				"output":  "result",
			},
		},
	}, 1)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Fusion previous_response_id state not found or expired")
}

func TestResponsesFusionPreservesFunctionCallOutputForCandidates(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServerWithRequestCheck(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionRelayResponse("candidate-a", "candidate saw tool output", 10, 2),
		"judge-model": fusionRelayResponse("judge-model", "final answer", 5, 3),
	}, func(t *testing.T, request dto.GeneralOpenAIRequest) {
		if request.Model != "candidate-a" {
			return
		}
		require.Len(t, request.Messages, 4)
		assert.Equal(t, "user", request.Messages[0].Role)
		assert.Equal(t, "read index.html", request.Messages[0].StringContent())

		assert.Equal(t, "assistant", request.Messages[1].Role)
		toolCalls := request.Messages[1].ParseToolCalls()
		require.Len(t, toolCalls, 1)
		assert.Equal(t, "call_read", toolCalls[0].ID)
		assert.Equal(t, "read_file", toolCalls[0].Function.Name)
		assert.Equal(t, `{"path":"index.html"}`, toolCalls[0].Function.Arguments)

		assert.Equal(t, "tool", request.Messages[2].Role)
		assert.Equal(t, "call_read", request.Messages[2].ToolCallId)
		assert.Contains(t, request.Messages[2].StringContent(), "<title>Home</title>")

		assert.Equal(t, "user", request.Messages[3].Role)
		assert.Equal(t, "summarize it", request.Messages[3].StringContent())
	})
	configureFusionRelayServer(t, server.URL, nil)
	initialQuota := common.GetTrustQuota() + 1000
	seedFusionRelayUserAndToken(t, 1, initialQuota, initialQuota)
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	createFusionRelayConfig(t, 1, key)

	recorder := fusionStandardResponsesRelayJSONRequest(t, gin.H{
		"model": "fusion:research",
		"input": []gin.H{
			{
				"type": "message",
				"role": "user",
				"content": []gin.H{
					{"type": "input_text", "text": "read index.html"},
				},
			},
			{
				"type":      "function_call",
				"call_id":   "call_read",
				"name":      "read_file",
				"arguments": `{"path":"index.html"}`,
			},
			{
				"type":    "function_call_output",
				"call_id": "call_read",
				"output":  "<html><title>Home</title></html>",
			},
			{
				"type": "message",
				"role": "user",
				"content": []gin.H{
					{"type": "input_text", "text": "summarize it"},
				},
			},
		},
	}, 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, int32(2), calls.Load())
}

func TestResponsesFusionToolCallStreamReturnsFunctionCallEvents(t *testing.T) {
	setupFusionControllerTestDB(t)
	server, calls := newFusionRelayTLSServer(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionRelayToolCallResponse("candidate-a", "call_read", "read_file", `{"path":"main.go"}`, 10, 1),
		"judge-model": fusionRelayResponse("judge-model", "should not run", 5, 3),
	})
	configureFusionRelayServer(t, server.URL, nil)
	initialQuota := common.GetTrustQuota() + 1000
	seedFusionRelayUserAndToken(t, 1, initialQuota, initialQuota)
	key := createFusionRelayKey(t, 1, server.URL+"/v1")
	createFusionRelayConfig(t, 1, key)

	recorder := fusionStandardResponsesRelayJSONRequest(t, gin.H{
		"model":  "fusion:research",
		"input":  "read main.go",
		"stream": true,
		"tools": []gin.H{
			{
				"type": "function",
				"function": gin.H{
					"name": "read_file",
				},
			},
		},
	}, 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	assert.Equal(t, int32(1), calls.Load())
	body := recorder.Body.String()
	expectedEvents := []string{
		"response.created",
		"response.in_progress",
		"response.output_item.added",
		"response.function_call_arguments.delta",
		"response.function_call_arguments.done",
		"response.output_item.done",
		"response.completed",
	}
	lastEventIndex := -1
	for _, eventType := range expectedEvents {
		eventIndex := strings.Index(body, "event: "+eventType)
		require.NotEqual(t, -1, eventIndex, eventType)
		assert.Greater(t, eventIndex, lastEventIndex, eventType)
		lastEventIndex = eventIndex
	}
	assert.Contains(t, body, "function_call")
	assert.Contains(t, body, "call_read")
	assert.Contains(t, body, "read_file")
	idStart := strings.Index(body, `"id":"resp_fusion_`)
	require.NotEqual(t, -1, idStart)
	idStart += len(`"id":"`)
	idEnd := strings.Index(body[idStart:], `"`)
	require.NotEqual(t, -1, idEnd)
	responseID := body[idStart : idStart+idEnd]
	_, err := model.GetFusionResponseState(responseID, 1, 10)
	require.NoError(t, err)
}
