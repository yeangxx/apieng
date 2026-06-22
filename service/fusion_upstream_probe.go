package service

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

type FusionUpstreamTestRequest struct {
	UserID     int
	Key        *model.FusionAPIKey
	Model      string
	HTTPClient *http.Client
}

type FusionUpstreamTestResult struct {
	OK             bool
	Status         int
	Message        string
	DetectedConfig model.FusionUpstreamConfig
}

func TestFusionUpstreamKey(ctx context.Context, request FusionUpstreamTestRequest) FusionUpstreamTestResult {
	if request.Key == nil {
		return FusionUpstreamTestResult{Message: "fusion api key is required"}
	}
	modelName := strings.TrimSpace(request.Model)
	if modelName == "" {
		modelName = request.Key.DefaultModel
	}
	target, err := resolveFusionCallTarget(request.Key, modelName)
	template, templateErr := model.GetFusionTemplateForKey(request.Key)
	if err != nil {
		return FusionUpstreamTestResult{Message: sanitizeFusionError(err)}
	}
	if templateErr != nil {
		return FusionUpstreamTestResult{Message: sanitizeFusionError(templateErr)}
	}
	maxTokens := uint(1)
	callRequest := &dto.GeneralOpenAIRequest{
		Model: modelName,
		Messages: []dto.Message{
			{Role: "user", Content: "ping"},
		},
		MaxTokens: common.GetPointer(maxTokens),
		Stream:    common.GetPointer(false),
	}
	client := fusionNoRedirectClient(request.HTTPClient)
	result := executeFusionChatCall(ctx, nil, client, target, callRequest, time.Now(), 0, nil)
	if result.Success {
		return FusionUpstreamTestResult{
			OK:      true,
			Status:  result.UpstreamStatus,
			Message: "fusion upstream test succeeded",
		}
	}
	detected := DetectUpstreamProtocolConfig(template, result.UpstreamStatus, result.SanitizedError)
	return FusionUpstreamTestResult{
		OK:             false,
		Status:         result.UpstreamStatus,
		Message:        result.SanitizedError,
		DetectedConfig: detected,
	}
}

func DetectUpstreamProtocolConfig(template *model.UpstreamProtocolTemplate, status int, message string) model.UpstreamProtocolConfig {
	if template == nil {
		return model.UpstreamProtocolConfig{}
	}
	rules, err := template.GetDetectRules()
	if err != nil {
		return model.UpstreamProtocolConfig{}
	}
	messageLower := strings.ToLower(message)
	for _, rule := range rules {
		if !fusionDetectStatusMatches(rule.StatusCodes, status) {
			continue
		}
		if !fusionDetectTextMatches(rule.ErrorContains, messageLower) {
			continue
		}
		return rule.SuggestedConfig
	}
	return model.UpstreamProtocolConfig{}
}

func fusionDetectStatusMatches(statusCodes []int, status int) bool {
	if len(statusCodes) == 0 {
		return true
	}
	for _, statusCode := range statusCodes {
		if statusCode == status {
			return true
		}
	}
	return false
}

func fusionDetectTextMatches(needles []string, messageLower string) bool {
	if len(needles) == 0 {
		return true
	}
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle != "" && strings.Contains(messageLower, needle) {
			return true
		}
	}
	return false
}
