package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFusionBillingExpressionUsesFusionVariables(t *testing.T) {
	result, err := RunFusionBillingExpr(
		"cp + cc*2 + jp*3 + jc*4 + failed*failed_quota + failed_prompt",
		FusionBillingInput{
			CandidatePromptTokens:     10,
			CandidateCompletionTokens: 20,
			JudgePromptTokens:         30,
			JudgeCompletionTokens:     40,
			FailedCandidates:          2,
			FailedPromptTokens:        7,
			SuccessfulCandidates:      3,
			TotalCandidates:           5,
		},
		FusionBillingPolicy{
			ChargeFailedCandidates: true,
			FailedCandidateQuota:   6,
			GroupRatio:             1,
		},
	)

	require.NoError(t, err)
	assert.Equal(t, 319.0, result.QuotaBeforeGroup)
	assert.Equal(t, 319, result.QuotaAfterGroup)
	assert.Equal(t, 2.0, result.MatchedVars["failed"])
	assert.Equal(t, 7.0, result.MatchedVars["failed_prompt"])
}

func TestFusionBillingExpressionRejectsTieredBillingVariables(t *testing.T) {
	_, err := RunFusionBillingExpr("p + c", FusionBillingInput{}, FusionBillingPolicy{GroupRatio: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compile failed")
}

func TestFusionBillingInvalidExpressionFailsClosed(t *testing.T) {
	_, err := CalculateFusionServiceQuota(FusionBillingInput{}, FusionBillingPolicy{
		Expression: "max(",
		GroupRatio: 1,
	})
	require.Error(t, err)
}

func TestFusionBillingAppliesGroupRatioAfterQuotaExpression(t *testing.T) {
	quota, err := CalculateFusionServiceQuota(FusionBillingInput{}, FusionBillingPolicy{
		Expression: "10",
		GroupRatio: 2.5,
	})

	require.NoError(t, err)
	assert.Equal(t, 25, quota)
}

func TestFusionBillingAppliesMinimumQuota(t *testing.T) {
	result, err := RunFusionBillingExpr("1", FusionBillingInput{}, FusionBillingPolicy{
		MinimumQuota: 5,
		GroupRatio:   2,
	})

	require.NoError(t, err)
	assert.Equal(t, 5.0, result.QuotaBeforeGroup)
	assert.Equal(t, 10, result.QuotaAfterGroup)
}

func TestFusionBillingExpressionCanChargeFailedCandidates(t *testing.T) {
	result, err := RunFusionBillingExpr("failed*failed_quota + failed_prompt", FusionBillingInput{
		FailedCandidates:   3,
		FailedPromptTokens: 11,
	}, FusionBillingPolicy{
		ChargeFailedCandidates: true,
		FailedCandidateQuota:   7,
		GroupRatio:             1,
	})

	require.NoError(t, err)
	assert.Equal(t, 32.0, result.QuotaBeforeGroup)
	assert.Equal(t, 3.0, result.MatchedVars["failed"])
	assert.Equal(t, 11.0, result.MatchedVars["failed_prompt"])
}

func TestFusionBillingExpressionIgnoresFailedCandidatesWhenDisabled(t *testing.T) {
	result, err := RunFusionBillingExpr("failed*failed_quota + failed_prompt", FusionBillingInput{
		FailedCandidates:   3,
		FailedPromptTokens: 11,
	}, FusionBillingPolicy{
		ChargeFailedCandidates: false,
		FailedCandidateQuota:   7,
		GroupRatio:             1,
	})

	require.NoError(t, err)
	assert.Equal(t, 0.0, result.QuotaBeforeGroup)
	assert.Equal(t, 0.0, result.MatchedVars["failed"])
	assert.Equal(t, 0.0, result.MatchedVars["failed_prompt"])
}
