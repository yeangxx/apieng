package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/fusion_setting"

	"github.com/gin-gonic/gin"
)

const (
	fusionNameMaxLength        = 80
	fusionDefaultModelMaxBytes = 128
)

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
