package fusion_setting

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/expr-lang/expr"
)

const (
	BillingModeExpr          = "expr"
	DefaultBillingExpression = "max(min_quota, (cp + cc) * 0.20 + (jp + jc) * 0.50 + failed * failed_quota)"
)

type FusionSetting struct {
	Enabled                      bool     `json:"enabled"`
	MaxKeysPerUser               int      `json:"max_keys_per_user"`
	MaxConfigsPerUser            int      `json:"max_configs_per_user"`
	MaxCandidatesPerConfig       int      `json:"max_candidates_per_config"`
	MaxParallel                  int      `json:"max_parallel"`
	DefaultTimeoutMS             int      `json:"default_timeout_ms"`
	MaxTimeoutMS                 int      `json:"max_timeout_ms"`
	ServiceModelName             string   `json:"service_model_name"`
	BillingMode                  string   `json:"billing_mode"`
	BillingExpr                  string   `json:"billing_expr"`
	MinimumQuota                 int      `json:"minimum_quota"`
	ChargeFailedCandidates       bool     `json:"charge_failed_candidates"`
	FailedCandidateQuota         int      `json:"failed_candidate_quota"`
	KeyTestQuota                 int      `json:"key_test_quota"`
	MaxJudgeInputTokens          int      `json:"max_judge_input_tokens"`
	MaxCandidateOutputChars      int      `json:"max_candidate_output_chars"`
	StreamCandidateBrief         bool     `json:"stream_candidate_brief"`
	StreamCandidateMaxTokens     int      `json:"stream_candidate_max_tokens"`
	ResultCacheEnabled           bool     `json:"result_cache_enabled"`
	ResultCacheTTLSeconds        int      `json:"result_cache_ttl_seconds"`
	ResultCacheMaxPayloadBytes   int      `json:"result_cache_max_payload_bytes"`
	ResponseStateTTLSeconds      int      `json:"response_state_ttl_seconds"`
	ResponseStateMaxPayloadBytes int      `json:"response_state_max_payload_bytes"`
	AllowPrivateBaseURL          bool     `json:"allow_private_base_url"`
	AllowedBaseURLDomains        []string `json:"allowed_base_url_domains"`
	AllowedBaseURLPorts          []int    `json:"allowed_base_url_ports"`
	AllowedTokenGroups           []string `json:"allowed_token_groups"`
}

var fusionSetting = defaultFusionSetting()

func init() {
	config.GlobalConfig.Register("fusion_setting", &fusionSetting)
}

func defaultFusionSetting() FusionSetting {
	return FusionSetting{
		Enabled:                      false,
		MaxKeysPerUser:               10,
		MaxConfigsPerUser:            10,
		MaxCandidatesPerConfig:       4,
		MaxParallel:                  4,
		DefaultTimeoutMS:             45000,
		MaxTimeoutMS:                 90000,
		ServiceModelName:             "fusion-service",
		BillingMode:                  BillingModeExpr,
		BillingExpr:                  DefaultBillingExpression,
		MinimumQuota:                 1,
		ChargeFailedCandidates:       false,
		FailedCandidateQuota:         0,
		KeyTestQuota:                 1,
		MaxJudgeInputTokens:          128000,
		MaxCandidateOutputChars:      6000,
		StreamCandidateBrief:         true,
		StreamCandidateMaxTokens:     1024,
		ResultCacheEnabled:           false,
		ResultCacheTTLSeconds:        300,
		ResultCacheMaxPayloadBytes:   262144,
		ResponseStateTTLSeconds:      86400,
		ResponseStateMaxPayloadBytes: 2097152,
		AllowPrivateBaseURL:          false,
		AllowedBaseURLDomains:        []string{},
		AllowedBaseURLPorts:          []int{443},
		AllowedTokenGroups:           []string{},
	}
}

func resetFusionSettingForTest() {
	fusionSetting = defaultFusionSetting()
}

func IsFusionEnabled() bool {
	return fusionSetting.Enabled
}

func GetFusionMaxKeysPerUser() int {
	return fusionSetting.MaxKeysPerUser
}

func GetFusionMaxConfigsPerUser() int {
	return fusionSetting.MaxConfigsPerUser
}

func GetFusionMaxCandidatesPerConfig() int {
	return fusionSetting.MaxCandidatesPerConfig
}

func GetFusionMaxParallel() int {
	return fusionSetting.MaxParallel
}

func GetFusionDefaultTimeoutMS() int {
	return fusionSetting.DefaultTimeoutMS
}

func GetFusionMaxTimeoutMS() int {
	return fusionSetting.MaxTimeoutMS
}

func GetFusionServiceModelName() string {
	return fusionSetting.ServiceModelName
}

func GetFusionBillingMode() string {
	if strings.TrimSpace(fusionSetting.BillingMode) == "" {
		return BillingModeExpr
	}
	return fusionSetting.BillingMode
}

func GetFusionBillingExpr() string {
	if strings.TrimSpace(fusionSetting.BillingExpr) == "" {
		return DefaultBillingExpression
	}
	return fusionSetting.BillingExpr
}

func GetFusionMinimumQuota() int {
	return fusionSetting.MinimumQuota
}

func ShouldFusionChargeFailedCandidates() bool {
	return fusionSetting.ChargeFailedCandidates
}

func GetFusionFailedCandidateQuota() int {
	return fusionSetting.FailedCandidateQuota
}

func GetFusionKeyTestQuota() int {
	return fusionSetting.KeyTestQuota
}

func GetFusionMaxJudgeInputTokens() int {
	return fusionSetting.MaxJudgeInputTokens
}

func GetFusionMaxCandidateOutputChars() int {
	return fusionSetting.MaxCandidateOutputChars
}

func ShouldFusionUseStreamCandidateBrief() bool {
	return fusionSetting.StreamCandidateBrief
}

func GetFusionStreamCandidateMaxTokens() int {
	return fusionSetting.StreamCandidateMaxTokens
}

func IsFusionResultCacheEnabled() bool {
	return fusionSetting.ResultCacheEnabled
}

func GetFusionResultCacheTTLSeconds() int {
	return fusionSetting.ResultCacheTTLSeconds
}

func GetFusionResultCacheMaxPayloadBytes() int {
	return fusionSetting.ResultCacheMaxPayloadBytes
}

func GetFusionResponseStateTTLSeconds() int {
	return fusionSetting.ResponseStateTTLSeconds
}

func GetFusionResponseStateMaxPayloadBytes() int {
	return fusionSetting.ResponseStateMaxPayloadBytes
}

func IsFusionPrivateBaseURLAllowed() bool {
	return fusionSetting.AllowPrivateBaseURL
}

func GetFusionAllowedBaseURLDomains() []string {
	return append([]string{}, fusionSetting.AllowedBaseURLDomains...)
}

func GetFusionAllowedBaseURLPorts() []int {
	return append([]int{}, fusionSetting.AllowedBaseURLPorts...)
}

func GetFusionAllowedTokenGroups() []string {
	return append([]string{}, fusionSetting.AllowedTokenGroups...)
}

func IsFusionTokenGroup(group string) bool {
	group = strings.TrimSpace(group)
	if group == "" {
		return false
	}
	for _, allowedGroup := range fusionSetting.AllowedTokenGroups {
		if strings.TrimSpace(allowedGroup) == group {
			return true
		}
	}
	return false
}

func ValidateFusionBillingExpr(exprStr string) error {
	exprStr = strings.TrimSpace(exprStr)
	if exprStr == "" {
		return fmt.Errorf("fusion billing expression is required")
	}
	_, err := expr.Compile(exprStr, expr.Env(fusionBillingExprEnv()), expr.AsFloat64())
	if err != nil {
		return fmt.Errorf("fusion billing expression compile failed: %w", err)
	}
	return nil
}

func fusionBillingExprEnv() map[string]interface{} {
	return map[string]interface{}{
		"cp":                   float64(0),
		"cc":                   float64(0),
		"jp":                   float64(0),
		"jc":                   float64(0),
		"candidate_prompt":     float64(0),
		"candidate_completion": float64(0),
		"judge_prompt":         float64(0),
		"judge_completion":     float64(0),
		"failed":               float64(0),
		"failed_prompt":        float64(0),
		"failed_quota":         float64(0),
		"success":              float64(0),
		"total":                float64(0),
		"min_quota":            float64(0),
		"max":                  math.Max,
		"min":                  math.Min,
		"abs":                  math.Abs,
		"ceil":                 math.Ceil,
		"floor":                math.Floor,
	}
}
