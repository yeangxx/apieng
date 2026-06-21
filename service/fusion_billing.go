package service

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/pkg/billingexpr"

	"github.com/expr-lang/expr"
)

type FusionBillingInput struct {
	CandidatePromptTokens     int
	CandidateCompletionTokens int
	JudgePromptTokens         int
	JudgeCompletionTokens     int
	FailedCandidates          int
	FailedPromptTokens        int
	SuccessfulCandidates      int
	TotalCandidates           int
}

type FusionBillingPolicy struct {
	Expression             string
	MinimumQuota           int
	ChargeFailedCandidates bool
	FailedCandidateQuota   int
	GroupRatio             float64
}

type FusionBillingResult struct {
	QuotaBeforeGroup float64
	QuotaAfterGroup  int
	MatchedVars      map[string]float64
}

func RunFusionBillingExpr(exprStr string, input FusionBillingInput, policy FusionBillingPolicy) (FusionBillingResult, error) {
	exprStr = strings.TrimSpace(exprStr)
	if exprStr == "" {
		return FusionBillingResult{}, fmt.Errorf("fusion billing expression is required")
	}
	if policy.GroupRatio == 0 {
		policy.GroupRatio = 1
	}

	failedCandidates := input.FailedCandidates
	failedPromptTokens := input.FailedPromptTokens
	if !policy.ChargeFailedCandidates {
		failedCandidates = 0
		failedPromptTokens = 0
	}

	vars := map[string]float64{
		"cp":                   float64(input.CandidatePromptTokens),
		"cc":                   float64(input.CandidateCompletionTokens),
		"jp":                   float64(input.JudgePromptTokens),
		"jc":                   float64(input.JudgeCompletionTokens),
		"candidate_prompt":     float64(input.CandidatePromptTokens),
		"candidate_completion": float64(input.CandidateCompletionTokens),
		"judge_prompt":         float64(input.JudgePromptTokens),
		"judge_completion":     float64(input.JudgeCompletionTokens),
		"failed":               float64(failedCandidates),
		"failed_prompt":        float64(failedPromptTokens),
		"failed_quota":         float64(policy.FailedCandidateQuota),
		"success":              float64(input.SuccessfulCandidates),
		"total":                float64(input.TotalCandidates),
		"min_quota":            float64(policy.MinimumQuota),
	}
	env := map[string]interface{}{
		"cp":                   vars["cp"],
		"cc":                   vars["cc"],
		"jp":                   vars["jp"],
		"jc":                   vars["jc"],
		"candidate_prompt":     vars["candidate_prompt"],
		"candidate_completion": vars["candidate_completion"],
		"judge_prompt":         vars["judge_prompt"],
		"judge_completion":     vars["judge_completion"],
		"failed":               vars["failed"],
		"failed_prompt":        vars["failed_prompt"],
		"failed_quota":         vars["failed_quota"],
		"success":              vars["success"],
		"total":                vars["total"],
		"min_quota":            vars["min_quota"],
		"max":                  math.Max,
		"min":                  math.Min,
		"abs":                  math.Abs,
		"ceil":                 math.Ceil,
		"floor":                math.Floor,
	}

	program, err := expr.Compile(exprStr, expr.Env(env), expr.AsFloat64())
	if err != nil {
		return FusionBillingResult{}, fmt.Errorf("fusion billing expression compile failed: %w", err)
	}
	out, err := expr.Run(program, env)
	if err != nil {
		return FusionBillingResult{}, fmt.Errorf("fusion billing expression run failed: %w", err)
	}
	quotaBeforeGroup, ok := out.(float64)
	if !ok {
		return FusionBillingResult{}, fmt.Errorf("fusion billing expression returned %T, want float64", out)
	}
	if math.IsNaN(quotaBeforeGroup) || math.IsInf(quotaBeforeGroup, 0) || quotaBeforeGroup < 0 {
		return FusionBillingResult{}, fmt.Errorf("fusion billing expression returned invalid quota: %v", quotaBeforeGroup)
	}
	if minQuota := float64(policy.MinimumQuota); quotaBeforeGroup < minQuota {
		quotaBeforeGroup = minQuota
	}

	return FusionBillingResult{
		QuotaBeforeGroup: quotaBeforeGroup,
		QuotaAfterGroup:  billingexpr.QuotaRound(quotaBeforeGroup * policy.GroupRatio),
		MatchedVars:      vars,
	}, nil
}

func CalculateFusionServiceQuota(input FusionBillingInput, policy FusionBillingPolicy) (int, error) {
	result, err := RunFusionBillingExpr(policy.Expression, input, policy)
	if err != nil {
		return 0, err
	}
	return result.QuotaAfterGroup, nil
}
