package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fusionTestServerState struct {
	mu          sync.Mutex
	modelCalls  map[string]int
	judgeBodies []string
}

func setupFusionServiceTestDB(t *testing.T) {
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
}

func newFusionTestTLSServer(t *testing.T, responses map[string]dto.OpenAITextResponse, failures map[string]int) (*httptest.Server, *fusionTestServerState) {
	t.Helper()
	state := &fusionTestServerState{modelCalls: map[string]int{}}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer sk-fusion-test", r.Header.Get("Authorization"))

		var req dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &req))
		state.mu.Lock()
		state.modelCalls[req.Model]++
		if strings.Contains(req.Model, "judge") {
			body, _ := common.Marshal(req)
			state.judgeBodies = append(state.judgeBodies, string(body))
		}
		state.mu.Unlock()

		if status, ok := failures[req.Model]; ok {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"candidate failed"}}`))
			return
		}
		resp, ok := responses[req.Model]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"unknown model"}}`))
			return
		}
		data, err := common.Marshal(resp)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	return server, state
}

func configureFusionServiceTestBaseURL(t *testing.T, serverURL string, extra map[string]string) {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	require.NoError(t, err)
	port, err := strconv.Atoi(parsed.Port())
	require.NoError(t, err)
	values := map[string]string{
		"fusion_setting.allow_private_base_url":     "true",
		"fusion_setting.allowed_base_url_ports":     fmt.Sprintf("[%d]", port),
		"fusion_setting.max_candidate_output_chars": "20000",
		"fusion_setting.max_judge_input_tokens":     "128000",
	}
	for key, value := range extra {
		values[key] = value
	}
	require.NoError(t, config.GlobalConfig.LoadFromDB(values))
}

func createFusionServiceKey(t *testing.T, userID int, name string, baseURL string, defaultModel string) *model.FusionAPIKey {
	t.Helper()
	key := &model.FusionAPIKey{
		UserId:       userID,
		Name:         name,
		Provider:     model.FusionProviderOpenAICompatible,
		BaseURL:      baseURL,
		DefaultModel: defaultModel,
		Status:       model.FusionKeyStatusEnabled,
	}
	require.NoError(t, key.SetPlainAPIKey("sk-fusion-test"))
	require.NoError(t, key.SetModels([]string{defaultModel}))
	require.NoError(t, key.Insert())
	return key
}

func createFusionServiceConfig(t *testing.T, userID int, candidateIDs []int, judgeKeyID int, judgeModel string, minSuccesses int) *model.FusionConfig {
	t.Helper()
	config := &model.FusionConfig{
		UserId:       userID,
		Name:         "Research",
		ModelAlias:   "fusion:research",
		Enabled:      true,
		JudgeKeyID:   judgeKeyID,
		JudgeModel:   judgeModel,
		Strategy:     model.FusionStrategySynthesize,
		TimeoutMS:    5000,
		MaxParallel:  2,
		MinSuccesses: minSuccesses,
	}
	require.NoError(t, config.SetCandidateKeyIDs(candidateIDs))
	require.NoError(t, config.SetCandidateModels(nil))
	require.NoError(t, config.Insert())
	return config
}

func fusionTestResponse(modelName string, content string, promptTokens int, completionTokens int) dto.OpenAITextResponse {
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

func fusionEngineTestRequest(config *model.FusionConfig, client *http.Client) FusionEngineRequest {
	return FusionEngineRequest{
		UserID:     1,
		Config:     config,
		HTTPClient: client,
		Request: &dto.GeneralOpenAIRequest{
			Model: "fusion:research",
			Messages: []dto.Message{
				{Role: "user", Content: "Compare the options."},
			},
		},
	}
}

func TestFusionEngineSynthesizesSuccessfulCandidates(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, _ := newFusionTestTLSServer(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionTestResponse("candidate-a", "answer A", 10, 5),
		"candidate-b": fusionTestResponse("candidate-b", "answer B", 11, 6),
		"judge-model": fusionTestResponse("judge-model", "final synthesis", 30, 7),
	}, nil)
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	keyA := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	keyB := createFusionServiceKey(t, 1, "candidate-b", server.URL+"/v1", "candidate-b")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{keyA.Id, keyB.Id}, judgeKey.Id, "judge-model", 2)

	result, err := RunFusionEngine(context.Background(), fusionEngineTestRequest(fusionConfig, server.Client()))

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "final synthesis", result.Content)
	require.Len(t, result.Candidates, 2)
	assert.True(t, result.Candidates[0].Success)
	assert.True(t, result.Candidates[1].Success)
	assert.True(t, result.Judge.Success)
	assert.Equal(t, 51, result.Usage.PromptTokens)
	assert.Equal(t, 18, result.Usage.CompletionTokens)

	billingInput := BuildFusionBillingInput(result)
	assert.Equal(t, 2, billingInput.SuccessfulCandidates)
	assert.Equal(t, 21, billingInput.CandidatePromptTokens)
	assert.Equal(t, 11, billingInput.CandidateCompletionTokens)
	assert.Equal(t, 30, billingInput.JudgePromptTokens)
	assert.Equal(t, 7, billingInput.JudgeCompletionTokens)
}

func TestFusionEngineFailsBeforeJudgeWhenMinimumSuccessesNotMet(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, state := newFusionTestTLSServer(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionTestResponse("candidate-a", "answer A", 10, 5),
		"judge-model": fusionTestResponse("judge-model", "should not run", 30, 7),
	}, map[string]int{
		"candidate-b": http.StatusUnauthorized,
	})
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	keyA := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	keyB := createFusionServiceKey(t, 1, "candidate-b", server.URL+"/v1", "candidate-b")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{keyA.Id, keyB.Id}, judgeKey.Id, "judge-model", 2)

	result, err := RunFusionEngine(context.Background(), fusionEngineTestRequest(fusionConfig, server.Client()))

	require.Error(t, err)
	require.NotNil(t, result)
	assert.Contains(t, err.Error(), "minimum successes")
	state.mu.Lock()
	judgeCalls := state.modelCalls["judge-model"]
	state.mu.Unlock()
	assert.Equal(t, 0, judgeCalls)

	billingInput := BuildFusionBillingInput(result)
	assert.Equal(t, 1, billingInput.SuccessfulCandidates)
	assert.Equal(t, 1, billingInput.FailedCandidates)
	assert.Greater(t, billingInput.FailedPromptTokens, 0)
}

func TestFusionEngineRedactsAPIKeyFromUpstreamErrors(t *testing.T) {
	setupFusionServiceTestDB(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad bearer sk-fusion-test"}}`))
	}))
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	key := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{key.Id}, judgeKey.Id, "judge-model", 1)

	result, err := RunFusionEngine(context.Background(), fusionEngineTestRequest(fusionConfig, server.Client()))

	require.Error(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Candidates, 1)
	assert.NotContains(t, result.Candidates[0].SanitizedError, "sk-fusion-test")
	assert.Contains(t, result.Candidates[0].SanitizedError, common.MaskSecret("sk-fusion-test"))
}

func TestFusionProtectedTransportRejectsPrivateConnectAddress(t *testing.T) {
	setupFusionServiceTestDB(t)
	originalLookup := fusionDialLookupIPAddr
	fusionDialLookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		require.Equal(t, "rebind.test", host)
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	t.Cleanup(func() {
		fusionDialLookupIPAddr = originalLookup
	})

	client := fusionNoRedirectClient(&http.Client{Transport: &http.Transport{TLSClientConfig: common.InsecureTLSConfig}})
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://rebind.test/v1/chat/completions", strings.NewReader("{}"))
	require.NoError(t, err)

	resp, err := client.Do(request)
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.Error(t, err)
	assert.Contains(t, err.Error(), "private IP")
}

func TestFusionBillingUsesEstimatedFallbackWhenUsageMissing(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, _ := newFusionTestTLSServer(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionTestResponse("candidate-a", "answer A without usage", 0, 0),
		"judge-model": fusionTestResponse("judge-model", "final without usage", 0, 0),
	}, nil)
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	keyA := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{keyA.Id}, judgeKey.Id, "judge-model", 1)

	result, err := RunFusionEngine(context.Background(), fusionEngineTestRequest(fusionConfig, server.Client()))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Candidates, 1)
	assert.Equal(t, "estimated", result.Candidates[0].Usage.UsageSource)
	assert.Greater(t, result.Candidates[0].Usage.PromptTokens, 0)
	assert.Greater(t, result.Candidates[0].Usage.CompletionTokens, 0)
	assert.Equal(t, "estimated", result.Judge.Usage.UsageSource)
	assert.Greater(t, result.Judge.Usage.PromptTokens, 0)
	assert.Greater(t, result.Judge.Usage.CompletionTokens, 0)
}

func TestFusionPreConsumeCapsCandidateOutputAndJudgeInput(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, state := newFusionTestTLSServer(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionTestResponse("candidate-a", "abcdefghijklmnopqrstuvwxyz", 10, 5),
		"judge-model": fusionTestResponse("judge-model", "final", 30, 7),
	}, nil)
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, map[string]string{
		"fusion_setting.max_candidate_output_chars": "5",
	})

	keyA := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{keyA.Id}, judgeKey.Id, "judge-model", 1)

	result, err := RunFusionEngine(context.Background(), fusionEngineTestRequest(fusionConfig, server.Client()))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Candidates, 1)
	assert.Equal(t, "abcde", result.Candidates[0].Content)

	state.mu.Lock()
	require.Len(t, state.judgeBodies, 1)
	judgeBody := state.judgeBodies[0]
	state.mu.Unlock()
	assert.Contains(t, judgeBody, "abcde")
	assert.NotContains(t, judgeBody, "abcdef")
}

func TestFusionEngineRejectsUnsupportedRequestFields(t *testing.T) {
	setupFusionServiceTestDB(t)
	stream := true
	fusionConfig := &model.FusionConfig{
		UserId:       1,
		ModelAlias:   "fusion:research",
		Strategy:     model.FusionStrategySynthesize,
		MinSuccesses: 1,
	}

	result, err := RunFusionEngine(context.Background(), FusionEngineRequest{
		UserID: 1,
		Config: fusionConfig,
		Request: &dto.GeneralOpenAIRequest{
			Model:    "fusion:research",
			Stream:   &stream,
			Messages: []dto.Message{{Role: "user", Content: "hi"}},
		},
		HTTPClient: http.DefaultClient,
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "stream=true")
}
