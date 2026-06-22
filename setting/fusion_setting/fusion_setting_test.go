package fusion_setting

import (
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
	assert.Equal(t, 20000, GetFusionMaxCandidateOutputChars())
	assert.True(t, ShouldFusionUseStreamCandidateBrief())
	assert.Equal(t, 1024, GetFusionStreamCandidateMaxTokens())
	assert.Equal(t, 86400, GetFusionResponseStateTTLSeconds())
	assert.Equal(t, 2097152, GetFusionResponseStateMaxPayloadBytes())
	assert.False(t, IsFusionPrivateBaseURLAllowed())
	assert.Equal(t, []string{}, GetFusionAllowedBaseURLDomains())
	assert.Equal(t, []int{443}, GetFusionAllowedBaseURLPorts())
	assert.Equal(t, []string{}, GetFusionAllowedTokenGroups())
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
		"fusion_setting.response_state_ttl_seconds":       "3600",
		"fusion_setting.response_state_max_payload_bytes": "1048576",
		"fusion_setting.allowed_base_url_domains":         `["example.com","*.example.org"]`,
		"fusion_setting.allowed_base_url_ports":           `[443,8443]`,
		"fusion_setting.allowed_token_groups":             `["fusion-basic","fusion-pro"]`,
	})

	require.NoError(t, err)
	assert.True(t, IsFusionEnabled())
	assert.Equal(t, 20, GetFusionMaxKeysPerUser())
	assert.Equal(t, "max(min_quota, cp + jp + failed * failed_quota)", GetFusionBillingExpr())
	assert.True(t, ShouldFusionChargeFailedCandidates())
	assert.False(t, ShouldFusionUseStreamCandidateBrief())
	assert.Equal(t, 384, GetFusionStreamCandidateMaxTokens())
	assert.Equal(t, 3600, GetFusionResponseStateTTLSeconds())
	assert.Equal(t, 1048576, GetFusionResponseStateMaxPayloadBytes())
	assert.Equal(t, []string{"example.com", "*.example.org"}, GetFusionAllowedBaseURLDomains())
	assert.Equal(t, []int{443, 8443}, GetFusionAllowedBaseURLPorts())
	assert.Equal(t, []string{"fusion-basic", "fusion-pro"}, GetFusionAllowedTokenGroups())
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
