package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/fusion_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	fusionNameMaxLength        = 80
	fusionDefaultModelMaxBytes = 128
)

var fusionDirectRequestFields = map[string]struct{}{
	"api_key":           {},
	"base_url":          {},
	"key_id":            {},
	"key_ids":           {},
	"candidate_key_ids": {},
	"judge_key_id":      {},
	"upstream_api_key":  {},
	"upstream_base_url": {},
}

var fusionNestedRequestFields = map[string]struct{}{
	"api_key":           {},
	"base_url":          {},
	"key_id":            {},
	"candidate_key_ids": {},
	"judge_key_id":      {},
}

func bindFusionJSON(c *gin.Context, v any) bool {
	if err := common.UnmarshalBodyReusable(c, v); err != nil {
		common.ApiError(c, err)
		return false
	}
	return true
}

func fusionError(c *gin.Context, status int, err error) {
	if status == http.StatusOK {
		common.ApiError(c, err)
		return
	}
	c.JSON(status, gin.H{
		"success": false,
		"message": err.Error(),
	})
}

func fusionRequestID(c *gin.Context) string {
	requestID := c.GetString(common.RequestIdKey)
	if requestID == "" {
		requestID = common.GetTimeString() + common.GetRandomString(8)
		c.Set(common.RequestIdKey, requestID)
	}
	return requestID
}

func fusionOpenAIError(c *gin.Context, status int, message string, code types.ErrorCode) {
	if code == "" {
		code = types.ErrorCodeInvalidRequest
	}
	c.JSON(status, gin.H{
		"error": types.OpenAIError{
			Message: common.MessageWithRequestId(message, fusionRequestID(c)),
			Type:    "new_api_error",
			Code:    code,
		},
	})
	c.Abort()
}

func fusionNewAPIError(c *gin.Context, err *types.NewAPIError) {
	if err == nil {
		fusionOpenAIError(c, http.StatusInternalServerError, "fusion request failed", types.ErrorCodeBadResponse)
		return
	}
	openAIError := err.ToOpenAIError()
	openAIError.Message = common.MessageWithRequestId(openAIError.Message, fusionRequestID(c))
	status := err.StatusCode
	if status == 0 {
		status = http.StatusInternalServerError
	}
	c.JSON(status, gin.H{"error": openAIError})
	c.Abort()
}

func parseFusionID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, errors.New("invalid fusion id"))
		return 0, false
	}
	return id, true
}

func fusionBaseURLPolicyFromSetting() common.FusionBaseURLPolicy {
	return common.FusionBaseURLPolicy{
		AllowPrivateIP: fusion_setting.IsFusionPrivateBaseURLAllowed(),
		AllowedDomains: fusion_setting.GetFusionAllowedBaseURLDomains(),
		AllowedPorts:   fusion_setting.GetFusionAllowedBaseURLPorts(),
	}
}

func validateFusionExecutionGate(c *gin.Context) bool {
	if !fusion_setting.IsFusionEnabled() {
		fusionError(c, http.StatusForbidden, errors.New("Fusion is disabled"))
		return false
	}
	if !common.HasPersistentCryptoSecret() {
		fusionError(c, http.StatusServiceUnavailable, errors.New("CRYPTO_SECRET is required for Fusion"))
		return false
	}
	return true
}

func rejectFusionDirectCredentialFields(rawBody []byte) error {
	var payload map[string]any
	if err := common.Unmarshal(rawBody, &payload); err != nil {
		return err
	}
	for field := range fusionDirectRequestFields {
		if _, ok := payload[field]; ok {
			return fmt.Errorf("fusion request cannot include %s", field)
		}
	}
	fusionValue, ok := payload["fusion"]
	if !ok {
		return nil
	}
	fusionMap, ok := fusionValue.(map[string]any)
	if !ok {
		return nil
	}
	for field := range fusionNestedRequestFields {
		if _, ok := fusionMap[field]; ok {
			return fmt.Errorf("fusion request cannot include fusion.%s", field)
		}
	}
	return nil
}

func validateFusionRawRelayRequest(c *gin.Context) bool {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		status := http.StatusBadRequest
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		fusionOpenAIError(c, status, err.Error(), types.ErrorCodeReadRequestBodyFailed)
		return false
	}
	rawBody, err := storage.Bytes()
	if err != nil {
		fusionOpenAIError(c, http.StatusBadRequest, err.Error(), types.ErrorCodeReadRequestBodyFailed)
		return false
	}
	if err := rejectFusionDirectCredentialFields(rawBody); err != nil {
		fusionOpenAIError(c, http.StatusBadRequest, err.Error(), types.ErrorCodeInvalidRequest)
		return false
	}
	return true
}

func validateFusionRelayChatRequest(request *dto.GeneralOpenAIRequest) error {
	if request.Model == "" {
		return errors.New("model is required")
	}
	if request.Stream != nil && *request.Stream {
		return errors.New("fusion does not support stream=true in v1")
	}
	if request.N != nil && *request.N > 1 {
		return errors.New("fusion does not support n>1 in v1")
	}
	if len(request.Tools) > 0 {
		return errors.New("fusion does not support tools in v1")
	}
	if request.ToolChoice != nil {
		return errors.New("fusion does not support tool_choice in v1")
	}
	if len(request.Functions) > 0 {
		return errors.New("fusion does not support functions in v1")
	}
	if len(request.FunctionCall) > 0 {
		return errors.New("fusion does not support function_call in v1")
	}
	if len(request.Messages) == 0 {
		return errors.New("messages are required")
	}
	return nil
}

func enforceFusionTokenModelLimit(c *gin.Context, modelName string) bool {
	if !common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled) {
		return true
	}
	value, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
	if !ok {
		fusionOpenAIError(c, http.StatusForbidden, "This token has no access to any model", types.ErrorCodeAccessDenied)
		return false
	}
	tokenModelLimit, ok := value.(map[string]bool)
	if !ok {
		tokenModelLimit = map[string]bool{}
	}
	matchName := ratio_setting.FormatMatchingModelName(modelName)
	if _, ok := tokenModelLimit[matchName]; !ok {
		fusionOpenAIError(c, http.StatusForbidden, fmt.Sprintf("This token has no access to model %s", modelName), types.ErrorCodeAccessDenied)
		return false
	}
	return true
}

func fusionGroupRatioInfo(c *gin.Context) types.GroupRatioInfo {
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if usingGroup == "" {
		usingGroup = userGroup
	}
	if usingGroup == "" {
		usingGroup = "default"
	}
	info := types.GroupRatioInfo{
		GroupRatio:        ratio_setting.GetGroupRatio(usingGroup),
		GroupSpecialRatio: -1,
	}
	if ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, usingGroup); ok {
		info.GroupRatio = ratio
		info.GroupSpecialRatio = ratio
		info.HasSpecialRatio = true
	}
	return info
}

func fusionBillingPolicyFromContext(c *gin.Context) service.FusionBillingPolicy {
	return service.FusionBillingPolicy{
		Expression:             fusion_setting.GetFusionBillingExpr(),
		MinimumQuota:           fusion_setting.GetFusionMinimumQuota(),
		ChargeFailedCandidates: fusion_setting.ShouldFusionChargeFailedCandidates(),
		FailedCandidateQuota:   fusion_setting.GetFusionFailedCandidateQuota(),
		GroupRatio:             fusionGroupRatioInfo(c).GroupRatio,
	}
}

func estimateFusionRelayPromptTokens(request *dto.GeneralOpenAIRequest) int {
	meta := request.GetTokenCountMeta()
	tokens := service.CountTextToken(meta.CombineText, request.Model)
	tokens += meta.MessagesCount*3 + meta.NameCount*3 + meta.ToolsCount*8 + 3
	if tokens <= 0 {
		return 1
	}
	return tokens
}

func buildFusionPreConsumeInput(config *model.FusionConfig, request *dto.GeneralOpenAIRequest) (service.FusionBillingInput, error) {
	candidateIDs, err := config.GetCandidateKeyIDs()
	if err != nil {
		return service.FusionBillingInput{}, err
	}
	promptTokens := estimateFusionRelayPromptTokens(request)
	completionTokens := int(request.GetMaxTokens())
	if completionTokens <= 0 {
		completionTokens = common.PreConsumedQuota
	}
	if completionTokens <= 0 {
		completionTokens = 1
	}
	candidateCount := len(candidateIDs)
	return service.FusionBillingInput{
		CandidatePromptTokens:     promptTokens * candidateCount,
		CandidateCompletionTokens: completionTokens * candidateCount,
		JudgePromptTokens:         promptTokens + completionTokens*candidateCount,
		JudgeCompletionTokens:     completionTokens,
		SuccessfulCandidates:      candidateCount,
		TotalCandidates:           candidateCount,
	}, nil
}

func buildFusionRelayInfo(c *gin.Context, request *dto.GeneralOpenAIRequest) (*relaycommon.RelayInfo, error) {
	common.SetContextKey(c, constant.ContextKeyOriginalModel, request.Model)
	c.Set("original_model", request.Model)
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
	c.Set("relay_mode", relayconstant.RelayModeChatCompletions)
	return relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, request, nil)
}

func buildFusionChatCompletionResponse(modelAlias string, result *service.FusionEngineResult) dto.OpenAITextResponse {
	finishReason := result.Judge.FinishReason
	if finishReason == "" {
		finishReason = "stop"
	}
	return dto.OpenAITextResponse{
		Id:      "chatcmpl-fusion-" + common.GetRandomString(12),
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Model:   modelAlias,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: result.Content,
				},
				FinishReason: finishReason,
			},
		},
		Usage: result.Usage,
	}
}

func buildFusionLogOther(relayInfo *relaycommon.RelayInfo, result *service.FusionEngineResult, preConsumedQuota int, actualQuota int, policy service.FusionBillingPolicy) map[string]interface{} {
	other := map[string]interface{}{
		"fusion":                   true,
		"pre_consumed_quota":       preConsumedQuota,
		"actual_quota":             actualQuota,
		"group_ratio":              policy.GroupRatio,
		"charge_failed_candidates": policy.ChargeFailedCandidates,
		"failed_candidate_quota":   policy.FailedCandidateQuota,
	}
	if relayInfo != nil {
		other["request_path"] = relayInfo.RequestURLPath
		if relayInfo.BillingSource != "" {
			other["billing_source"] = relayInfo.BillingSource
		}
		if relayInfo.UserSetting.BillingPreference != "" {
			other["billing_preference"] = relayInfo.UserSetting.BillingPreference
		}
		if relayInfo.BillingSource == service.BillingSourceSubscription {
			if relayInfo.SubscriptionId != 0 {
				other["subscription_id"] = relayInfo.SubscriptionId
			}
			if relayInfo.SubscriptionPreConsumed > 0 {
				other["subscription_pre_consumed"] = relayInfo.SubscriptionPreConsumed
			}
			if relayInfo.SubscriptionPostDelta != 0 {
				other["subscription_post_delta"] = relayInfo.SubscriptionPostDelta
			}
		}
	}
	if result == nil {
		return other
	}
	candidates := make([]map[string]interface{}, 0, len(result.Candidates))
	successCount := 0
	for _, candidate := range result.Candidates {
		if candidate.Success {
			successCount++
		}
		candidates = append(candidates, map[string]interface{}{
			"key_id":            candidate.KeyID,
			"model":             candidate.Model,
			"success":           candidate.Success,
			"prompt_tokens":     candidate.Usage.PromptTokens,
			"completion_tokens": candidate.Usage.CompletionTokens,
			"latency_ms":        candidate.LatencyMS,
			"status":            candidate.UpstreamStatus,
			"error":             candidate.SanitizedError,
		})
	}
	other["candidates"] = candidates
	other["candidate_count"] = len(result.Candidates)
	other["candidate_success_count"] = successCount
	other["judge"] = map[string]interface{}{
		"key_id":            result.Judge.KeyID,
		"model":             result.Judge.Model,
		"success":           result.Judge.Success,
		"prompt_tokens":     result.Judge.Usage.PromptTokens,
		"completion_tokens": result.Judge.Usage.CompletionTokens,
		"latency_ms":        result.Judge.LatencyMS,
		"status":            result.Judge.UpstreamStatus,
	}
	input := service.BuildFusionBillingInput(result)
	other["candidate_prompt_tokens"] = input.CandidatePromptTokens
	other["candidate_completion_tokens"] = input.CandidateCompletionTokens
	other["judge_prompt_tokens"] = input.JudgePromptTokens
	other["judge_completion_tokens"] = input.JudgeCompletionTokens
	other["failed_candidates"] = input.FailedCandidates
	other["failed_prompt_tokens"] = input.FailedPromptTokens
	return other
}

func recordFusionConsumeLog(c *gin.Context, relayInfo *relaycommon.RelayInfo, result *service.FusionEngineResult, preConsumedQuota int, actualQuota int, policy service.FusionBillingPolicy) {
	useTimeSeconds := int(time.Since(relayInfo.StartTime).Seconds())
	model.RecordConsumeLog(c, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        0,
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
		ModelName:        relayInfo.OriginModelName,
		TokenName:        c.GetString("token_name"),
		Quota:            actualQuota,
		Content:          fmt.Sprintf("Fusion synthesis, candidates %d, judge model %s", len(result.Candidates), result.Judge.Model),
		TokenId:          relayInfo.TokenId,
		UseTimeSeconds:   useTimeSeconds,
		IsStream:         false,
		Group:            relayInfo.UsingGroup,
		Other:            buildFusionLogOther(relayInfo, result, preConsumedQuota, actualQuota, policy),
	})
}

func FusionChatCompletions(c *gin.Context) {
	if !fusion_setting.IsFusionEnabled() {
		fusionOpenAIError(c, http.StatusForbidden, "Fusion is disabled", types.ErrorCodeAccessDenied)
		return
	}
	if !common.HasPersistentCryptoSecret() {
		fusionOpenAIError(c, http.StatusServiceUnavailable, "CRYPTO_SECRET is required for Fusion", types.ErrorCodeInvalidRequest)
		return
	}
	if !validateFusionRawRelayRequest(c) {
		return
	}

	var request dto.GeneralOpenAIRequest
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		status := http.StatusBadRequest
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		fusionOpenAIError(c, status, err.Error(), types.ErrorCodeInvalidRequest)
		return
	}
	request.Model = strings.TrimSpace(request.Model)
	if err := validateFusionRelayChatRequest(&request); err != nil {
		fusionOpenAIError(c, http.StatusBadRequest, err.Error(), types.ErrorCodeInvalidRequest)
		return
	}
	if !enforceFusionTokenModelLimit(c, request.Model) {
		return
	}

	config, err := model.GetFusionConfigByUserAndAlias(c.GetInt("id"), request.Model)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fusionOpenAIError(c, http.StatusNotFound, fmt.Sprintf("fusion model %s not found", request.Model), types.ErrorCodeModelNotFound)
			return
		}
		fusionOpenAIError(c, http.StatusInternalServerError, err.Error(), types.ErrorCodeQueryDataError)
		return
	}
	if !config.Enabled {
		fusionOpenAIError(c, http.StatusForbidden, "fusion config is disabled", types.ErrorCodeAccessDenied)
		return
	}

	relayInfo, err := buildFusionRelayInfo(c, &request)
	if err != nil {
		fusionOpenAIError(c, http.StatusInternalServerError, err.Error(), types.ErrorCodeGenRelayInfoFailed)
		return
	}
	groupRatioInfo := fusionGroupRatioInfo(c)
	relayInfo.PriceData = types.PriceData{
		GroupRatioInfo: groupRatioInfo,
	}
	policy := fusionBillingPolicyFromContext(c)
	preConsumeInput, err := buildFusionPreConsumeInput(config, &request)
	if err != nil {
		fusionOpenAIError(c, http.StatusBadRequest, err.Error(), types.ErrorCodeInvalidRequest)
		return
	}
	preConsumedQuota, err := service.CalculateFusionServiceQuota(preConsumeInput, policy)
	if err != nil {
		fusionOpenAIError(c, http.StatusServiceUnavailable, err.Error(), types.ErrorCodeModelPriceError)
		return
	}
	relayInfo.PriceData.QuotaToPreConsume = preConsumedQuota
	if newAPIError := service.PreConsumeBilling(c, preConsumedQuota, relayInfo); newAPIError != nil {
		fusionNewAPIError(c, newAPIError)
		return
	}

	result, err := service.RunFusionEngine(c.Request.Context(), service.FusionEngineRequest{
		UserID:    relayInfo.UserId,
		TokenID:   relayInfo.TokenId,
		TokenName: c.GetString("token_name"),
		Config:    config,
		Request:   &request,
	})
	if err != nil {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		fusionOpenAIError(c, http.StatusBadGateway, err.Error(), types.ErrorCodeBadResponse)
		return
	}

	actualQuota, err := service.CalculateFusionServiceQuota(service.BuildFusionBillingInput(result), policy)
	if err != nil {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		fusionOpenAIError(c, http.StatusServiceUnavailable, err.Error(), types.ErrorCodeModelPriceError)
		return
	}
	relayInfo.SetFirstResponseTime()
	model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, actualQuota)
	if err := service.SettleBilling(c, relayInfo, actualQuota); err != nil {
		common.SysError("error settling fusion billing: " + err.Error())
	}
	recordFusionConsumeLog(c, relayInfo, result, preConsumedQuota, actualQuota, policy)
	c.JSON(http.StatusOK, buildFusionChatCompletionResponse(request.Model, result))
}

func validateFusionKeyStatus(status int) error {
	if status == model.FusionKeyStatusEnabled || status == model.FusionKeyStatusDisabled {
		return nil
	}
	return fmt.Errorf("invalid fusion key status: %d", status)
}

func buildFusionAPIKeyResponse(key *model.FusionAPIKey) (dto.FusionAPIKeyResponse, error) {
	models, err := key.GetModels()
	if err != nil {
		return dto.FusionAPIKeyResponse{}, err
	}
	return dto.FusionAPIKeyResponse{
		Id:           key.Id,
		Name:         key.Name,
		Provider:     key.Provider,
		BaseURL:      key.BaseURL,
		DefaultModel: key.DefaultModel,
		Models:       models,
		APIKeyHint:   key.APIKeyHint,
		Status:       key.Status,
		LastTestTime: key.LastTestTime,
		LastError:    key.LastError,
		CreatedAt:    key.CreatedAt,
		UpdatedAt:    key.UpdatedAt,
	}, nil
}

func buildFusionAPIKeyResponses(keys []*model.FusionAPIKey) ([]dto.FusionAPIKeyResponse, error) {
	responses := make([]dto.FusionAPIKeyResponse, 0, len(keys))
	for _, key := range keys {
		response, err := buildFusionAPIKeyResponse(key)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func validateFusionAPIKeyFields(name, provider, baseURL, defaultModel string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	if len(strings.TrimSpace(name)) > fusionNameMaxLength {
		return fmt.Errorf("name must be at most %d characters", fusionNameMaxLength)
	}
	if provider != "" && provider != model.FusionProviderOpenAICompatible {
		return fmt.Errorf("unsupported fusion provider: %s", provider)
	}
	if strings.TrimSpace(baseURL) == "" {
		return errors.New("base_url is required")
	}
	if strings.TrimSpace(defaultModel) == "" {
		return errors.New("default_model is required")
	}
	if len(strings.TrimSpace(defaultModel)) > fusionDefaultModelMaxBytes {
		return fmt.Errorf("default_model must be at most %d characters", fusionDefaultModelMaxBytes)
	}
	return nil
}

func prepareFusionAPIKeyForCreate(userId int, req dto.FusionAPIKeyCreateRequest) (*model.FusionAPIKey, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Provider = strings.TrimSpace(req.Provider)
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.DefaultModel = strings.TrimSpace(req.DefaultModel)
	if err := validateFusionAPIKeyFields(req.Name, req.Provider, req.BaseURL, req.DefaultModel); err != nil {
		return nil, err
	}
	if !common.HasPersistentCryptoSecret() {
		return nil, errors.New("CRYPTO_SECRET is required for Fusion")
	}
	normalizedBaseURL, err := common.ValidateFusionBaseURL(req.BaseURL, fusionBaseURLPolicyFromSetting())
	if err != nil {
		return nil, err
	}
	key := &model.FusionAPIKey{
		UserId:       userId,
		Name:         req.Name,
		Provider:     req.Provider,
		BaseURL:      normalizedBaseURL,
		DefaultModel: req.DefaultModel,
		Status:       model.FusionKeyStatusEnabled,
	}
	key.Normalize()
	if err := key.SetModels(req.Models); err != nil {
		return nil, err
	}
	allowed, err := key.IsModelAllowed(key.DefaultModel)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("default_model %s is not allowed by models", key.DefaultModel)
	}
	if err := key.SetPlainAPIKey(req.APIKey); err != nil {
		return nil, err
	}
	return key, nil
}

func prepareFusionAPIKeyForUpdate(existing *model.FusionAPIKey, req dto.FusionAPIKeyUpdateRequest) (*model.FusionAPIKey, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Provider = strings.TrimSpace(req.Provider)
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.DefaultModel = strings.TrimSpace(req.DefaultModel)
	if err := validateFusionAPIKeyFields(req.Name, req.Provider, req.BaseURL, req.DefaultModel); err != nil {
		return nil, err
	}
	status := req.Status
	if status == 0 {
		status = existing.Status
	}
	if err := validateFusionKeyStatus(status); err != nil {
		return nil, err
	}
	normalizedBaseURL, err := common.ValidateFusionBaseURL(req.BaseURL, fusionBaseURLPolicyFromSetting())
	if err != nil {
		return nil, err
	}
	key := &model.FusionAPIKey{
		Id:               existing.Id,
		UserId:           existing.UserId,
		Name:             req.Name,
		Provider:         req.Provider,
		BaseURL:          normalizedBaseURL,
		DefaultModel:     req.DefaultModel,
		APIKeyCiphertext: existing.APIKeyCiphertext,
		APIKeyHint:       existing.APIKeyHint,
		KeyFingerprint:   existing.KeyFingerprint,
		Status:           status,
		LastError:        existing.LastError,
		LastTestTime:     existing.LastTestTime,
	}
	key.Normalize()
	if err := key.SetModels(req.Models); err != nil {
		return nil, err
	}
	allowed, err := key.IsModelAllowed(key.DefaultModel)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("default_model %s is not allowed by models", key.DefaultModel)
	}
	if strings.TrimSpace(req.APIKey) != "" {
		if !common.HasPersistentCryptoSecret() {
			return nil, errors.New("CRYPTO_SECRET is required for Fusion")
		}
		if err := key.SetPlainAPIKey(req.APIKey); err != nil {
			return nil, err
		}
	}
	return key, nil
}

func GetFusionAPIKeys(c *gin.Context) {
	userId := c.GetInt("id")
	keys, err := model.GetFusionAPIKeysByUserId(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	responses, err := buildFusionAPIKeyResponses(keys)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items": responses,
		"total": len(responses),
	})
}

func CreateFusionAPIKey(c *gin.Context) {
	userId := c.GetInt("id")
	count, err := model.CountFusionAPIKeysByUserId(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	maxKeys := fusion_setting.GetFusionMaxKeysPerUser()
	if maxKeys > 0 && int(count) >= maxKeys {
		common.ApiError(c, fmt.Errorf("fusion key limit reached: %d", maxKeys))
		return
	}
	var req dto.FusionAPIKeyCreateRequest
	if !bindFusionJSON(c, &req) {
		return
	}
	key, err := prepareFusionAPIKeyForCreate(userId, req)
	if err != nil {
		if strings.Contains(err.Error(), "CRYPTO_SECRET") {
			fusionError(c, http.StatusServiceUnavailable, err)
			return
		}
		common.ApiError(c, err)
		return
	}
	duplicated, err := model.IsFusionAPIKeyFingerprintDuplicated(userId, 0, key.KeyFingerprint)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if duplicated {
		common.ApiError(c, errors.New("fusion api key already exists"))
		return
	}
	if err := key.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	response, err := buildFusionAPIKeyResponse(key)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func UpdateFusionAPIKey(c *gin.Context) {
	userId := c.GetInt("id")
	id, ok := parseFusionID(c)
	if !ok {
		return
	}
	existing, err := model.GetFusionAPIKeyByUserAndId(userId, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req dto.FusionAPIKeyUpdateRequest
	if !bindFusionJSON(c, &req) {
		return
	}
	key, err := prepareFusionAPIKeyForUpdate(existing, req)
	if err != nil {
		if strings.Contains(err.Error(), "CRYPTO_SECRET") {
			fusionError(c, http.StatusServiceUnavailable, err)
			return
		}
		common.ApiError(c, err)
		return
	}
	duplicated, err := model.IsFusionAPIKeyFingerprintDuplicated(userId, id, key.KeyFingerprint)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if duplicated {
		common.ApiError(c, errors.New("fusion api key already exists"))
		return
	}
	if err := key.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	response, err := buildFusionAPIKeyResponse(key)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func DeleteFusionAPIKey(c *gin.Context) {
	userId := c.GetInt("id")
	id, ok := parseFusionID(c)
	if !ok {
		return
	}
	inUse, err := isFusionAPIKeyInUse(userId, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if inUse {
		common.ApiError(c, errors.New("fusion api key is used by a fusion config"))
		return
	}
	if err := model.DeleteFusionAPIKeyByUserAndId(userId, id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func isFusionAPIKeyInUse(userId int, keyId int) (bool, error) {
	configs, err := model.GetFusionConfigsByUserId(userId)
	if err != nil {
		return false, err
	}
	for _, config := range configs {
		if config.JudgeKeyID == keyId {
			return true, nil
		}
		candidateIDs, err := config.GetCandidateKeyIDs()
		if err != nil {
			return false, err
		}
		for _, candidateID := range candidateIDs {
			if candidateID == keyId {
				return true, nil
			}
		}
	}
	return false, nil
}

func TestFusionAPIKey(c *gin.Context) {
	if !validateFusionExecutionGate(c) {
		return
	}
	userId := c.GetInt("id")
	id, ok := parseFusionID(c)
	if !ok {
		return
	}
	key, err := model.GetFusionAPIKeyByUserAndId(userId, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if key.Status != model.FusionKeyStatusEnabled {
		common.ApiError(c, errors.New("fusion api key is disabled"))
		return
	}
	fusionError(c, http.StatusNotImplemented, errors.New("fusion key test execution is not implemented until billing and engine are available"))
}

func normalizeCandidateIDs(ids []int) ([]int, error) {
	normalized := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, errors.New("candidate_key_ids contains invalid id")
		}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

func validateCandidateModels(candidateIDs []int, models map[string]string) error {
	if len(models) == 0 {
		return nil
	}
	candidateSet := make(map[int]struct{}, len(candidateIDs))
	for _, id := range candidateIDs {
		candidateSet[id] = struct{}{}
	}
	for keyID := range models {
		id, err := strconv.Atoi(strings.TrimSpace(keyID))
		if err != nil || id <= 0 {
			return fmt.Errorf("candidate_models contains invalid key id: %s", keyID)
		}
		if _, ok := candidateSet[id]; !ok {
			return fmt.Errorf("candidate_models references non-candidate key id: %d", id)
		}
	}
	return nil
}

func prepareFusionConfig(userId int, req dto.FusionConfigCreateRequest) (*model.FusionConfig, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.ModelAlias = strings.TrimSpace(req.ModelAlias)
	req.JudgeModel = strings.TrimSpace(req.JudgeModel)
	req.Strategy = strings.TrimSpace(req.Strategy)
	if req.Strategy == "" {
		req.Strategy = model.FusionStrategySynthesize
	}
	if req.Name == "" {
		return nil, errors.New("name is required")
	}
	if len(req.Name) > fusionNameMaxLength {
		return nil, fmt.Errorf("name must be at most %d characters", fusionNameMaxLength)
	}
	if req.JudgeKeyID <= 0 {
		return nil, errors.New("judge_key_id is required")
	}
	if req.JudgeModel == "" {
		return nil, errors.New("judge_model is required")
	}
	candidateIDs, err := normalizeCandidateIDs(req.CandidateKeyIDs)
	if err != nil {
		return nil, err
	}
	if len(candidateIDs) == 0 {
		return nil, errors.New("at least one candidate key is required")
	}
	maxCandidates := fusion_setting.GetFusionMaxCandidatesPerConfig()
	if maxCandidates > 0 && len(candidateIDs) > maxCandidates {
		return nil, fmt.Errorf("candidate key count exceeds limit: %d", maxCandidates)
	}
	if err := validateCandidateModels(candidateIDs, req.CandidateModels); err != nil {
		return nil, err
	}
	timeoutMS := req.TimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = fusion_setting.GetFusionDefaultTimeoutMS()
	}
	maxTimeoutMS := fusion_setting.GetFusionMaxTimeoutMS()
	if maxTimeoutMS > 0 && timeoutMS > maxTimeoutMS {
		return nil, fmt.Errorf("timeout_ms exceeds limit: %d", maxTimeoutMS)
	}
	maxParallel := req.MaxParallel
	if maxParallel <= 0 {
		maxParallel = fusion_setting.GetFusionMaxParallel()
	}
	if maxParallel <= 0 {
		maxParallel = 1
	}
	settingMaxParallel := fusion_setting.GetFusionMaxParallel()
	if settingMaxParallel > 0 && maxParallel > settingMaxParallel {
		return nil, fmt.Errorf("max_parallel exceeds limit: %d", settingMaxParallel)
	}
	minSuccesses := req.MinSuccesses
	if minSuccesses <= 0 {
		minSuccesses = 1
	}
	config := &model.FusionConfig{
		UserId:       userId,
		Name:         req.Name,
		ModelAlias:   req.ModelAlias,
		Enabled:      req.Enabled,
		JudgeKeyID:   req.JudgeKeyID,
		JudgeModel:   req.JudgeModel,
		Strategy:     req.Strategy,
		TimeoutMS:    timeoutMS,
		MaxParallel:  maxParallel,
		MinSuccesses: minSuccesses,
		JudgePrompt:  strings.TrimSpace(req.JudgePrompt),
	}
	if err := config.SetCandidateKeyIDs(candidateIDs); err != nil {
		return nil, err
	}
	if err := config.SetCandidateModels(req.CandidateModels); err != nil {
		return nil, err
	}
	if err := model.ValidateFusionConfigKeyOwnership(userId, config); err != nil {
		return nil, err
	}
	return config, nil
}

func buildFusionConfigResponse(config *model.FusionConfig) (dto.FusionConfigResponse, error) {
	candidateIDs, err := config.GetCandidateKeyIDs()
	if err != nil {
		return dto.FusionConfigResponse{}, err
	}
	candidateModels, err := config.GetCandidateModels()
	if err != nil {
		return dto.FusionConfigResponse{}, err
	}
	return dto.FusionConfigResponse{
		Id:              config.Id,
		Name:            config.Name,
		ModelAlias:      config.ModelAlias,
		Enabled:         config.Enabled,
		CandidateKeyIDs: candidateIDs,
		CandidateModels: candidateModels,
		JudgeKeyID:      config.JudgeKeyID,
		JudgeModel:      config.JudgeModel,
		Strategy:        config.Strategy,
		TimeoutMS:       config.TimeoutMS,
		MaxParallel:     config.MaxParallel,
		MinSuccesses:    config.MinSuccesses,
		JudgePrompt:     config.JudgePrompt,
		CreatedAt:       config.CreatedAt,
		UpdatedAt:       config.UpdatedAt,
	}, nil
}

func buildFusionConfigResponses(configs []*model.FusionConfig) ([]dto.FusionConfigResponse, error) {
	responses := make([]dto.FusionConfigResponse, 0, len(configs))
	for _, config := range configs {
		response, err := buildFusionConfigResponse(config)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func GetFusionConfigs(c *gin.Context) {
	userId := c.GetInt("id")
	configs, err := model.GetFusionConfigsByUserId(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	responses, err := buildFusionConfigResponses(configs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items": responses,
		"total": len(responses),
	})
}

func CreateFusionConfig(c *gin.Context) {
	userId := c.GetInt("id")
	count, err := model.CountFusionConfigsByUserId(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	maxConfigs := fusion_setting.GetFusionMaxConfigsPerUser()
	if maxConfigs > 0 && int(count) >= maxConfigs {
		common.ApiError(c, fmt.Errorf("fusion config limit reached: %d", maxConfigs))
		return
	}
	var req dto.FusionConfigCreateRequest
	if !bindFusionJSON(c, &req) {
		return
	}
	config, err := prepareFusionConfig(userId, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	duplicated, err := model.IsFusionConfigAliasDuplicated(userId, 0, config.ModelAlias)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if duplicated {
		common.ApiError(c, errors.New("fusion model alias already exists"))
		return
	}
	if err := config.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	response, err := buildFusionConfigResponse(config)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func UpdateFusionConfig(c *gin.Context) {
	userId := c.GetInt("id")
	id, ok := parseFusionID(c)
	if !ok {
		return
	}
	existing, err := model.GetFusionConfigByUserAndId(userId, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req dto.FusionConfigUpdateRequest
	if !bindFusionJSON(c, &req) {
		return
	}
	config, err := prepareFusionConfig(userId, req.FusionConfigCreateRequest)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	config.Id = existing.Id
	duplicated, err := model.IsFusionConfigAliasDuplicated(userId, id, config.ModelAlias)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if duplicated {
		common.ApiError(c, errors.New("fusion model alias already exists"))
		return
	}
	if err := config.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	response, err := buildFusionConfigResponse(config)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func DeleteFusionConfig(c *gin.Context) {
	userId := c.GetInt("id")
	id, ok := parseFusionID(c)
	if !ok {
		return
	}
	if err := model.DeleteFusionConfigByUserAndId(userId, id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func TestFusionConfig(c *gin.Context) {
	if !validateFusionExecutionGate(c) {
		return
	}
	userId := c.GetInt("id")
	id, ok := parseFusionID(c)
	if !ok {
		return
	}
	config, err := model.GetFusionConfigByUserAndId(userId, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !config.Enabled {
		common.ApiError(c, errors.New("fusion config is disabled"))
		return
	}
	if err := model.ValidateFusionConfigKeyOwnership(userId, config); err != nil {
		common.ApiError(c, err)
		return
	}
	fusionError(c, http.StatusNotImplemented, errors.New("fusion config test execution is not implemented until billing and engine are available"))
}
