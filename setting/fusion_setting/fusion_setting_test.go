package fusion_setting

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFusionSettingDefaults(t *testing.T) {
	resetFusionSettingForTest()
	t.Cleanup(resetFusionSettingForTest)

	assert.False(t, IsFusionEnabled())
	assert.Equal(t, 10, GetFusionMaxKeysPerUser())
	assert.Equal(t, 10, GetFusionMaxConfigsPerUser())
	assert.Equal(t, 4, GetFusionMaxCandidatesPerConfig())
	assert.Equal(t, 4, GetFusionMaxParallel())
	assert.Equal(t, 45000, GetFusionDefaultTimeoutMS())
	assert.Equal(t, 90000, GetFusionMaxTimeoutMS())
	assert.Equal(t, "fusion-service", GetFusionServiceModelName())
	assert.Equal(t, BillingModeExpr, GetFusionBillingMode())
	assert.Equal(t, DefaultBillingExpression, GetFusionBillingExpr())
	assert.Equal(t, 1, GetFusionMinimumQuota())
	assert.False(t, ShouldFusionChargeFailedCandidates())
	assert.Equal(t, 0, GetFusionFailedCandidateQuota())
	assert.Equal(t, 1, GetFusionKeyTestQuota())
	assert.Equal(t, 128000, GetFusionMaxJudgeInputTokens())
	assert.Equal(t, 6000, GetFusionMaxCandidateOutputChars())
	assert.True(t, ShouldFusionUseStreamCandidateBrief())
	assert.Equal(t, 1024, GetFusionStreamCandidateMaxTokens())
	assert.False(t, IsFusionResultCacheEnabled())
	assert.Equal(t, 300, GetFusionResultCacheTTLSeconds())
	assert.Equal(t, 262144, GetFusionResultCacheMaxPayloadBytes())
	assert.Equal(t, 86400, GetFusionResponseStateTTLSeconds())
	assert.Equal(t, 2097152, GetFusionResponseStateMaxPayloadBytes())
	assert.False(t, IsFusionPrivateBaseURLAllowed())
	assert.Equal(t, []string{}, GetFusionAllowedBaseURLDomains())
	assert.Equal(t, []int{443}, GetFusionAllowedBaseURLPorts())
	assert.Equal(t, []string{}, GetFusionAllowedTokenGroups())
	assert.Equal(t, "", GetFusionCandidateSystemPrompt())
	assert.Equal(t, "", GetFusionJudgeSystemPrompt())
	assert.False(t, IsFusionTokenGroup("fusion-basic"))
}

func TestFusionSettingLoadFromDB(t *testing.T) {
	resetFusionSettingForTest()
	t.Cleanup(resetFusionSettingForTest)

	err := config.GlobalConfig.LoadFromDB(map[string]string{
		"fusion_setting.enabled":                          "true",
		"fusion_setting.max_keys_per_user":                "20",
		"fusion_setting.billing_expr":                     "max(min_quota, cp + jp + failed * failed_quota)",
		"fusion_setting.charge_failed_candidates":         "true",
		"fusion_setting.stream_candidate_brief":           "false",
		"fusion_setting.stream_candidate_max_tokens":      "384",
		"fusion_setting.result_cache_enabled":             "true",
		"fusion_setting.result_cache_ttl_seconds":         "120",
		"fusion_setting.result_cache_max_payload_bytes":   "65536",
		"fusion_setting.response_state_ttl_seconds":       "3600",
		"fusion_setting.response_state_max_payload_bytes": "1048576",
		"fusion_setting.allowed_base_url_domains":         `["example.com","*.example.org"]`,
		"fusion_setting.allowed_base_url_ports":           `[443,8443]`,
		"fusion_setting.allowed_token_groups":             `["fusion-basic","fusion-pro"]`,
		"fusion_setting.candidate_system_prompt":          "candidate admin prompt",
		"fusion_setting.judge_system_prompt":              "judge admin prompt",
	})

	require.NoError(t, err)
	assert.True(t, IsFusionEnabled())
	assert.Equal(t, 20, GetFusionMaxKeysPerUser())
	assert.Equal(t, "max(min_quota, cp + jp + failed * failed_quota)", GetFusionBillingExpr())
	assert.True(t, ShouldFusionChargeFailedCandidates())
	assert.False(t, ShouldFusionUseStreamCandidateBrief())
	assert.Equal(t, 384, GetFusionStreamCandidateMaxTokens())
	assert.True(t, IsFusionResultCacheEnabled())
	assert.Equal(t, 120, GetFusionResultCacheTTLSeconds())
	assert.Equal(t, 65536, GetFusionResultCacheMaxPayloadBytes())
	assert.Equal(t, 3600, GetFusionResponseStateTTLSeconds())
	assert.Equal(t, 1048576, GetFusionResponseStateMaxPayloadBytes())
	assert.Equal(t, []string{"example.com", "*.example.org"}, GetFusionAllowedBaseURLDomains())
	assert.Equal(t, []int{443, 8443}, GetFusionAllowedBaseURLPorts())
	assert.Equal(t, []string{"fusion-basic", "fusion-pro"}, GetFusionAllowedTokenGroups())
	assert.Equal(t, "candidate admin prompt", GetFusionCandidateSystemPrompt())
	assert.Equal(t, "judge admin prompt", GetFusionJudgeSystemPrompt())
	assert.True(t, IsFusionTokenGroup("fusion-basic"))
	assert.False(t, IsFusionTokenGroup("default"))
}

func TestValidateFusionBillingExprAcceptsDefault(t *testing.T) {
	require.NoError(t, ValidateFusionBillingExpr(DefaultBillingExpression))
}

func TestValidateFusionBillingExprRejectsEmpty(t *testing.T) {
	err := ValidateFusionBillingExpr(" ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestValidateFusionBillingExprRejectsTieredBillingVariables(t *testing.T) {
	err := ValidateFusionBillingExpr("p + c")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compile failed")
}

func TestValidateFusionSystemPromptRejectsOversizedPrompt(t *testing.T) {
	err := ValidateFusionSystemPrompt(strings.Repeat("a", FusionSystemPromptMaxBytes+1), "fusion prompt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no more than")
}
