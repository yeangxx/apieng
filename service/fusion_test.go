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
	"sync/atomic"
	"testing"
	"time"

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

type fusionTestServerState struct {
	mu          sync.Mutex
	modelCalls  map[string]int
	modelBodies map[string][]dto.GeneralOpenAIRequest
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
	require.NoError(t, db.AutoMigrate(&model.FusionUpstreamTemplate{}, &model.FusionAPIKey{}, &model.FusionConfig{}, &model.FusionResponseState{}))
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
	state := &fusionTestServerState{
		modelCalls:  map[string]int{},
		modelBodies: map[string][]dto.GeneralOpenAIRequest{},
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer sk-fusion-test", r.Header.Get("Authorization"))

		var req dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &req))
		state.mu.Lock()
		state.modelCalls[req.Model]++
		state.modelBodies[req.Model] = append(state.modelBodies[req.Model], req)
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

func TestValidateFusionChatRequestRejectsMissingToolCallHistory(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "fusion:research",
		Messages: []dto.Message{
			{
				Role:       "tool",
				ToolCallId: "call_read",
				Content:    "file content",
			},
		},
	}

	err := validateFusionChatRequest(request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing tool call id")
}

func TestValidateFusionChatRequestAcceptsToolCallHistory(t *testing.T) {
	assistant := dto.Message{
		Role:    "assistant",
		Content: "",
	}
	assistant.SetToolCalls([]dto.ToolCallResponse{
		{
			ID:   "call_read",
			Type: "function",
			Function: dto.FunctionResponse{
				Name:      "read_file",
				Arguments: `{"path":"main.go"}`,
			},
		},
	})
	request := &dto.GeneralOpenAIRequest{
		Model: "fusion:research",
		Messages: []dto.Message{
			{Role: "user", Content: "read file"},
			assistant,
			{Role: "tool", ToolCallId: "call_read", Content: "file content"},
		},
	}

	require.NoError(t, validateFusionChatRequest(request))
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

func createFusionServiceTemplate(t *testing.T) *model.FusionUpstreamTemplate {
	t.Helper()
	template := &model.FusionUpstreamTemplate{
		Name:                 "Gateway",
		ProviderLabel:        "Gateway",
		Protocol:             model.FusionProtocolOpenAIChatCompatible,
		EndpointPath:         "/gateway/chat",
		AuthType:             model.FusionAuthTypeBearer,
		AuthHeader:           "Authorization",
		DefaultHeaders:       `{"X-Template":"template"}`,
		DefaultQuery:         `{"template":"1"}`,
		DefaultBodyOverrides: `{"template_flag":true}`,
		DetectRules:          `[]`,
		Enabled:              true,
	}
	require.NoError(t, template.Insert())
	return template
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

func fusionToolCallResponse(modelName string, callID string, toolName string, arguments string, promptTokens int, completionTokens int) dto.OpenAITextResponse {
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

func writeFusionTestStreamChunk(t *testing.T, w http.ResponseWriter, chunk gin.H) {
	t.Helper()
	data, err := common.Marshal(chunk)
	require.NoError(t, err)
	_, err = fmt.Fprintf(w, "data: %s\n\n", string(data))
	require.NoError(t, err)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func writeFusionTestStreamDone(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	_, err := fmt.Fprint(w, "data: [DONE]\n\n")
	require.NoError(t, err)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
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

func fusionEngineToolTestRequest(config *model.FusionConfig, client *http.Client) FusionEngineRequest {
	request := fusionEngineTestRequest(config, client)
	request.Request.Tools = []dto.ToolCallRequest{
		{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        "read_file",
				Description: "Read a local file",
				Parameters: map[string]any{
					"type": "object",
				},
			},
		},
	}
	request.Request.ToolChoice = map[string]any{
		"type": "function",
		"function": map[string]any{
			"name": "read_file",
		},
	}
	return request
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

func TestFusionEngineAppliesUpstreamTemplateAndKeyConfig(t *testing.T) {
	setupFusionServiceTestDB(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/custom/chat", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("template"))
		require.Equal(t, "abc", r.URL.Query().Get("tenant"))
		require.Equal(t, "template", r.Header.Get("X-Template"))
		require.Equal(t, "user", r.Header.Get("X-User"))
		require.Equal(t, "Bearer sk-fusion-test", r.Header.Get("Authorization"))

		var payload map[string]interface{}
		require.NoError(t, common.DecodeJson(r.Body, &payload))
		assert.Equal(t, true, payload["template_flag"])
		assert.Equal(t, "enabled", payload["custom_flag"])
		modelName, _ := payload["model"].(string)
		response := fusionTestResponse(modelName, "ok "+modelName, 2, 1)
		if strings.Contains(modelName, "judge") {
			response = fusionTestResponse(modelName, "final", 3, 1)
		}
		data, err := common.Marshal(response)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(data)
		require.NoError(t, err)
	}))
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	template := createFusionServiceTemplate(t)
	key := createFusionServiceKey(t, 1, "candidate-a", server.URL, "candidate-a")
	key.TemplateID = template.Id
	require.NoError(t, key.SetUpstreamConfig(`{
		"endpoint_path": "/custom/chat",
		"headers": {"X-User": "user"},
		"query": {"tenant": "abc"},
		"body_overrides": {"custom_flag": "enabled"}
	}`))
	require.NoError(t, key.Update())
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL, "judge-model")
	judgeKey.TemplateID = template.Id
	require.NoError(t, judgeKey.SetUpstreamConfig(key.UpstreamConfig))
	require.NoError(t, judgeKey.Update())
	fusionConfig := createFusionServiceConfig(t, 1, []int{key.Id}, judgeKey.Id, "judge-model", 1)

	result, err := RunFusionEngine(context.Background(), fusionEngineTestRequest(fusionConfig, server.Client()))

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "final", result.Content)
}

func TestFusionEngineReturnsFirstCandidateToolCallWithoutJudge(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, state := newFusionTestTLSServer(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionToolCallResponse("candidate-a", "call_a", "read_file", `{"path":"a.go"}`, 10, 1),
		"candidate-b": fusionTestResponse("candidate-b", "answer B", 11, 6),
		"judge-model": fusionTestResponse("judge-model", "should not run", 30, 7),
	}, nil)
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	keyA := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	keyB := createFusionServiceKey(t, 1, "candidate-b", server.URL+"/v1", "candidate-b")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{keyA.Id, keyB.Id}, judgeKey.Id, "judge-model", 2)

	result, err := RunFusionEngine(context.Background(), fusionEngineToolTestRequest(fusionConfig, server.Client()))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.ToolCalls, 1)
	assert.Equal(t, "call_a", result.ToolCalls[0].ID)
	assert.Equal(t, "read_file", result.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"path":"a.go"}`, result.ToolCalls[0].Function.Arguments)
	assert.False(t, result.Judge.Success)
	assert.Equal(t, 21, result.Usage.PromptTokens)
	assert.Equal(t, 7, result.Usage.CompletionTokens)
	state.mu.Lock()
	judgeCalls := state.modelCalls["judge-model"]
	candidateARequests := state.modelBodies["candidate-a"]
	state.mu.Unlock()
	assert.Equal(t, 0, judgeCalls)
	require.Len(t, candidateARequests, 1)
	require.Len(t, candidateARequests[0].Tools, 1)
	assert.Equal(t, "read_file", candidateARequests[0].Tools[0].Function.Name)
	require.NotNil(t, candidateARequests[0].ToolChoice)
}

func TestFusionEngineReturnsSecondCandidateToolCallWhenFirstIsText(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, state := newFusionTestTLSServer(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionTestResponse("candidate-a", "answer A", 10, 5),
		"candidate-b": fusionToolCallResponse("candidate-b", "call_b", "run_tests", `{"cmd":"go test ./..."}`, 11, 1),
		"judge-model": fusionTestResponse("judge-model", "should not run", 30, 7),
	}, nil)
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	keyA := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	keyB := createFusionServiceKey(t, 1, "candidate-b", server.URL+"/v1", "candidate-b")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{keyA.Id, keyB.Id}, judgeKey.Id, "judge-model", 2)

	result, err := RunFusionEngine(context.Background(), fusionEngineToolTestRequest(fusionConfig, server.Client()))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.ToolCalls, 1)
	assert.Equal(t, "call_b", result.ToolCalls[0].ID)
	assert.Equal(t, "run_tests", result.ToolCalls[0].Function.Name)
	state.mu.Lock()
	judgeCalls := state.modelCalls["judge-model"]
	state.mu.Unlock()
	assert.Equal(t, 0, judgeCalls)
}

func TestFusionEngineChoosesFirstConfiguredToolCallCandidate(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, state := newFusionTestTLSServer(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionToolCallResponse("candidate-a", "call_a", "read_file", `{"path":"a.go"}`, 10, 1),
		"candidate-b": fusionToolCallResponse("candidate-b", "call_b", "run_tests", `{"cmd":"go test ./..."}`, 11, 1),
		"judge-model": fusionTestResponse("judge-model", "should not run", 30, 7),
	}, nil)
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	keyA := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	keyB := createFusionServiceKey(t, 1, "candidate-b", server.URL+"/v1", "candidate-b")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{keyA.Id, keyB.Id}, judgeKey.Id, "judge-model", 2)

	result, err := RunFusionEngine(context.Background(), fusionEngineToolTestRequest(fusionConfig, server.Client()))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.ToolCalls, 1)
	assert.Equal(t, "call_a", result.ToolCalls[0].ID)
	state.mu.Lock()
	judgeCalls := state.modelCalls["judge-model"]
	state.mu.Unlock()
	assert.Equal(t, 0, judgeCalls)
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
	assert.Contains(t, err.Error(), "candidate failures")
	assert.Contains(t, err.Error(), "candidate-b")
	assert.Contains(t, err.Error(), "status=401")
	assert.Contains(t, err.Error(), "upstream returned status 401")
	state.mu.Lock()
	judgeCalls := state.modelCalls["judge-model"]
	state.mu.Unlock()
	assert.Equal(t, 0, judgeCalls)

	billingInput := BuildFusionBillingInput(result)
	assert.Equal(t, 1, billingInput.SuccessfulCandidates)
	assert.Equal(t, 1, billingInput.FailedCandidates)
	assert.Greater(t, billingInput.FailedPromptTokens, 0)
}

func TestFusionEngineSanitizesCandidateTimeoutErrors(t *testing.T) {
	setupFusionServiceTestDB(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &req))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(50 * time.Millisecond)
		data, err := common.Marshal(fusionTestResponse(req.Model, "late answer", 10, 5))
		require.NoError(t, err)
		_, _ = w.Write(data)
	}))
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	key := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{key.Id}, judgeKey.Id, "judge-model", 1)
	fusionConfig.TimeoutMS = 10

	result, err := RunFusionEngine(context.Background(), fusionEngineTestRequest(fusionConfig, server.Client()))

	require.Error(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Candidates, 1)
	assert.Contains(t, err.Error(), "upstream request timeout")
	assert.NotContains(t, err.Error(), "context deadline exceeded")
	assert.Contains(t, result.Candidates[0].SanitizedError, "upstream request timeout")
	assert.NotContains(t, result.Candidates[0].SanitizedError, "context deadline exceeded")
	assert.Equal(t, http.StatusOK, result.Candidates[0].UpstreamStatus)
}

func TestFusionEngineStreamsAgentCandidateToAvoidBodyTimeout(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, state := newFusionTestTLSServer(t, nil, nil)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer sk-fusion-test", r.Header.Get("Authorization"))
		var req dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &req))
		state.mu.Lock()
		state.modelCalls[req.Model]++
		state.modelBodies[req.Model] = append(state.modelBodies[req.Model], req)
		state.mu.Unlock()

		if req.Model == "judge-model" {
			response := fusionTestResponse("judge-model", "final synthesis", 10, 2)
			data, err := common.Marshal(response)
			require.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(data)
			require.NoError(t, err)
			return
		}

		if req.Stream == nil || !*req.Stream {
			time.Sleep(300 * time.Millisecond)
			response := fusionTestResponse(req.Model, "late non-stream answer", 10, 2)
			data, err := common.Marshal(response)
			require.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(data)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		writeFusionTestStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-stream-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{"role": "assistant", "content": "candidate answer"}, "finish_reason": nil}},
		})
		writeFusionTestStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-stream-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{}, "finish_reason": "stop"}},
			"usage": gin.H{
				"prompt_tokens":     10,
				"completion_tokens": 2,
				"total_tokens":      12,
			},
		})
		writeFusionTestStreamDone(t, w)
	})
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	candidateKey := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{candidateKey.Id}, judgeKey.Id, "judge-model", 1)
	fusionConfig.TimeoutMS = 100
	require.NoError(t, model.DB.Model(fusionConfig).Update("timeout_ms", fusionConfig.TimeoutMS).Error)

	result, err := RunFusionEngine(context.Background(), fusionEngineToolTestRequest(fusionConfig, server.Client()))

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "final synthesis", result.Content)
	require.Len(t, result.Candidates, 1)
	assert.True(t, result.Candidates[0].Success)
	state.mu.Lock()
	candidateBodies := append([]dto.GeneralOpenAIRequest(nil), state.modelBodies["candidate-a"]...)
	state.mu.Unlock()
	require.Len(t, candidateBodies, 1)
	require.NotNil(t, candidateBodies[0].Stream)
	assert.True(t, *candidateBodies[0].Stream)
}

func TestFusionEngineAllowsActiveStreamBeyondConfiguredIdleTimeout(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, state := newFusionTestTLSServer(t, nil, nil)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-fusion-test", r.Header.Get("Authorization"))
		var req dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &req))
		state.mu.Lock()
		state.modelCalls[req.Model]++
		state.modelBodies[req.Model] = append(state.modelBodies[req.Model], req)
		state.mu.Unlock()
		require.NotNil(t, req.Stream)
		require.True(t, *req.Stream)

		w.Header().Set("Content-Type", "text/event-stream")
		if req.Model == "candidate-a" {
			writeFusionTestStreamChunk(t, w, gin.H{
				"id":      "chatcmpl-active-stream-test",
				"object":  "chat.completion.chunk",
				"created": common.GetTimestamp(),
				"model":   req.Model,
				"choices": []gin.H{{"index": 0, "delta": gin.H{"role": "assistant", "content": "part1 "}, "finish_reason": nil}},
			})
			time.Sleep(25 * time.Millisecond)
			writeFusionTestStreamChunk(t, w, gin.H{
				"id":      "chatcmpl-active-stream-test",
				"object":  "chat.completion.chunk",
				"created": common.GetTimestamp(),
				"model":   req.Model,
				"choices": []gin.H{{"index": 0, "delta": gin.H{"content": "part2 "}, "finish_reason": nil}},
			})
			time.Sleep(25 * time.Millisecond)
			writeFusionTestStreamChunk(t, w, gin.H{
				"id":      "chatcmpl-active-stream-test",
				"object":  "chat.completion.chunk",
				"created": common.GetTimestamp(),
				"model":   req.Model,
				"choices": []gin.H{{"index": 0, "delta": gin.H{"content": "part3"}, "finish_reason": "stop"}},
				"usage": gin.H{
					"prompt_tokens":     10,
					"completion_tokens": 3,
					"total_tokens":      13,
				},
			})
			writeFusionTestStreamDone(t, w)
			return
		}

		writeFusionTestStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-active-judge-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{"role": "assistant", "content": "final after active stream"}, "finish_reason": "stop"}},
			"usage": gin.H{
				"prompt_tokens":     10,
				"completion_tokens": 2,
				"total_tokens":      12,
			},
		})
		writeFusionTestStreamDone(t, w)
	})
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	candidateKey := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{candidateKey.Id}, judgeKey.Id, "judge-model", 1)
	fusionConfig.TimeoutMS = 40
	require.NoError(t, model.DB.Model(fusionConfig).Update("timeout_ms", fusionConfig.TimeoutMS).Error)
	request := fusionEngineTestRequest(fusionConfig, server.Client())
	request.Request.Stream = common.GetPointer(true)

	result, err := RunFusionEngine(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "final after active stream", result.Content)
	require.Len(t, result.Candidates, 1)
	assert.True(t, result.Candidates[0].Success)
	assert.Equal(t, "part1 part2 part3", result.Candidates[0].Content)
}

func TestFusionEngineStreamsJudgeForExternalStreamRequest(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, state := newFusionTestTLSServer(t, nil, nil)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-fusion-test", r.Header.Get("Authorization"))
		var req dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &req))
		state.mu.Lock()
		state.modelCalls[req.Model]++
		state.modelBodies[req.Model] = append(state.modelBodies[req.Model], req)
		state.mu.Unlock()

		if req.Stream == nil || !*req.Stream {
			time.Sleep(300 * time.Millisecond)
			response := fusionTestResponse(req.Model, "late non-stream answer", 10, 2)
			data, err := common.Marshal(response)
			require.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(data)
			return
		}

		content := "candidate answer"
		if req.Model == "judge-model" {
			content = "streamed final synthesis"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		writeFusionTestStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-stream-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{"role": "assistant", "content": content}, "finish_reason": nil}},
		})
		writeFusionTestStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-stream-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{}, "finish_reason": "stop"}},
			"usage": gin.H{
				"prompt_tokens":     10,
				"completion_tokens": 2,
				"total_tokens":      12,
			},
		})
		writeFusionTestStreamDone(t, w)
	})
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	candidateKey := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{candidateKey.Id}, judgeKey.Id, "judge-model", 1)
	fusionConfig.TimeoutMS = 100
	require.NoError(t, model.DB.Model(fusionConfig).Update("timeout_ms", fusionConfig.TimeoutMS).Error)
	request := fusionEngineTestRequest(fusionConfig, server.Client())
	request.Request.Stream = common.GetPointer(true)

	result, err := RunFusionEngine(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "streamed final synthesis", result.Content)
	state.mu.Lock()
	defer state.mu.Unlock()
	require.Len(t, state.modelBodies["candidate-a"], 1)
	require.Len(t, state.modelBodies["judge-model"], 1)
	require.NotNil(t, state.modelBodies["candidate-a"][0].Stream)
	require.NotNil(t, state.modelBodies["judge-model"][0].Stream)
	assert.True(t, *state.modelBodies["candidate-a"][0].Stream)
	assert.True(t, *state.modelBodies["judge-model"][0].Stream)
}

func TestFusionEngineStreamFinalStartsJudgeAfterMinSuccesses(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, state := newFusionTestTLSServer(t, nil, nil)
	var slowCandidateCanceled atomic.Bool
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-fusion-test", r.Header.Get("Authorization"))
		var req dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &req))
		state.mu.Lock()
		state.modelCalls[req.Model]++
		state.modelBodies[req.Model] = append(state.modelBodies[req.Model], req)
		state.mu.Unlock()
		require.NotNil(t, req.Stream)
		require.True(t, *req.Stream)

		if req.Model == "candidate-slow" {
			<-r.Context().Done()
			slowCandidateCanceled.Store(true)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		content := "fast candidate"
		if req.Model == "judge-model" {
			content = "streamed final"
		}
		writeFusionTestStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-min-success-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{"role": "assistant", "content": content}, "finish_reason": nil}},
		})
		writeFusionTestStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-min-success-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{}, "finish_reason": "stop"}},
			"usage": gin.H{
				"prompt_tokens":     10,
				"completion_tokens": 2,
				"total_tokens":      12,
			},
		})
		writeFusionTestStreamDone(t, w)
	})
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	fastKey := createFusionServiceKey(t, 1, "fast", server.URL+"/v1", "candidate-fast")
	slowKey := createFusionServiceKey(t, 1, "slow", server.URL+"/v1", "candidate-slow")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{fastKey.Id, slowKey.Id}, judgeKey.Id, "judge-model", 1)
	fusionConfig.TimeoutMS = 5000
	require.NoError(t, model.DB.Model(fusionConfig).Update("timeout_ms", fusionConfig.TimeoutMS).Error)
	request := fusionEngineTestRequest(fusionConfig, server.Client())
	request.Request.Stream = common.GetPointer(true)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var deltas []string
	result, err := RunFusionEngineStreamFinal(ctx, request, FusionStreamCallbacks{
		OnTextDelta: func(delta string) error {
			deltas = append(deltas, delta)
			return nil
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "streamed final", result.Content)
	assert.Equal(t, []string{"streamed final"}, deltas)
	require.Len(t, result.Candidates, 1)
	assert.Equal(t, "candidate-fast", result.Candidates[0].Model)
	state.mu.Lock()
	defer state.mu.Unlock()
	assert.Equal(t, 1, state.modelCalls["candidate-fast"])
	assert.Equal(t, 1, state.modelCalls["candidate-slow"])
	assert.Equal(t, 1, state.modelCalls["judge-model"])
	assert.True(t, slowCandidateCanceled.Load())
}

func TestFusionEngineStreamFinalUsesBriefCandidateRequests(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, state := newFusionTestTLSServer(t, nil, nil)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-fusion-test", r.Header.Get("Authorization"))
		var req dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &req))
		state.mu.Lock()
		state.modelCalls[req.Model]++
		state.modelBodies[req.Model] = append(state.modelBodies[req.Model], req)
		state.mu.Unlock()
		require.NotNil(t, req.Stream)
		require.True(t, *req.Stream)

		w.Header().Set("Content-Type", "text/event-stream")
		content := "candidate brief"
		if req.Model == "judge-model" {
			content = "streamed final html"
		}
		writeFusionTestStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-brief-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{"role": "assistant", "content": content}, "finish_reason": nil}},
		})
		writeFusionTestStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-brief-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{}, "finish_reason": "stop"}},
			"usage": gin.H{
				"prompt_tokens":     10,
				"completion_tokens": 2,
				"total_tokens":      12,
			},
		})
		writeFusionTestStreamDone(t, w)
	})
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, map[string]string{
		"fusion_setting.stream_candidate_brief":      "true",
		"fusion_setting.stream_candidate_max_tokens": "384",
	})

	candidateKey := createFusionServiceKey(t, 1, "candidate", server.URL+"/v1", "candidate-a")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{candidateKey.Id}, judgeKey.Id, "judge-model", 1)
	request := fusionEngineTestRequest(fusionConfig, server.Client())
	request.Request.Stream = common.GetPointer(true)
	request.Request.Messages = []dto.Message{
		{Role: "user", Content: "Create a complete HTML landing page."},
	}

	result, err := RunFusionEngineStreamFinal(context.Background(), request, FusionStreamCallbacks{})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "streamed final html", result.Content)
	state.mu.Lock()
	defer state.mu.Unlock()
	require.Len(t, state.modelBodies["candidate-a"], 1)
	candidateRequest := state.modelBodies["candidate-a"][0]
	require.NotEmpty(t, candidateRequest.Messages)
	assert.Equal(t, "system", candidateRequest.Messages[0].Role)
	assert.Contains(t, candidateRequest.Messages[0].StringContent(), "Fusion candidate brief")
	require.NotNil(t, candidateRequest.MaxTokens)
	assert.Equal(t, uint(384), *candidateRequest.MaxTokens)

	require.Len(t, state.modelBodies["judge-model"], 1)
	judgeRequest := state.modelBodies["judge-model"][0]
	require.NotEmpty(t, judgeRequest.Messages)
	assert.NotContains(t, judgeRequest.Messages[0].StringContent(), "Fusion candidate brief")
	assert.Nil(t, judgeRequest.MaxTokens)
}

func TestFusionEngineParsesStreamingToolCallCandidate(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, state := newFusionTestTLSServer(t, nil, nil)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-fusion-test", r.Header.Get("Authorization"))
		var req dto.GeneralOpenAIRequest
		require.NoError(t, common.DecodeJson(r.Body, &req))
		state.mu.Lock()
		state.modelCalls[req.Model]++
		state.modelBodies[req.Model] = append(state.modelBodies[req.Model], req)
		state.mu.Unlock()

		require.Equal(t, "candidate-a", req.Model)
		require.NotNil(t, req.Stream)
		require.True(t, *req.Stream)

		w.Header().Set("Content-Type", "text/event-stream")
		writeFusionTestStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-tool-stream-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{"role": "assistant"}, "finish_reason": nil}},
		})
		writeFusionTestStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-tool-stream-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{"tool_calls": []gin.H{{"index": 0, "id": "call_read", "type": "function", "function": gin.H{"name": "read_file", "arguments": ""}}}}, "finish_reason": nil}},
		})
		writeFusionTestStreamChunk(t, w, gin.H{
			"id":      "chatcmpl-tool-stream-test",
			"object":  "chat.completion.chunk",
			"created": common.GetTimestamp(),
			"model":   req.Model,
			"choices": []gin.H{{"index": 0, "delta": gin.H{"tool_calls": []gin.H{{"index": 0, "function": gin.H{"arguments": `{"path":"index.html"}`}}}}, "finish_reason": "tool_calls"}},
			"usage": gin.H{
				"prompt_tokens":     10,
				"completion_tokens": 1,
				"total_tokens":      11,
			},
		})
		writeFusionTestStreamDone(t, w)
	})
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	candidateKey := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{candidateKey.Id}, judgeKey.Id, "judge-model", 1)

	result, err := RunFusionEngine(context.Background(), fusionEngineToolTestRequest(fusionConfig, server.Client()))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.ToolCalls, 1)
	assert.Equal(t, "call_read", result.ToolCalls[0].ID)
	assert.Equal(t, "read_file", result.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"path":"index.html"}`, result.ToolCalls[0].Function.Arguments)
	assert.Empty(t, result.Content)
	state.mu.Lock()
	defer state.mu.Unlock()
	assert.Equal(t, 1, state.modelCalls["candidate-a"])
	assert.Zero(t, state.modelCalls["judge-model"])
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

func TestFusionJudgePromptSkipsClientSystemIdentity(t *testing.T) {
	setupFusionServiceTestDB(t)
	server, state := newFusionTestTLSServer(t, map[string]dto.OpenAITextResponse{
		"candidate-a": fusionTestResponse("candidate-a", "candidate answer", 10, 5),
		"judge-model": fusionTestResponse("judge-model", "final", 30, 7),
	}, nil)
	defer server.Close()
	configureFusionServiceTestBaseURL(t, server.URL, nil)

	keyA := createFusionServiceKey(t, 1, "candidate-a", server.URL+"/v1", "candidate-a")
	judgeKey := createFusionServiceKey(t, 1, "judge", server.URL+"/v1", "judge-model")
	fusionConfig := createFusionServiceConfig(t, 1, []int{keyA.Id}, judgeKey.Id, "judge-model", 1)

	request := fusionEngineTestRequest(fusionConfig, server.Client())
	request.Request.Messages = []dto.Message{
		{
			Role:    "system",
			Content: "You are GPT-5.1 running in Codex CLI. The current working directory is F:\\workspace\\testspace\\test1.",
		},
		{
			Role:    "user",
			Content: "你是谁",
		},
	}
	result, err := RunFusionEngine(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, result)
	state.mu.Lock()
	require.Len(t, state.judgeBodies, 1)
	judgeBody := state.judgeBodies[0]
	state.mu.Unlock()
	assert.Contains(t, judgeBody, "user: 你是谁")
	assert.NotContains(t, judgeBody, "GPT-5.1")
	assert.NotContains(t, judgeBody, "Codex CLI")
	assert.NotContains(t, judgeBody, "F:\\workspace\\testspace\\test1")
}

func TestFusionEngineRejectsUnsupportedRequestFields(t *testing.T) {
	setupFusionServiceTestDB(t)
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
			Model:        "fusion:research",
			FunctionCall: []byte(`{"name":"legacy"}`),
			Messages:     []dto.Message{{Role: "user", Content: "hi"}},
		},
		HTTPClient: http.DefaultClient,
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "function_call")
}
