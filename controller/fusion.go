package controller

import (
	"context"
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
	"github.com/QuantumNous/new-api/relay/helper"
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

var fusionStreamPingInterval = 5 * time.Second

var errFusionResponseStateUnavailable = errors.New("Fusion previous_response_id state not found or expired")

type fusionResponsesPreparedRequest struct {
	ChatRequest      *dto.GeneralOpenAIRequest
	ParentResponseID string
	Messages         []dto.Message
}

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
	return validateFusionCryptoSecretGate(c)
}

func validateFusionCryptoSecretGate(c *gin.Context) bool {
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
	if request.N != nil && *request.N > 1 {
		return errors.New("fusion does not support n>1 in v1")
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
		fusionOpenAIError(c, http.StatusForbidden, "Fusion token group requires exactly one enabled Fusion model limit", types.ErrorCodeAccessDenied)
		return false
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
	normalizedLimits := make(map[string]bool, len(tokenModelLimit))
	for limit, allowed := range tokenModelLimit {
		if !allowed {
			continue
		}
		limit = strings.TrimSpace(limit)
		if limit == "" {
			continue
		}
		normalizedLimits[ratio_setting.FormatMatchingModelName(limit)] = true
	}
	if len(normalizedLimits) != 1 {
		fusionOpenAIError(c, http.StatusForbidden, "Fusion token group requires exactly one enabled Fusion model limit", types.ErrorCodeAccessDenied)
		return false
	}
	if _, ok := normalizedLimits[matchName]; !ok {
		fusionOpenAIError(c, http.StatusForbidden, fmt.Sprintf("This token has no access to model %s", modelName), types.ErrorCodeAccessDenied)
		return false
	}
	return true
}

func enforceFusionTokenGroup(c *gin.Context) bool {
	tokenGroup := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	if !fusion_setting.IsFusionTokenGroup(tokenGroup) {
		fusionOpenAIError(c, http.StatusForbidden, "Fusion models require a Fusion token group", types.ErrorCodeAccessDenied)
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
	candidates, err := config.GetCandidates()
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
	candidateCount := len(candidates)
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
	message := dto.Message{
		Role:    "assistant",
		Content: result.Content,
	}
	if len(result.ToolCalls) > 0 {
		message.Content = ""
		message.SetToolCalls(result.ToolCalls)
		finishReason = "tool_calls"
	}
	return dto.OpenAITextResponse{
		Id:      "chatcmpl-fusion-" + common.GetRandomString(12),
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Model:   modelAlias,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index:        0,
				Message:      message,
				FinishReason: finishReason,
			},
		},
		Usage: result.Usage,
	}
}

func buildFusionChatCompletionStreamResponse(modelAlias string, result *service.FusionEngineResult) dto.ChatCompletionsStreamResponse {
	finishReason := result.Judge.FinishReason
	if finishReason == "" {
		finishReason = "stop"
	}
	delta := dto.ChatCompletionsStreamResponseChoiceDelta{
		Role:    "assistant",
		Content: &result.Content,
	}
	if len(result.ToolCalls) > 0 {
		delta.Content = nil
		delta.ToolCalls = result.ToolCalls
		finishReason = "tool_calls"
		for index := range delta.ToolCalls {
			delta.ToolCalls[index].SetIndex(index)
		}
	}
	return dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-fusion-" + common.GetRandomString(12),
		Object:  "chat.completion.chunk",
		Created: common.GetTimestamp(),
		Model:   modelAlias,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: &finishReason,
			},
		},
		Usage: &result.Usage,
	}
}

func writeFusionChatCompletionStream(c *gin.Context, modelAlias string, result *service.FusionEngineResult) {
	helper.SetEventStreamHeaders(c)
	chunk := buildFusionChatCompletionStreamResponse(modelAlias, result)
	_ = helper.ObjectData(c, chunk)
	helper.Done(c)
}

func writeFusionChatCompletionDelta(c *gin.Context, streamID string, created int64, modelAlias string, delta string, includeRole bool) {
	if delta == "" {
		return
	}
	responseDelta := dto.ChatCompletionsStreamResponseChoiceDelta{}
	responseDelta.SetContentString(delta)
	if includeRole {
		responseDelta.Role = "assistant"
	}
	chunk := dto.ChatCompletionsStreamResponse{
		Id:      streamID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   modelAlias,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: responseDelta,
			},
		},
	}
	_ = helper.ObjectData(c, chunk)
}

func writeFusionChatCompletionStop(c *gin.Context, streamID string, created int64, modelAlias string, result *service.FusionEngineResult) {
	finishReason := "stop"
	if result != nil && result.Judge.FinishReason != "" {
		finishReason = result.Judge.FinishReason
	}
	chunk := dto.ChatCompletionsStreamResponse{
		Id:      streamID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   modelAlias,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index:        0,
				Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				FinishReason: &finishReason,
			},
		},
	}
	if result != nil {
		chunk.Usage = &result.Usage
	}
	_ = helper.ObjectData(c, chunk)
	helper.Done(c)
}

func newFusionResponseID() string {
	return "resp_fusion_" + common.GetRandomString(12)
}

func fusionResponsesRawString(value string) []byte {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	data, _ := common.Marshal(value)
	return data
}

func buildFusionResponsesResponse(modelAlias string, responseID string, parentResponseID string, result *service.FusionEngineResult) dto.OpenAIResponsesResponse {
	if strings.TrimSpace(responseID) == "" {
		responseID = newFusionResponseID()
	}
	status, _ := common.Marshal("completed")
	output := []dto.ResponsesOutput{
		{
			Type:   "message",
			ID:     "msg_fusion_" + common.GetRandomString(12),
			Status: "completed",
			Role:   "assistant",
			Content: []dto.ResponsesOutputContent{
				{
					Type:        "output_text",
					Text:        result.Content,
					Annotations: []interface{}{},
				},
			},
		},
	}
	if len(result.ToolCalls) > 0 {
		output = fusionResponsesToolCallOutputs(result.ToolCalls)
	}
	return dto.OpenAIResponsesResponse{
		ID:                 responseID,
		Object:             "response",
		CreatedAt:          int(common.GetTimestamp()),
		Status:             status,
		Model:              modelAlias,
		Output:             output,
		ParallelToolCalls:  true,
		PreviousResponseID: fusionResponsesRawString(parentResponseID),
		Store:              false,
		Usage: &dto.Usage{
			InputTokens:        result.Usage.PromptTokens,
			OutputTokens:       result.Usage.CompletionTokens,
			TotalTokens:        result.Usage.TotalTokens,
			InputTokensDetails: &result.Usage.PromptTokensDetails,
		},
	}
}

func fusionResponsesToolCallOutputs(toolCalls []dto.ToolCallResponse) []dto.ResponsesOutput {
	output := make([]dto.ResponsesOutput, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		if strings.TrimSpace(toolCall.ID) == "" || strings.TrimSpace(toolCall.Function.Name) == "" {
			continue
		}
		arguments, _ := common.Marshal(toolCall.Function.Arguments)
		output = append(output, dto.ResponsesOutput{
			Type:      "function_call",
			ID:        toolCall.ID,
			Status:    "completed",
			CallId:    toolCall.ID,
			Name:      toolCall.Function.Name,
			Arguments: arguments,
		})
	}
	return output
}

func fusionResponsesCallIDVariants(id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if strings.HasPrefix(id, "call_") {
		return []string{"fc_" + strings.TrimPrefix(id, "call_")}
	}
	if strings.HasPrefix(id, "fc_") {
		return []string{"call_" + strings.TrimPrefix(id, "fc_")}
	}
	return nil
}

func addFusionResponseCallIDMapping(mapping map[string]string, itemID string, callID string) {
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return
	}
	mapping[callID] = callID
	for _, variant := range fusionResponsesCallIDVariants(callID) {
		mapping[variant] = callID
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return
	}
	mapping[itemID] = callID
	for _, variant := range fusionResponsesCallIDVariants(itemID) {
		mapping[variant] = callID
	}
}

func fusionResponsesCallIDMappings(output []dto.ResponsesOutput) map[string]string {
	mapping := make(map[string]string)
	for _, item := range output {
		if item.Type != "function_call" {
			continue
		}
		callID := strings.TrimSpace(item.CallId)
		if callID == "" {
			callID = strings.TrimSpace(item.ID)
		}
		addFusionResponseCallIDMapping(mapping, item.ID, callID)
	}
	return mapping
}

func fusionResponsesAssistantMessageFromOutput(output []dto.ResponsesOutput) (dto.Message, bool) {
	toolCalls := make([]dto.ToolCallResponse, 0)
	texts := make([]string, 0)
	for _, item := range output {
		switch item.Type {
		case "function_call":
			callID := strings.TrimSpace(item.CallId)
			if callID == "" {
				callID = strings.TrimSpace(item.ID)
			}
			if callID == "" || strings.TrimSpace(item.Name) == "" {
				continue
			}
			toolCalls = append(toolCalls, dto.ToolCallResponse{
				ID:   callID,
				Type: "function",
				Function: dto.FunctionResponse{
					Name:      item.Name,
					Arguments: item.ArgumentsString(),
				},
			})
		case "message":
			for _, content := range item.Content {
				if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
					texts = append(texts, content.Text)
				}
			}
		}
	}
	if len(toolCalls) > 0 {
		message := dto.Message{
			Role:    "assistant",
			Content: "",
		}
		message.SetToolCalls(toolCalls)
		return message, true
	}
	if len(texts) > 0 {
		return dto.Message{
			Role:    "assistant",
			Content: strings.Join(texts, "\n"),
		}, true
	}
	return dto.Message{}, false
}

func saveFusionResponseState(c *gin.Context, modelAlias string, responseID string, parentResponseID string, messages []dto.Message, output []dto.ResponsesOutput) error {
	stateMessages := append([]dto.Message{}, messages...)
	if assistantMessage, ok := fusionResponsesAssistantMessageFromOutput(output); ok {
		stateMessages = append(stateMessages, assistantMessage)
	}
	payload := model.FusionResponseStatePayload{
		Messages:       stateMessages,
		Outputs:        append([]dto.ResponsesOutput{}, output...),
		CallIDByItemID: fusionResponsesCallIDMappings(output),
	}
	payloadData, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	maxPayloadBytes := fusion_setting.GetFusionResponseStateMaxPayloadBytes()
	if maxPayloadBytes > 0 && len(payloadData) > maxPayloadBytes {
		return fmt.Errorf("fusion response state payload exceeds limit: %d > %d bytes", len(payloadData), maxPayloadBytes)
	}
	ttlSeconds := fusion_setting.GetFusionResponseStateTTLSeconds()
	if ttlSeconds <= 0 {
		ttlSeconds = 86400
	}
	state := &model.FusionResponseState{
		ResponseID:       responseID,
		UserID:           c.GetInt("id"),
		TokenID:          c.GetInt("token_id"),
		ModelAlias:       modelAlias,
		ParentResponseID: parentResponseID,
		ExpiresAt:        time.Now().Add(time.Duration(ttlSeconds) * time.Second).Unix(),
	}
	if err := state.SetStatePayload(payload); err != nil {
		return err
	}
	return state.Insert()
}

func fusionResponseStateNewAPIError(err error) *types.NewAPIError {
	return types.NewErrorWithStatusCode(err, types.ErrorCodeBadResponse, http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
}

func writeFusionResponsesStream(c *gin.Context, modelAlias string, result *service.FusionEngineResult, stateContext fusionResponsesPreparedRequest) bool {
	helper.SetEventStreamHeaders(c)
	sequenceNumber := 0
	writeEvent := func(eventType string, event dto.ResponsesStreamResponse) {
		sequenceNumber++
		event.SequenceNumber = &sequenceNumber
		writeFusionResponsesStreamEvent(c, eventType, event)
	}
	responseID := newFusionResponseID()
	response := buildFusionResponsesResponse(modelAlias, responseID, stateContext.ParentResponseID, result)
	writeEvent("response.created", dto.ResponsesStreamResponse{Response: &response})
	writeEvent("response.in_progress", dto.ResponsesStreamResponse{Response: &response})
	if len(result.ToolCalls) > 0 {
		for index := range response.Output {
			outputIndex := index
			item := response.Output[index]
			writeEvent(dto.ResponsesOutputTypeItemAdded, dto.ResponsesStreamResponse{
				Item:        &item,
				OutputIndex: &outputIndex,
			})
			if arguments := item.ArgumentsString(); arguments != "" {
				writeEvent("response.function_call_arguments.delta", dto.ResponsesStreamResponse{
					Delta:       arguments,
					OutputIndex: &outputIndex,
					ItemID:      item.ID,
				})
			}
			writeEvent("response.function_call_arguments.done", dto.ResponsesStreamResponse{
				Arguments:   item.ArgumentsString(),
				OutputIndex: &outputIndex,
				ItemID:      item.ID,
			})
			writeEvent(dto.ResponsesOutputTypeItemDone, dto.ResponsesStreamResponse{
				Item:        &item,
				OutputIndex: &outputIndex,
			})
		}
		if err := saveFusionResponseState(c, modelAlias, response.ID, stateContext.ParentResponseID, stateContext.Messages, response.Output); err != nil {
			writeFusionResponsesStreamError(c, modelAlias, fusionResponseStateNewAPIError(err))
			return false
		}
		writeEvent("response.completed", dto.ResponsesStreamResponse{Response: &response})
		return true
	}
	outputIndex := 0
	contentIndex := 0
	if len(response.Output) == 0 {
		if err := saveFusionResponseState(c, modelAlias, response.ID, stateContext.ParentResponseID, stateContext.Messages, response.Output); err != nil {
			writeFusionResponsesStreamError(c, modelAlias, fusionResponseStateNewAPIError(err))
			return false
		}
		writeEvent("response.completed", dto.ResponsesStreamResponse{Response: &response})
		return true
	}
	item := response.Output[0]
	part := dto.ResponsesStreamPart{
		Type:        "output_text",
		Text:        result.Content,
		Annotations: []interface{}{},
	}
	writeEvent(dto.ResponsesOutputTypeItemAdded, dto.ResponsesStreamResponse{
		Item:        &item,
		OutputIndex: &outputIndex,
	})
	writeEvent("response.content_part.added", dto.ResponsesStreamResponse{
		OutputIndex:  &outputIndex,
		ContentIndex: &contentIndex,
		ItemID:       item.ID,
		Part:         &part,
	})
	writeEvent("response.output_text.delta", dto.ResponsesStreamResponse{
		Delta:        result.Content,
		OutputIndex:  &outputIndex,
		ContentIndex: &contentIndex,
		ItemID:       item.ID,
	})
	writeEvent("response.output_text.done", dto.ResponsesStreamResponse{
		Text:         result.Content,
		OutputIndex:  &outputIndex,
		ContentIndex: &contentIndex,
		ItemID:       item.ID,
	})
	writeEvent("response.content_part.done", dto.ResponsesStreamResponse{
		OutputIndex:  &outputIndex,
		ContentIndex: &contentIndex,
		ItemID:       item.ID,
		Part:         &part,
	})
	writeEvent(dto.ResponsesOutputTypeItemDone, dto.ResponsesStreamResponse{
		Item:        &item,
		OutputIndex: &outputIndex,
	})
	if err := saveFusionResponseState(c, modelAlias, response.ID, stateContext.ParentResponseID, stateContext.Messages, response.Output); err != nil {
		writeFusionResponsesStreamError(c, modelAlias, fusionResponseStateNewAPIError(err))
		return false
	}
	writeEvent("response.completed", dto.ResponsesStreamResponse{Response: &response})
	return true
}

func writeFusionResponsesStreamEvent(c *gin.Context, eventType string, event dto.ResponsesStreamResponse) {
	event.Type = eventType
	if data, err := common.Marshal(event); err == nil {
		helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: eventType}, string(data))
	}
}

type fusionResponsesStreamState struct {
	response     dto.OpenAIResponsesResponse
	item         dto.ResponsesOutput
	part         dto.ResponsesStreamPart
	sequence     int
	outputIndex  int
	contentIndex int
	text         strings.Builder
}

func newFusionResponsesStreamState(modelAlias string, responseID string, parentResponseID string) *fusionResponsesStreamState {
	status, _ := common.Marshal("in_progress")
	if strings.TrimSpace(responseID) == "" {
		responseID = newFusionResponseID()
	}
	itemID := "msg_fusion_" + common.GetRandomString(12)
	return &fusionResponsesStreamState{
		response: dto.OpenAIResponsesResponse{
			ID:                 responseID,
			Object:             "response",
			CreatedAt:          int(common.GetTimestamp()),
			Status:             status,
			Model:              modelAlias,
			Output:             []dto.ResponsesOutput{},
			ParallelToolCalls:  true,
			PreviousResponseID: fusionResponsesRawString(parentResponseID),
			Store:              false,
		},
		item: dto.ResponsesOutput{
			Type:   "message",
			ID:     itemID,
			Status: "in_progress",
			Role:   "assistant",
		},
		part: dto.ResponsesStreamPart{
			Type:        "output_text",
			Text:        "",
			Annotations: []interface{}{},
		},
	}
}

func (state *fusionResponsesStreamState) writeEvent(c *gin.Context, eventType string, event dto.ResponsesStreamResponse) {
	state.sequence++
	event.SequenceNumber = &state.sequence
	writeFusionResponsesStreamEvent(c, eventType, event)
}

func (state *fusionResponsesStreamState) start(c *gin.Context) {
	state.writeEvent(c, "response.created", dto.ResponsesStreamResponse{Response: &state.response})
	state.writeEvent(c, "response.in_progress", dto.ResponsesStreamResponse{Response: &state.response})
	state.writeEvent(c, dto.ResponsesOutputTypeItemAdded, dto.ResponsesStreamResponse{
		Item:        &state.item,
		OutputIndex: &state.outputIndex,
	})
	state.writeEvent(c, "response.content_part.added", dto.ResponsesStreamResponse{
		OutputIndex:  &state.outputIndex,
		ContentIndex: &state.contentIndex,
		ItemID:       state.item.ID,
		Part:         &state.part,
	})
}

func (state *fusionResponsesStreamState) delta(c *gin.Context, delta string) {
	if delta == "" {
		return
	}
	state.text.WriteString(delta)
	state.writeEvent(c, "response.output_text.delta", dto.ResponsesStreamResponse{
		Delta:        delta,
		OutputIndex:  &state.outputIndex,
		ContentIndex: &state.contentIndex,
		ItemID:       state.item.ID,
	})
}

func (state *fusionResponsesStreamState) complete(c *gin.Context, result *service.FusionEngineResult, stateContext fusionResponsesPreparedRequest) bool {
	text := state.text.String()
	if result != nil && text == "" {
		text = result.Content
	}
	state.part.Text = text
	state.item.Status = "completed"
	state.item.Content = []dto.ResponsesOutputContent{
		{
			Type:        "output_text",
			Text:        text,
			Annotations: []interface{}{},
		},
	}
	state.writeEvent(c, "response.output_text.done", dto.ResponsesStreamResponse{
		Text:         text,
		OutputIndex:  &state.outputIndex,
		ContentIndex: &state.contentIndex,
		ItemID:       state.item.ID,
	})
	state.writeEvent(c, "response.content_part.done", dto.ResponsesStreamResponse{
		OutputIndex:  &state.outputIndex,
		ContentIndex: &state.contentIndex,
		ItemID:       state.item.ID,
		Part:         &state.part,
	})
	state.writeEvent(c, dto.ResponsesOutputTypeItemDone, dto.ResponsesStreamResponse{
		Item:        &state.item,
		OutputIndex: &state.outputIndex,
	})
	response := state.response
	completedStatus, _ := common.Marshal("completed")
	response.Status = completedStatus
	response.Output = []dto.ResponsesOutput{state.item}
	if result != nil {
		response.Usage = &dto.Usage{
			InputTokens:        result.Usage.PromptTokens,
			OutputTokens:       result.Usage.CompletionTokens,
			TotalTokens:        result.Usage.TotalTokens,
			InputTokensDetails: &result.Usage.PromptTokensDetails,
		}
	}
	if err := saveFusionResponseState(c, response.Model, response.ID, stateContext.ParentResponseID, stateContext.Messages, response.Output); err != nil {
		writeFusionResponsesStreamError(c, response.Model, fusionResponseStateNewAPIError(err))
		return false
	}
	state.writeEvent(c, "response.completed", dto.ResponsesStreamResponse{Response: &response})
	return true
}

type fusionRelayExecution struct {
	RelayInfo        *relaycommon.RelayInfo
	Result           *service.FusionEngineResult
	PreConsumedQuota int
	ActualQuota      int
	Policy           service.FusionBillingPolicy
	IsStream         bool
}

type fusionRelayPreparedExecution struct {
	RelayInfo        *relaycommon.RelayInfo
	Config           *model.FusionConfig
	Request          dto.GeneralOpenAIRequest
	PreConsumedQuota int
	Policy           service.FusionBillingPolicy
	IsStream         bool
}

func prepareFusionRelay(c *gin.Context, request *dto.GeneralOpenAIRequest) (*fusionRelayPreparedExecution, bool) {
	if !enforceFusionTokenGroup(c) {
		return nil, false
	}
	if !enforceFusionTokenModelLimit(c, request.Model) {
		return nil, false
	}

	config, err := model.GetFusionConfigByUserAndAlias(c.GetInt("id"), request.Model)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fusionOpenAIError(c, http.StatusNotFound, fmt.Sprintf("fusion model %s not found", request.Model), types.ErrorCodeModelNotFound)
			return nil, false
		}
		fusionOpenAIError(c, http.StatusInternalServerError, err.Error(), types.ErrorCodeQueryDataError)
		return nil, false
	}
	if !config.Enabled {
		fusionOpenAIError(c, http.StatusForbidden, "fusion config is disabled", types.ErrorCodeAccessDenied)
		return nil, false
	}

	relayInfo, err := buildFusionRelayInfo(c, request)
	if err != nil {
		fusionOpenAIError(c, http.StatusInternalServerError, err.Error(), types.ErrorCodeGenRelayInfoFailed)
		return nil, false
	}
	groupRatioInfo := fusionGroupRatioInfo(c)
	relayInfo.PriceData = types.PriceData{
		GroupRatioInfo: groupRatioInfo,
	}
	policy := fusionBillingPolicyFromContext(c)
	preConsumeInput, err := buildFusionPreConsumeInput(config, request)
	if err != nil {
		fusionOpenAIError(c, http.StatusBadRequest, err.Error(), types.ErrorCodeInvalidRequest)
		return nil, false
	}
	preConsumedQuota, err := service.CalculateFusionServiceQuota(preConsumeInput, policy)
	if err != nil {
		fusionOpenAIError(c, http.StatusServiceUnavailable, err.Error(), types.ErrorCodeModelPriceError)
		return nil, false
	}
	relayInfo.PriceData.QuotaToPreConsume = preConsumedQuota
	if newAPIError := service.PreConsumeBilling(c, preConsumedQuota, relayInfo); newAPIError != nil {
		fusionNewAPIError(c, newAPIError)
		return nil, false
	}

	executionRequest := *request
	executionRequest.Stream = common.GetPointer(request.Stream != nil && *request.Stream)
	return &fusionRelayPreparedExecution{
		RelayInfo:        relayInfo,
		Config:           config,
		Request:          executionRequest,
		PreConsumedQuota: preConsumedQuota,
		Policy:           policy,
		IsStream:         request.Stream != nil && *request.Stream,
	}, true
}

func executePreparedFusionRelay(c *gin.Context, prepared *fusionRelayPreparedExecution) (*fusionRelayExecution, error) {
	if prepared == nil {
		return nil, errors.New("fusion execution is not prepared")
	}
	relayInfo := prepared.RelayInfo
	result, err := service.RunFusionEngine(c.Request.Context(), service.FusionEngineRequest{
		UserID:    relayInfo.UserId,
		TokenID:   relayInfo.TokenId,
		TokenName: c.GetString("token_name"),
		Config:    prepared.Config,
		Request:   &prepared.Request,
	})
	if err != nil {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		return nil, fusionExecutionNewAPIError(err)
	}

	actualQuota, err := service.CalculateFusionServiceQuota(service.BuildFusionBillingInput(result), prepared.Policy)
	if err != nil {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeModelPriceError, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	relayInfo.SetFirstResponseTime()
	model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, actualQuota)
	if err := service.SettleBilling(c, relayInfo, actualQuota); err != nil {
		common.SysError("error settling fusion billing: " + err.Error())
	}
	recordFusionConsumeLog(c, relayInfo, result, prepared.PreConsumedQuota, actualQuota, prepared.Policy, prepared.IsStream)
	return &fusionRelayExecution{
		RelayInfo:        relayInfo,
		Result:           result,
		PreConsumedQuota: prepared.PreConsumedQuota,
		ActualQuota:      actualQuota,
		Policy:           prepared.Policy,
		IsStream:         prepared.IsStream,
	}, nil
}

func executePreparedFusionRelayStream(c *gin.Context, prepared *fusionRelayPreparedExecution, callbacks service.FusionStreamCallbacks) (*fusionRelayExecution, error) {
	if prepared == nil {
		return nil, errors.New("fusion execution is not prepared")
	}
	relayInfo := prepared.RelayInfo
	result, err := service.RunFusionEngineStreamFinal(c.Request.Context(), service.FusionEngineRequest{
		UserID:    relayInfo.UserId,
		TokenID:   relayInfo.TokenId,
		TokenName: c.GetString("token_name"),
		Config:    prepared.Config,
		Request:   &prepared.Request,
	}, callbacks)
	if err != nil {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		return nil, fusionExecutionNewAPIError(err)
	}

	actualQuota, err := service.CalculateFusionServiceQuota(service.BuildFusionBillingInput(result), prepared.Policy)
	if err != nil {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeModelPriceError, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	relayInfo.SetFirstResponseTime()
	model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, actualQuota)
	if err := service.SettleBilling(c, relayInfo, actualQuota); err != nil {
		common.SysError("error settling fusion billing: " + err.Error())
	}
	recordFusionConsumeLog(c, relayInfo, result, prepared.PreConsumedQuota, actualQuota, prepared.Policy, prepared.IsStream)
	return &fusionRelayExecution{
		RelayInfo:        relayInfo,
		Result:           result,
		PreConsumedQuota: prepared.PreConsumedQuota,
		ActualQuota:      actualQuota,
		Policy:           prepared.Policy,
		IsStream:         prepared.IsStream,
	}, nil
}

func runFusionRelay(c *gin.Context, request *dto.GeneralOpenAIRequest) (*fusionRelayExecution, bool) {
	prepared, ok := prepareFusionRelay(c, request)
	if !ok {
		return nil, false
	}
	execution, err := executePreparedFusionRelay(c, prepared)
	if err != nil {
		if newAPIError, ok := err.(*types.NewAPIError); ok {
			fusionNewAPIError(c, newAPIError)
		} else {
			fusionNewAPIError(c, fusionExecutionNewAPIError(err))
		}
		return nil, false
	}
	return execution, true
}

func fusionExecutionNewAPIError(err error) *types.NewAPIError {
	code := types.ErrorCodeBadResponse
	status := http.StatusBadGateway
	if isFusionExecutionTimeoutError(err) {
		code = types.ErrorCodeChannelResponseTimeExceeded
		status = http.StatusGatewayTimeout
	}
	return types.NewErrorWithStatusCode(err, code, status, types.ErrOptionWithSkipRetry())
}

func isFusionExecutionTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "upstream request timeout")
}

type fusionStreamExecutionResult struct {
	execution *fusionRelayExecution
	err       *types.NewAPIError
}

type fusionStreamExecutionChannels struct {
	deltaChan  <-chan string
	resultChan <-chan fusionStreamExecutionResult
	cancel     context.CancelFunc
}

func fusionStreamOpenAIError(c *gin.Context, err *types.NewAPIError) types.OpenAIError {
	if err == nil {
		return types.OpenAIError{
			Message: common.MessageWithRequestId("fusion request failed", fusionRequestID(c)),
			Type:    "new_api_error",
			Code:    types.ErrorCodeBadResponse,
		}
	}
	openAIError := err.ToOpenAIError()
	openAIError.Message = common.MessageWithRequestId(openAIError.Message, fusionRequestID(c))
	return openAIError
}

func writeFusionChatCompletionStreamError(c *gin.Context, err *types.NewAPIError) {
	logFusionStreamError(c, "chat.completions", err)
	_ = helper.ObjectData(c, gin.H{"error": fusionStreamOpenAIError(c, err)})
	helper.Done(c)
}

func buildFusionResponsesFailedResponse(c *gin.Context, modelAlias string, err *types.NewAPIError) dto.OpenAIResponsesResponse {
	status, _ := common.Marshal("failed")
	return dto.OpenAIResponsesResponse{
		ID:                "resp_fusion_" + common.GetRandomString(12),
		Object:            "response",
		CreatedAt:         int(common.GetTimestamp()),
		Status:            status,
		Error:             fusionStreamOpenAIError(c, err),
		Model:             modelAlias,
		Output:            []dto.ResponsesOutput{},
		ParallelToolCalls: true,
		Store:             false,
	}
}

func writeFusionResponsesStreamError(c *gin.Context, modelAlias string, err *types.NewAPIError) {
	logFusionStreamError(c, "responses", err)
	response := buildFusionResponsesFailedResponse(c, modelAlias, err)
	writeFusionResponsesStreamEvent(c, "response.failed", dto.ResponsesStreamResponse{
		Response: &response,
	})
}

func logFusionStreamError(c *gin.Context, format string, err *types.NewAPIError) {
	message := "fusion request failed"
	if err != nil {
		message = err.Error()
	}
	common.SysError(fmt.Sprintf(
		"fusion %s stream error: request_id=%s err=%s",
		format,
		fusionRequestID(c),
		common.LocalLogPreview(message),
	))
}

func waitFusionStreamExecution(c *gin.Context, prepared *fusionRelayPreparedExecution, writeError func(*gin.Context, *types.NewAPIError)) (*fusionRelayExecution, bool) {
	resultChan := make(chan fusionStreamExecutionResult, 1)
	executionCtx, cancelExecution := context.WithCancel(c.Request.Context())
	defer cancelExecution()
	executionContext := c.Copy()
	if executionContext.Request != nil {
		executionContext.Request = executionContext.Request.WithContext(executionCtx)
	}
	go func() {
		execution, err := executePreparedFusionRelay(executionContext, prepared)
		result := fusionStreamExecutionResult{execution: execution}
		if err != nil {
			if newAPIError, ok := err.(*types.NewAPIError); ok {
				result.err = newAPIError
			} else {
				result.err = fusionExecutionNewAPIError(err)
			}
		}
		resultChan <- result
	}()

	if err := helper.PingData(c); err != nil {
		cancelExecution()
		return nil, false
	}

	ticker := time.NewTicker(fusionStreamPingInterval)
	defer ticker.Stop()
	for {
		select {
		case result := <-resultChan:
			if result.err != nil {
				writeError(c, result.err)
				return nil, false
			}
			return result.execution, true
		case <-ticker.C:
			if err := helper.PingData(c); err != nil {
				cancelExecution()
				return nil, false
			}
		case <-c.Request.Context().Done():
			cancelExecution()
			return nil, false
		}
	}
}

func startFusionStreamExecution(c *gin.Context, prepared *fusionRelayPreparedExecution) fusionStreamExecutionChannels {
	deltaChan := make(chan string, 16)
	resultChan := make(chan fusionStreamExecutionResult, 1)
	executionCtx, cancelExecution := context.WithCancel(c.Request.Context())
	executionContext := c.Copy()
	if executionContext.Request != nil {
		executionContext.Request = executionContext.Request.WithContext(executionCtx)
	}
	go func() {
		defer close(deltaChan)
		execution, err := executePreparedFusionRelayStream(executionContext, prepared, service.FusionStreamCallbacks{
			OnTextDelta: func(delta string) error {
				if delta == "" {
					return nil
				}
				select {
				case deltaChan <- delta:
					return nil
				case <-executionCtx.Done():
					return executionCtx.Err()
				}
			},
		})
		result := fusionStreamExecutionResult{execution: execution}
		if err != nil {
			if newAPIError, ok := err.(*types.NewAPIError); ok {
				result.err = newAPIError
			} else {
				result.err = fusionExecutionNewAPIError(err)
			}
		}
		resultChan <- result
	}()
	return fusionStreamExecutionChannels{
		deltaChan:  deltaChan,
		resultChan: resultChan,
		cancel:     cancelExecution,
	}
}

func streamFusionChatCompletion(c *gin.Context, prepared *fusionRelayPreparedExecution, modelAlias string) bool {
	channels := startFusionStreamExecution(c, prepared)
	defer channels.cancel()
	if err := helper.PingData(c); err != nil {
		return false
	}
	streamID := "chatcmpl-fusion-" + common.GetRandomString(12)
	created := common.GetTimestamp()
	ticker := time.NewTicker(fusionStreamPingInterval)
	defer ticker.Stop()
	sentDelta := false
	deltaChan := channels.deltaChan
	for {
		select {
		case delta, ok := <-deltaChan:
			if !ok {
				deltaChan = nil
				continue
			}
			writeFusionChatCompletionDelta(c, streamID, created, modelAlias, delta, !sentDelta)
			sentDelta = true
		case result := <-channels.resultChan:
			if result.err != nil {
				writeFusionChatCompletionStreamError(c, result.err)
				return false
			}
			if !sentDelta {
				writeFusionChatCompletionStream(c, modelAlias, result.execution.Result)
				return true
			}
			writeFusionChatCompletionStop(c, streamID, created, modelAlias, result.execution.Result)
			return true
		case <-ticker.C:
			if err := helper.PingData(c); err != nil {
				return false
			}
		case <-c.Request.Context().Done():
			return false
		}
	}
}

func streamFusionResponses(c *gin.Context, prepared *fusionRelayPreparedExecution, modelAlias string, stateContext fusionResponsesPreparedRequest) bool {
	channels := startFusionStreamExecution(c, prepared)
	defer channels.cancel()
	if err := helper.PingData(c); err != nil {
		return false
	}
	ticker := time.NewTicker(fusionStreamPingInterval)
	defer ticker.Stop()
	started := false
	sentDelta := false
	state := newFusionResponsesStreamState(modelAlias, newFusionResponseID(), stateContext.ParentResponseID)
	deltaChan := channels.deltaChan
	for {
		select {
		case delta, ok := <-deltaChan:
			if !ok {
				deltaChan = nil
				continue
			}
			if !started {
				state.start(c)
				started = true
			}
			state.delta(c, delta)
			sentDelta = true
		case result := <-channels.resultChan:
			if result.err != nil {
				writeFusionResponsesStreamError(c, modelAlias, result.err)
				return false
			}
			if !sentDelta {
				return writeFusionResponsesStream(c, modelAlias, result.execution.Result, stateContext)
			}
			return state.complete(c, result.execution.Result, stateContext)
		case <-ticker.C:
			if err := helper.PingData(c); err != nil {
				return false
			}
		case <-c.Request.Context().Done():
			return false
		}
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
	if billingResult, err := service.RunFusionBillingExpr(policy.Expression, input, policy); err == nil {
		other["billing"] = map[string]interface{}{
			"mode":                     fusion_setting.GetFusionBillingMode(),
			"expr":                     policy.Expression,
			"minimum_quota":            policy.MinimumQuota,
			"charge_failed_candidates": policy.ChargeFailedCandidates,
			"failed_candidate_quota":   policy.FailedCandidateQuota,
			"group_ratio":              policy.GroupRatio,
			"quota_before_group":       billingResult.QuotaBeforeGroup,
			"quota_after_group":        billingResult.QuotaAfterGroup,
			"final_quota":              actualQuota,
			"matched_vars":             billingResult.MatchedVars,
		}
	}
	return other
}

func recordFusionConsumeLog(c *gin.Context, relayInfo *relaycommon.RelayInfo, result *service.FusionEngineResult, preConsumedQuota int, actualQuota int, policy service.FusionBillingPolicy, isStream bool) {
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
		IsStream:         isStream,
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
	if request.Stream != nil && *request.Stream {
		prepared, ok := prepareFusionRelay(c, &request)
		if !ok {
			return
		}
		helper.SetEventStreamHeaders(c)
		streamFusionChatCompletion(c, prepared, request.Model)
		return
	}
	execution, ok := runFusionRelay(c, &request)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, buildFusionChatCompletionResponse(request.Model, execution.Result))
}

func fusionResponsesRequestToChatRequest(c *gin.Context, request *dto.OpenAIResponsesRequest) (fusionResponsesPreparedRequest, error) {
	if strings.TrimSpace(request.Model) == "" {
		return fusionResponsesPreparedRequest{}, errors.New("model is required")
	}
	parentMessages, callIDByItemID, err := loadFusionPreviousResponseState(c, request)
	if err != nil {
		return fusionResponsesPreparedRequest{}, err
	}
	messages := make([]dto.Message, 0)
	if len(request.Instructions) > 0 && common.GetJsonType(request.Instructions) == "string" {
		var instructions string
		if err := common.Unmarshal(request.Instructions, &instructions); err == nil && strings.TrimSpace(instructions) != "" {
			messages = append(messages, dto.Message{
				Role:    "system",
				Content: instructions,
			})
		}
	}
	messages = append(messages, parentMessages...)
	inputConversion, err := fusionResponsesInputToChatMessages(request.Input)
	if err != nil {
		return fusionResponsesPreparedRequest{}, err
	}
	for itemID, callID := range inputConversion.CallIDByItemID {
		callIDByItemID[itemID] = callID
	}
	canonicalizeFusionResponseToolMessages(inputConversion.Messages, callIDByItemID)
	messages = append(messages, inputConversion.Messages...)
	if len(messages) == 0 {
		return fusionResponsesPreparedRequest{}, errors.New("input is required")
	}
	chatRequest := &dto.GeneralOpenAIRequest{
		Model:         strings.TrimSpace(request.Model),
		Messages:      messages,
		Stream:        request.Stream,
		StreamOptions: request.StreamOptions,
		Temperature:   request.Temperature,
		TopP:          request.TopP,
		MaxTokens:     request.MaxOutputTokens,
		User:          request.User,
		Metadata:      request.Metadata,
		Store:         request.Store,
	}
	if len(request.Tools) > 0 {
		tools, err := fusionResponsesToolsToChatTools(request.Tools)
		if err != nil {
			return fusionResponsesPreparedRequest{}, err
		}
		chatRequest.Tools = tools
	}
	if len(request.ToolChoice) > 0 {
		toolChoice, err := fusionResponsesToolChoiceToChatToolChoice(request.ToolChoice)
		if err != nil {
			return fusionResponsesPreparedRequest{}, err
		}
		chatRequest.ToolChoice = toolChoice
	}
	if len(request.ParallelToolCalls) > 0 && common.GetJsonType(request.ParallelToolCalls) == "boolean" {
		var parallelToolCalls bool
		if err := common.Unmarshal(request.ParallelToolCalls, &parallelToolCalls); err == nil {
			chatRequest.ParallelTooCalls = &parallelToolCalls
		}
	}
	return fusionResponsesPreparedRequest{
		ChatRequest:      chatRequest,
		ParentResponseID: strings.TrimSpace(request.PreviousResponseID),
		Messages:         append([]dto.Message{}, messages...),
	}, nil
}

func loadFusionPreviousResponseState(c *gin.Context, request *dto.OpenAIResponsesRequest) ([]dto.Message, map[string]string, error) {
	callIDByItemID := make(map[string]string)
	previousResponseID := strings.TrimSpace(request.PreviousResponseID)
	if previousResponseID == "" {
		return nil, callIDByItemID, nil
	}
	state, err := model.GetFusionResponseState(previousResponseID, c.GetInt("id"), c.GetInt("token_id"))
	if err != nil {
		return nil, nil, errFusionResponseStateUnavailable
	}
	if strings.TrimSpace(state.ModelAlias) != strings.TrimSpace(request.Model) {
		return nil, nil, errFusionResponseStateUnavailable
	}
	payload, err := state.GetStatePayload()
	if err != nil {
		return nil, nil, errFusionResponseStateUnavailable
	}
	for itemID, callID := range payload.CallIDByItemID {
		addFusionResponseCallIDMapping(callIDByItemID, itemID, callID)
	}
	for _, output := range payload.Outputs {
		callID := strings.TrimSpace(output.CallId)
		if callID == "" {
			callID = strings.TrimSpace(output.ID)
		}
		addFusionResponseCallIDMapping(callIDByItemID, output.ID, callID)
	}
	return append([]dto.Message{}, payload.Messages...), callIDByItemID, nil
}

type fusionResponsesInputConversion struct {
	Messages       []dto.Message
	CallIDByItemID map[string]string
}

func fusionResponsesInputToChatMessages(raw []byte) (fusionResponsesInputConversion, error) {
	conversion := fusionResponsesInputConversion{
		Messages:       []dto.Message{},
		CallIDByItemID: map[string]string{},
	}
	if len(raw) == 0 {
		return conversion, nil
	}

	switch common.GetJsonType(raw) {
	case "string":
		var text string
		if err := common.Unmarshal(raw, &text); err != nil {
			return conversion, err
		}
		if strings.TrimSpace(text) == "" {
			return conversion, nil
		}
		conversion.Messages = append(conversion.Messages, dto.Message{Role: "user", Content: text})
		return conversion, nil
	case "object":
		var item map[string]interface{}
		if err := common.Unmarshal(raw, &item); err != nil {
			return conversion, err
		}
		itemMessages, err := fusionResponsesInputItemToChatMessages(item)
		if err != nil {
			return conversion, err
		}
		conversion.Messages = append(conversion.Messages, itemMessages...)
		mergeFusionResponseCallIDMappings(conversion.CallIDByItemID, fusionResponsesInputItemCallIDMappings(item))
		return conversion, nil
	case "array":
		var items []interface{}
		if err := common.Unmarshal(raw, &items); err != nil {
			return conversion, err
		}
		for _, rawItem := range items {
			switch item := rawItem.(type) {
			case string:
				if strings.TrimSpace(item) != "" {
					conversion.Messages = append(conversion.Messages, dto.Message{Role: "user", Content: item})
				}
			case map[string]interface{}:
				itemMessages, err := fusionResponsesInputItemToChatMessages(item)
				if err != nil {
					return conversion, err
				}
				conversion.Messages = append(conversion.Messages, itemMessages...)
				mergeFusionResponseCallIDMappings(conversion.CallIDByItemID, fusionResponsesInputItemCallIDMappings(item))
			default:
				return conversion, fmt.Errorf("fusion responses input array contains unsupported item %T", rawItem)
			}
		}
		return conversion, nil
	default:
		return conversion, fmt.Errorf("fusion responses does not support input JSON type %s in v1", common.GetJsonType(raw))
	}
}

func mergeFusionResponseCallIDMappings(target map[string]string, source map[string]string) {
	for itemID, callID := range source {
		target[itemID] = callID
	}
}

func fusionResponsesInputItemCallIDMappings(item map[string]interface{}) map[string]string {
	mapping := make(map[string]string)
	itemType := strings.TrimSpace(common.Interface2String(item["type"]))
	if itemType != "function_call" {
		return mapping
	}
	callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
	itemID := strings.TrimSpace(common.Interface2String(item["id"]))
	if callID == "" {
		callID = itemID
	}
	addFusionResponseCallIDMapping(mapping, itemID, callID)
	return mapping
}

func canonicalizeFusionResponseToolMessages(messages []dto.Message, callIDByItemID map[string]string) {
	if len(callIDByItemID) == 0 {
		return
	}
	for index := range messages {
		if messages[index].Role != "tool" {
			continue
		}
		toolCallID := strings.TrimSpace(messages[index].ToolCallId)
		if canonical, ok := callIDByItemID[toolCallID]; ok && strings.TrimSpace(canonical) != "" {
			messages[index].ToolCallId = canonical
		}
	}
}

func fusionResponsesInputItemToChatMessages(item map[string]interface{}) ([]dto.Message, error) {
	itemType := strings.TrimSpace(common.Interface2String(item["type"]))
	role := strings.TrimSpace(common.Interface2String(item["role"]))

	switch itemType {
	case "function_call":
		callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
		if callID == "" {
			callID = strings.TrimSpace(common.Interface2String(item["id"]))
		}
		name := strings.TrimSpace(common.Interface2String(item["name"]))
		if callID == "" || name == "" {
			return nil, errors.New("fusion responses function_call requires call_id and name")
		}
		toolCalls, err := common.Marshal([]dto.ToolCallRequest{
			{
				ID:   callID,
				Type: "function",
				Function: dto.FunctionRequest{
					Name:      name,
					Arguments: fusionResponsesStringOrJSON(item["arguments"]),
				},
			},
		})
		if err != nil {
			return nil, err
		}
		return []dto.Message{{
			Role:      "assistant",
			Content:   "",
			ToolCalls: toolCalls,
		}}, nil
	case "function_call_output":
		callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
		if callID == "" {
			callID = strings.TrimSpace(common.Interface2String(item["id"]))
		}
		if callID == "" {
			return nil, errors.New("fusion responses function_call_output requires call_id")
		}
		return []dto.Message{{
			Role:       "tool",
			ToolCallId: callID,
			Content:    fusionResponsesToolOutputString(item),
		}}, nil
	case "input_text", "output_text", "text":
		text := strings.TrimSpace(common.Interface2String(item["text"]))
		if text == "" {
			return nil, nil
		}
		if role == "" {
			role = "user"
		}
		return []dto.Message{{Role: role, Content: text}}, nil
	case "input_image":
		if role == "" {
			role = "user"
		}
		imageURL := fusionResponsesImageURLString(item["image_url"])
		if strings.TrimSpace(imageURL) == "" {
			return nil, nil
		}
		return []dto.Message{{
			Role: role,
			Content: []map[string]interface{}{
				{
					"type": dto.ContentTypeImageURL,
					"image_url": map[string]interface{}{
						"url":    imageURL,
						"detail": common.Interface2String(item["detail"]),
					},
				},
			},
		}}, nil
	case "message", "":
		if role == "" {
			role = "user"
		}
		content, ok, err := fusionResponsesContentToChatContent(item["content"], role)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		return []dto.Message{{Role: role, Content: content}}, nil
	case "reasoning":
		return nil, nil
	default:
		return nil, fmt.Errorf("fusion responses does not support input type %s in v1", itemType)
	}
}

func fusionResponsesContentToChatContent(content interface{}, role string) (interface{}, bool, error) {
	switch value := content.(type) {
	case nil:
		return "", false, nil
	case string:
		return value, true, nil
	case []interface{}:
		texts := make([]string, 0, len(value))
		parts := make([]map[string]interface{}, 0, len(value))
		textOnly := true
		for _, rawPart := range value {
			switch part := rawPart.(type) {
			case string:
				if strings.TrimSpace(part) != "" {
					texts = append(texts, part)
					parts = append(parts, map[string]interface{}{
						"type": dto.ContentTypeText,
						"text": part,
					})
				}
			case map[string]interface{}:
				partType := strings.TrimSpace(common.Interface2String(part["type"]))
				switch partType {
				case "input_text", "output_text", "text", "":
					text := common.Interface2String(part["text"])
					if strings.TrimSpace(text) != "" {
						texts = append(texts, text)
						parts = append(parts, map[string]interface{}{
							"type": dto.ContentTypeText,
							"text": text,
						})
					}
				case "input_image":
					if role == "assistant" || role == "tool" {
						return nil, false, fmt.Errorf("fusion responses does not support %s image content in v1", role)
					}
					imageURL := fusionResponsesImageURLString(part["image_url"])
					if strings.TrimSpace(imageURL) == "" {
						continue
					}
					textOnly = false
					parts = append(parts, map[string]interface{}{
						"type": dto.ContentTypeImageURL,
						"image_url": map[string]interface{}{
							"url":    imageURL,
							"detail": common.Interface2String(part["detail"]),
						},
					})
				case "input_file":
					return nil, false, errors.New("fusion responses does not support input type input_file in v1")
				default:
					return nil, false, fmt.Errorf("fusion responses does not support content part type %s in v1", partType)
				}
			default:
				return nil, false, fmt.Errorf("fusion responses content contains unsupported part %T", rawPart)
			}
		}
		if len(parts) == 0 {
			return "", false, nil
		}
		if textOnly {
			return strings.Join(texts, "\n"), true, nil
		}
		return parts, true, nil
	case map[string]interface{}:
		return fusionResponsesContentToChatContent([]interface{}{value}, role)
	default:
		return fusionResponsesStringOrJSON(value), true, nil
	}
}

func fusionResponsesToolOutputString(item map[string]interface{}) string {
	if output, ok := item["output"]; ok {
		return fusionResponsesStringOrJSON(output)
	}
	if content, ok := item["content"]; ok {
		converted, ok, err := fusionResponsesContentToChatContent(content, "tool")
		if err == nil && ok {
			return fusionResponsesStringOrJSON(converted)
		}
		return fusionResponsesStringOrJSON(content)
	}
	return ""
}

func fusionResponsesStringOrJSON(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		data, err := common.Marshal(typed)
		if err != nil {
			return common.Interface2String(typed)
		}
		return string(data)
	}
}

func fusionResponsesImageURLString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]interface{}:
		return common.Interface2String(typed["url"])
	default:
		return ""
	}
}

func fusionResponsesToolsToChatTools(raw []byte) ([]dto.ToolCallRequest, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var tools []map[string]interface{}
	if err := common.Unmarshal(raw, &tools); err != nil {
		return nil, fmt.Errorf("invalid responses tools: %w", err)
	}
	chatTools := make([]dto.ToolCallRequest, 0, len(tools))
	for index, tool := range tools {
		toolType, _ := tool["type"].(string)
		if toolType == "" {
			toolType = "function"
		}
		if toolType != "function" {
			return nil, fmt.Errorf("fusion responses does not support tool type %s in v1", toolType)
		}
		functionMap, _ := tool["function"].(map[string]interface{})
		if functionMap == nil {
			functionMap = tool
		}
		name, _ := functionMap["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("responses tools[%d].name is required", index)
		}
		description, _ := functionMap["description"].(string)
		chatTools = append(chatTools, dto.ToolCallRequest{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        name,
				Description: description,
				Parameters:  functionMap["parameters"],
			},
		})
	}
	return chatTools, nil
}

func fusionResponsesToolChoiceToChatToolChoice(raw []byte) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	switch common.GetJsonType(raw) {
	case "string":
		var choice string
		if err := common.Unmarshal(raw, &choice); err != nil {
			return nil, err
		}
		if choice == "auto" || choice == "none" || choice == "required" {
			return choice, nil
		}
		return map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name": choice,
			},
		}, nil
	case "object":
		var choice map[string]interface{}
		if err := common.Unmarshal(raw, &choice); err != nil {
			return nil, err
		}
		toolType, _ := choice["type"].(string)
		if toolType == "" {
			toolType = "function"
		}
		if toolType != "function" {
			return nil, fmt.Errorf("fusion responses does not support tool_choice type %s in v1", toolType)
		}
		if functionMap, ok := choice["function"].(map[string]interface{}); ok {
			if name, _ := functionMap["name"].(string); strings.TrimSpace(name) != "" {
				return choice, nil
			}
		}
		name, _ := choice["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, errors.New("responses tool_choice.name is required")
		}
		return map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name": name,
			},
		}, nil
	default:
		var choice any
		if err := common.Unmarshal(raw, &choice); err != nil {
			return nil, err
		}
		return choice, nil
	}
}

func FusionResponses(c *gin.Context) {
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

	var responsesRequest dto.OpenAIResponsesRequest
	if err := common.UnmarshalBodyReusable(c, &responsesRequest); err != nil {
		status := http.StatusBadRequest
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		fusionOpenAIError(c, status, err.Error(), types.ErrorCodeInvalidRequest)
		return
	}
	preparedRequest, err := fusionResponsesRequestToChatRequest(c, &responsesRequest)
	if err != nil {
		fusionOpenAIError(c, http.StatusBadRequest, err.Error(), types.ErrorCodeInvalidRequest)
		return
	}
	chatRequest := preparedRequest.ChatRequest
	if err := validateFusionRelayChatRequest(chatRequest); err != nil {
		fusionOpenAIError(c, http.StatusBadRequest, err.Error(), types.ErrorCodeInvalidRequest)
		return
	}
	if responsesRequest.Stream != nil && *responsesRequest.Stream {
		prepared, ok := prepareFusionRelay(c, chatRequest)
		if !ok {
			return
		}
		helper.SetEventStreamHeaders(c)
		streamFusionResponses(c, prepared, chatRequest.Model, preparedRequest)
		return
	}
	execution, ok := runFusionRelay(c, chatRequest)
	if !ok {
		return
	}
	responseID := newFusionResponseID()
	response := buildFusionResponsesResponse(chatRequest.Model, responseID, preparedRequest.ParentResponseID, execution.Result)
	if err := saveFusionResponseState(c, chatRequest.Model, response.ID, preparedRequest.ParentResponseID, preparedRequest.Messages, response.Output); err != nil {
		fusionOpenAIError(c, http.StatusInternalServerError, err.Error(), types.ErrorCodeBadResponse)
		return
	}
	c.JSON(http.StatusOK, response)
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
		Id:             key.Id,
		Name:           key.Name,
		Provider:       key.Provider,
		TemplateID:     key.TemplateID,
		BaseURL:        key.BaseURL,
		DefaultModel:   key.DefaultModel,
		Models:         models,
		UpstreamConfig: key.UpstreamConfig,
		APIKeyHint:     key.APIKeyHint,
		Status:         key.Status,
		LastTestTime:   key.LastTestTime,
		LastError:      key.LastError,
		CreatedAt:      key.CreatedAt,
		UpdatedAt:      key.UpdatedAt,
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

func buildFusionUpstreamTemplateFromRequest(req dto.FusionUpstreamTemplateRequest) *model.FusionUpstreamTemplate {
	return &model.FusionUpstreamTemplate{
		Name:                 req.Name,
		ProviderLabel:        req.ProviderLabel,
		Protocol:             req.Protocol,
		EndpointPath:         req.EndpointPath,
		AuthType:             req.AuthType,
		AuthHeader:           req.AuthHeader,
		AuthQueryName:        req.AuthQueryName,
		DefaultHeaders:       req.DefaultHeaders,
		DefaultQuery:         req.DefaultQuery,
		DefaultBodyOverrides: req.DefaultBodyOverrides,
		DetectRules:          req.DetectRules,
		Enabled:              req.Enabled,
		Sort:                 req.Sort,
	}
}

func buildFusionUpstreamTemplateResponse(template *model.FusionUpstreamTemplate) dto.FusionUpstreamTemplateResponse {
	return dto.FusionUpstreamTemplateResponse{
		Id:                   template.Id,
		Name:                 template.Name,
		ProviderLabel:        template.ProviderLabel,
		Protocol:             template.Protocol,
		EndpointPath:         template.EndpointPath,
		AuthType:             template.AuthType,
		AuthHeader:           template.AuthHeader,
		AuthQueryName:        template.AuthQueryName,
		DefaultHeaders:       template.DefaultHeaders,
		DefaultQuery:         template.DefaultQuery,
		DefaultBodyOverrides: template.DefaultBodyOverrides,
		DetectRules:          template.DetectRules,
		Enabled:              template.Enabled,
		Sort:                 template.Sort,
		CreatedAt:            template.CreatedAt,
		UpdatedAt:            template.UpdatedAt,
	}
}

func buildFusionUpstreamTemplateResponses(templates []*model.FusionUpstreamTemplate) []dto.FusionUpstreamTemplateResponse {
	responses := make([]dto.FusionUpstreamTemplateResponse, 0, len(templates))
	for _, template := range templates {
		responses = append(responses, buildFusionUpstreamTemplateResponse(template))
	}
	return responses
}

func validateFusionAPIKeyFields(name, provider, baseURL, defaultModel string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	if len(strings.TrimSpace(name)) > fusionNameMaxLength {
		return fmt.Errorf("name must be at most %d characters", fusionNameMaxLength)
	}
	if len(strings.TrimSpace(provider)) > 64 {
		return errors.New("provider must be at most 64 characters")
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

func resolveFusionTemplateForKeyRequest(templateID int) (*model.FusionUpstreamTemplate, error) {
	if templateID > 0 {
		return model.GetEnabledFusionUpstreamTemplateByID(templateID)
	}
	template, err := model.GetDefaultEnabledFusionUpstreamTemplate()
	if err == nil {
		return template, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.EnsureDefaultFusionUpstreamTemplate()
	}
	return nil, err
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
	template, err := resolveFusionTemplateForKeyRequest(req.TemplateID)
	if err != nil {
		return nil, err
	}
	if req.Provider == "" {
		req.Provider = model.FusionProviderOpenAICompatible
	}
	key := &model.FusionAPIKey{
		UserId:       userId,
		Name:         req.Name,
		Provider:     req.Provider,
		TemplateID:   template.Id,
		BaseURL:      normalizedBaseURL,
		DefaultModel: req.DefaultModel,
		Status:       model.FusionKeyStatusEnabled,
	}
	key.Normalize()
	if err := key.SetUpstreamConfig(req.UpstreamConfig); err != nil {
		return nil, err
	}
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
	templateID := req.TemplateID
	if templateID == 0 {
		templateID = existing.TemplateID
	}
	template, err := resolveFusionTemplateForKeyRequest(templateID)
	if err != nil {
		return nil, err
	}
	if req.Provider == "" {
		req.Provider = existing.Provider
	}
	if req.Provider == "" {
		req.Provider = model.FusionProviderOpenAICompatible
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
		TemplateID:       template.Id,
		BaseURL:          normalizedBaseURL,
		DefaultModel:     req.DefaultModel,
		UpstreamConfig:   existing.UpstreamConfig,
		APIKeyCiphertext: existing.APIKeyCiphertext,
		APIKeyHint:       existing.APIKeyHint,
		KeyFingerprint:   existing.KeyFingerprint,
		Status:           status,
		LastError:        existing.LastError,
		LastTestTime:     existing.LastTestTime,
	}
	key.Normalize()
	if err := key.SetUpstreamConfig(req.UpstreamConfig); err != nil {
		return nil, err
	}
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

func GetFusionUpstreamTemplates(c *gin.Context) {
	templates, err := model.ListFusionUpstreamTemplates(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items": buildFusionUpstreamTemplateResponses(templates),
		"total": len(templates),
	})
}

func AdminGetFusionUpstreamTemplates(c *gin.Context) {
	templates, err := model.ListFusionUpstreamTemplates(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items": buildFusionUpstreamTemplateResponses(templates),
		"total": len(templates),
	})
}

func AdminCreateFusionUpstreamTemplate(c *gin.Context) {
	var req dto.FusionUpstreamTemplateRequest
	if !bindFusionJSON(c, &req) {
		return
	}
	template := buildFusionUpstreamTemplateFromRequest(req)
	if err := template.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildFusionUpstreamTemplateResponse(template))
}

func AdminUpdateFusionUpstreamTemplate(c *gin.Context) {
	id, ok := parseFusionID(c)
	if !ok {
		return
	}
	var req dto.FusionUpstreamTemplateRequest
	if !bindFusionJSON(c, &req) {
		return
	}
	existing, err := model.GetFusionUpstreamTemplateByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	template := buildFusionUpstreamTemplateFromRequest(req)
	template.Id = existing.Id
	if err := template.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	updated, err := model.GetFusionUpstreamTemplateByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildFusionUpstreamTemplateResponse(updated))
}

func AdminDeleteFusionUpstreamTemplate(c *gin.Context) {
	id, ok := parseFusionID(c)
	if !ok {
		return
	}
	if err := model.DeleteFusionUpstreamTemplateByID(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
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
		candidates, err := config.GetCandidates()
		if err != nil {
			return false, err
		}
		for _, candidate := range candidates {
			if candidate.KeyID == keyId {
				return true, nil
			}
		}
	}
	return false, nil
}

func fusionUpstreamConfigToMap(config model.FusionUpstreamConfig) map[string]interface{} {
	data, err := common.Marshal(config)
	if err != nil {
		return map[string]interface{}{}
	}
	result := map[string]interface{}{}
	if err := common.Unmarshal(data, &result); err != nil {
		return map[string]interface{}{}
	}
	return result
}

func buildFusionAPIKeyTestResponse(result service.FusionUpstreamTestResult) dto.FusionAPIKeyTestResponse {
	return dto.FusionAPIKeyTestResponse{
		OK:             result.OK,
		Status:         result.Status,
		Message:        result.Message,
		DetectedConfig: fusionUpstreamConfigToMap(result.DetectedConfig),
	}
}

func TestUnsavedFusionAPIKey(c *gin.Context) {
	if !validateFusionCryptoSecretGate(c) {
		return
	}
	userId := c.GetInt("id")
	var req dto.FusionAPIKeyTestRequest
	if !bindFusionJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "Fusion upstream test"
	}
	key, err := prepareFusionAPIKeyForCreate(userId, req.FusionAPIKeyCreateRequest)
	if err != nil {
		if strings.Contains(err.Error(), "CRYPTO_SECRET") {
			fusionError(c, http.StatusServiceUnavailable, err)
			return
		}
		common.ApiError(c, err)
		return
	}
	result := service.TestFusionUpstreamKey(c.Request.Context(), service.FusionUpstreamTestRequest{
		UserID: userId,
		Key:    key,
	})
	common.ApiSuccess(c, buildFusionAPIKeyTestResponse(result))
}

func TestFusionAPIKey(c *gin.Context) {
	if !validateFusionCryptoSecretGate(c) {
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
	testKey := key
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		var req dto.FusionAPIKeyUpdateRequest
		if !bindFusionJSON(c, &req) {
			return
		}
		testKey, err = prepareFusionAPIKeyForUpdate(key, req)
		if err != nil {
			if strings.Contains(err.Error(), "CRYPTO_SECRET") {
				fusionError(c, http.StatusServiceUnavailable, err)
				return
			}
			common.ApiError(c, err)
			return
		}
		if testKey.Status != model.FusionKeyStatusEnabled {
			common.ApiError(c, errors.New("fusion api key is disabled"))
			return
		}
	}
	result := service.TestFusionUpstreamKey(c.Request.Context(), service.FusionUpstreamTestRequest{
		UserID: userId,
		Key:    testKey,
	})
	lastError := result.Message
	if result.OK {
		lastError = ""
	}
	if err := model.DB.Model(&model.FusionAPIKey{}).
		Where("id = ? AND user_id = ?", key.Id, userId).
		Updates(map[string]interface{}{
			"last_test_time": common.GetTimestamp(),
			"last_error":     lastError,
		}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildFusionAPIKeyTestResponse(result))
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

func normalizeFusionCandidates(req dto.FusionConfigCreateRequest) ([]model.FusionCandidate, error) {
	if len(req.Candidates) > 0 {
		candidates := make([]model.FusionCandidate, 0, len(req.Candidates))
		for _, candidate := range req.Candidates {
			modelName := strings.TrimSpace(candidate.Model)
			if candidate.KeyID <= 0 {
				return nil, errors.New("candidates contains invalid key id")
			}
			if modelName == "" {
				return nil, errors.New("candidate model is required")
			}
			candidates = append(candidates, model.FusionCandidate{
				KeyID: candidate.KeyID,
				Model: modelName,
			})
		}
		return candidates, nil
	}

	candidateIDs, err := normalizeCandidateIDs(req.CandidateKeyIDs)
	if err != nil {
		return nil, err
	}
	if err := validateCandidateModels(candidateIDs, req.CandidateModels); err != nil {
		return nil, err
	}
	candidates := make([]model.FusionCandidate, 0, len(candidateIDs))
	for _, keyID := range candidateIDs {
		candidates = append(candidates, model.FusionCandidate{
			KeyID: keyID,
			Model: strings.TrimSpace(req.CandidateModels[strconv.Itoa(keyID)]),
		})
	}
	return candidates, nil
}

func buildFusionCandidateDTOs(candidates []model.FusionCandidate) []dto.FusionCandidateConfig {
	items := make([]dto.FusionCandidateConfig, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, dto.FusionCandidateConfig{
			KeyID: candidate.KeyID,
			Model: candidate.Model,
		})
	}
	return items
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
	candidates, err := normalizeFusionCandidates(req)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, errors.New("at least one candidate model is required")
	}
	maxCandidates := fusion_setting.GetFusionMaxCandidatesPerConfig()
	if maxCandidates > 0 && len(candidates) > maxCandidates {
		return nil, fmt.Errorf("candidate model count exceeds limit: %d", maxCandidates)
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
	if err := config.SetCandidates(candidates); err != nil {
		return nil, err
	}
	if err := config.SyncLegacyCandidateFields(candidates); err != nil {
		return nil, err
	}
	if err := model.ValidateFusionConfigKeyOwnership(userId, config); err != nil {
		return nil, err
	}
	return config, nil
}

func buildFusionConfigResponse(config *model.FusionConfig) (dto.FusionConfigResponse, error) {
	candidates, err := config.GetCandidates()
	if err != nil {
		return dto.FusionConfigResponse{}, err
	}
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
		Candidates:      buildFusionCandidateDTOs(candidates),
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
