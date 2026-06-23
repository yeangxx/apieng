package model

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const FusionStrategySynthesize = "synthesize"
const FusionRoutingModeAlways = "always_fusion"
const FusionRoutingModeAutoSimple = "auto_simple"
const FusionQualityModeOff = "off"
const FusionQualityModeRanked = "ranked"
const FusionQualityModeGuarded = "guarded"
const FusionCandidateSamplingModeConfigured = "configured"
const FusionCandidateSamplingModeSelfSample = "self_sample"

const FusionDefaultQualityThreshold = 0.65
const FusionDefaultRankerTopK = 3

var fusionModelAliasPattern = regexp.MustCompile(`^fusion:[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

type FusionConfig struct {
	Id                    int            `json:"id"`
	UserId                int            `json:"user_id" gorm:"index;index:idx_fusion_config_user_alias;index:idx_fusion_config_user_enabled"`
	Name                  string         `json:"name" gorm:"type:varchar(80);index"`
	ModelAlias            string         `json:"model_alias" gorm:"type:varchar(80);index:idx_fusion_config_user_alias"`
	Enabled               bool           `json:"enabled" gorm:"index:idx_fusion_config_user_enabled"`
	Candidates            string         `json:"candidates" gorm:"type:text"`
	CandidateKeyIDs       string         `json:"candidate_key_ids" gorm:"type:text"`
	CandidateModels       string         `json:"candidate_models" gorm:"type:text"`
	JudgeKeyID            int            `json:"judge_key_id" gorm:"index"`
	JudgeModel            string         `json:"judge_model" gorm:"type:varchar(128)"`
	RoutingMode           string         `json:"routing_mode" gorm:"type:varchar(32)"`
	DirectKeyID           int            `json:"direct_key_id" gorm:"index"`
	DirectModel           string         `json:"direct_model" gorm:"type:varchar(128)"`
	QualityMode           string         `json:"quality_mode" gorm:"type:varchar(32)"`
	RankerKeyID           int            `json:"ranker_key_id" gorm:"index"`
	RankerModel           string         `json:"ranker_model" gorm:"type:varchar(128)"`
	EscalationKeyID       int            `json:"escalation_key_id" gorm:"index"`
	EscalationModel       string         `json:"escalation_model" gorm:"type:varchar(128)"`
	QualityThreshold      float64        `json:"quality_threshold"`
	RankerTopK            int            `json:"ranker_top_k"`
	CandidateSamplingMode string         `json:"candidate_sampling_mode" gorm:"type:varchar(32)"`
	Strategy              string         `json:"strategy" gorm:"type:varchar(32)"`
	TimeoutMS             int            `json:"timeout_ms"`
	MaxParallel           int            `json:"max_parallel"`
	MinSuccesses          int            `json:"min_successes"`
	JudgePrompt           string         `json:"judge_prompt" gorm:"type:text"`
	CreatedAt             int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt             gorm.DeletedAt `json:"-" gorm:"index"`
}

type FusionCandidate struct {
	KeyID int    `json:"key_id"`
	Model string `json:"model"`
}

func (config *FusionConfig) Normalize() {
	config.Name = strings.TrimSpace(config.Name)
	config.ModelAlias = strings.TrimSpace(config.ModelAlias)
	config.JudgeModel = strings.TrimSpace(config.JudgeModel)
	config.RoutingMode = strings.TrimSpace(config.RoutingMode)
	if config.RoutingMode == "" {
		config.RoutingMode = FusionRoutingModeAlways
	}
	config.DirectModel = strings.TrimSpace(config.DirectModel)
	config.QualityMode = strings.TrimSpace(config.QualityMode)
	if config.QualityMode == "" {
		config.QualityMode = FusionQualityModeOff
	}
	config.RankerModel = strings.TrimSpace(config.RankerModel)
	config.EscalationModel = strings.TrimSpace(config.EscalationModel)
	if config.QualityThreshold <= 0 {
		config.QualityThreshold = FusionDefaultQualityThreshold
	}
	if config.RankerTopK <= 0 {
		config.RankerTopK = FusionDefaultRankerTopK
	}
	config.CandidateSamplingMode = strings.TrimSpace(config.CandidateSamplingMode)
	if config.CandidateSamplingMode == "" {
		config.CandidateSamplingMode = FusionCandidateSamplingModeConfigured
	}
	config.Strategy = strings.TrimSpace(config.Strategy)
	if config.Strategy == "" {
		config.Strategy = FusionStrategySynthesize
	}
}

func ValidateFusionModelAlias(alias string) error {
	alias = strings.TrimSpace(alias)
	if !fusionModelAliasPattern.MatchString(alias) {
		return fmt.Errorf("invalid fusion model alias: %s", alias)
	}
	return nil
}

func (config *FusionConfig) SetCandidateKeyIDs(ids []int) error {
	normalized := make([]int, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			normalized = append(normalized, id)
		}
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return err
	}
	config.CandidateKeyIDs = string(data)
	return nil
}

func (config *FusionConfig) GetCandidateKeyIDs() ([]int, error) {
	if strings.TrimSpace(config.CandidateKeyIDs) == "" {
		return []int{}, nil
	}
	var ids []int
	if err := common.UnmarshalJsonStr(config.CandidateKeyIDs, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func (config *FusionConfig) SetCandidateModels(models map[string]string) error {
	normalized := make(map[string]string, len(models))
	for keyId, modelName := range models {
		keyId = strings.TrimSpace(keyId)
		modelName = strings.TrimSpace(modelName)
		if keyId != "" && modelName != "" {
			normalized[keyId] = modelName
		}
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return err
	}
	config.CandidateModels = string(data)
	return nil
}

func (config *FusionConfig) GetCandidateModels() (map[string]string, error) {
	if strings.TrimSpace(config.CandidateModels) == "" {
		return map[string]string{}, nil
	}
	models := map[string]string{}
	if err := common.UnmarshalJsonStr(config.CandidateModels, &models); err != nil {
		return nil, err
	}
	return models, nil
}

func (config *FusionConfig) SetCandidates(candidates []FusionCandidate) error {
	normalized := make([]FusionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		candidate.Model = strings.TrimSpace(candidate.Model)
		if candidate.KeyID > 0 && candidate.Model != "" {
			normalized = append(normalized, candidate)
		}
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return err
	}
	config.Candidates = string(data)
	return nil
}

func (config *FusionConfig) GetCandidates() ([]FusionCandidate, error) {
	if strings.TrimSpace(config.Candidates) != "" {
		var candidates []FusionCandidate
		if err := common.UnmarshalJsonStr(config.Candidates, &candidates); err != nil {
			return nil, err
		}
		normalized := make([]FusionCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			candidate.Model = strings.TrimSpace(candidate.Model)
			if candidate.KeyID > 0 && candidate.Model != "" {
				normalized = append(normalized, candidate)
			}
		}
		if len(normalized) > 0 {
			return normalized, nil
		}
	}

	candidateIDs, err := config.GetCandidateKeyIDs()
	if err != nil {
		return nil, err
	}
	candidateModels, err := config.GetCandidateModels()
	if err != nil {
		return nil, err
	}
	candidates := make([]FusionCandidate, 0, len(candidateIDs))
	for _, keyID := range candidateIDs {
		candidates = append(candidates, FusionCandidate{
			KeyID: keyID,
			Model: strings.TrimSpace(candidateModels[strconv.Itoa(keyID)]),
		})
	}
	return candidates, nil
}

func (config *FusionConfig) SyncLegacyCandidateFields(candidates []FusionCandidate) error {
	ids := make([]int, 0, len(candidates))
	models := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		if candidate.KeyID <= 0 {
			continue
		}
		ids = append(ids, candidate.KeyID)
		models[strconv.Itoa(candidate.KeyID)] = strings.TrimSpace(candidate.Model)
	}
	if err := config.SetCandidateKeyIDs(ids); err != nil {
		return err
	}
	return config.SetCandidateModels(models)
}

func ValidateFusionConfigKeyOwnership(userId int, config *FusionConfig) error {
	if userId == 0 || config == nil {
		return errors.New("user id and fusion config are required")
	}
	config.Normalize()
	if err := ValidateFusionModelAlias(config.ModelAlias); err != nil {
		return err
	}
	if config.Strategy != FusionStrategySynthesize {
		return fmt.Errorf("unsupported fusion strategy: %s", config.Strategy)
	}
	if config.RoutingMode != FusionRoutingModeAlways && config.RoutingMode != FusionRoutingModeAutoSimple {
		return fmt.Errorf("unsupported fusion routing mode: %s", config.RoutingMode)
	}
	if config.QualityMode != FusionQualityModeOff && config.QualityMode != FusionQualityModeRanked && config.QualityMode != FusionQualityModeGuarded {
		return fmt.Errorf("unsupported fusion quality mode: %s", config.QualityMode)
	}
	if config.CandidateSamplingMode != FusionCandidateSamplingModeConfigured && config.CandidateSamplingMode != FusionCandidateSamplingModeSelfSample {
		return fmt.Errorf("unsupported fusion candidate sampling mode: %s", config.CandidateSamplingMode)
	}
	if config.QualityThreshold <= 0 || config.QualityThreshold > 1 {
		return errors.New("quality_threshold must be between 0 and 1")
	}
	if config.RankerTopK < 1 {
		return errors.New("ranker_top_k must be at least 1")
	}

	candidates, err := config.GetCandidates()
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return errors.New("at least one candidate model is required")
	}
	if config.MinSuccesses < 1 || config.MinSuccesses > len(candidates) {
		return errors.New("min_successes must be between 1 and candidate count")
	}
	if config.QualityMode != FusionQualityModeOff && config.RankerTopK > len(candidates) {
		return errors.New("ranker_top_k must not exceed candidate count")
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate.KeyID <= 0 {
			return errors.New("candidate key id is invalid")
		}
		candidate.Model = strings.TrimSpace(candidate.Model)

		key, err := GetFusionAPIKeyByUserAndId(userId, candidate.KeyID)
		if err != nil {
			return fmt.Errorf("candidate key %d is not owned by user %d: %w", candidate.KeyID, userId, err)
		}
		modelName := candidate.Model
		if modelName == "" {
			modelName = key.DefaultModel
		}
		if modelName == "" {
			return errors.New("candidate model is required")
		}
		identity := fmt.Sprintf("%d:%s", candidate.KeyID, modelName)
		if _, ok := seen[identity]; ok && config.CandidateSamplingMode != FusionCandidateSamplingModeSelfSample {
			return fmt.Errorf("duplicate candidate model: key %d model %s", candidate.KeyID, modelName)
		}
		seen[identity] = struct{}{}
		allowed, err := key.IsModelAllowed(modelName)
		if err != nil {
			return err
		}
		if !allowed {
			return fmt.Errorf("candidate model %s is not allowed by key %d", modelName, candidate.KeyID)
		}
	}

	judgeKey, err := GetFusionAPIKeyByUserAndId(userId, config.JudgeKeyID)
	if err != nil {
		return fmt.Errorf("judge key %d is not owned by user %d: %w", config.JudgeKeyID, userId, err)
	}
	allowed, err := judgeKey.IsModelAllowed(config.JudgeModel)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("judge model %s is not allowed by key %d", config.JudgeModel, config.JudgeKeyID)
	}

	if err := validateFusionOptionalQualityModel(userId, "ranker", config.RankerKeyID, config.RankerModel, config.JudgeKeyID, config.JudgeModel, config.QualityMode == FusionQualityModeRanked || config.QualityMode == FusionQualityModeGuarded); err != nil {
		return err
	}
	if err := validateFusionOptionalQualityModel(userId, "escalation", config.EscalationKeyID, config.EscalationModel, config.JudgeKeyID, config.JudgeModel, config.QualityMode == FusionQualityModeGuarded); err != nil {
		return err
	}

	directKeyID := config.DirectKeyID
	directModel := strings.TrimSpace(config.DirectModel)
	if directKeyID == 0 && directModel == "" {
		return nil
	}
	if directKeyID == 0 {
		directKeyID = config.JudgeKeyID
	}
	directKey, err := GetFusionAPIKeyByUserAndId(userId, directKeyID)
	if err != nil {
		return fmt.Errorf("direct key %d is not owned by user %d: %w", directKeyID, userId, err)
	}
	if directModel == "" {
		directModel = directKey.DefaultModel
	}
	if directModel == "" {
		return errors.New("direct model is required")
	}
	allowed, err = directKey.IsModelAllowed(directModel)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("direct model %s is not allowed by key %d", directModel, directKeyID)
	}
	return nil
}

func validateFusionOptionalQualityModel(userId int, role string, keyID int, modelName string, fallbackKeyID int, fallbackModel string, required bool) error {
	modelName = strings.TrimSpace(modelName)
	if keyID == 0 && modelName == "" && !required {
		return nil
	}
	if keyID == 0 {
		keyID = fallbackKeyID
	}
	key, err := GetFusionAPIKeyByUserAndId(userId, keyID)
	if err != nil {
		return fmt.Errorf("%s key %d is not owned by user %d: %w", role, keyID, userId, err)
	}
	if modelName == "" {
		modelName = strings.TrimSpace(fallbackModel)
	}
	if modelName == "" {
		modelName = key.DefaultModel
	}
	if modelName == "" {
		return fmt.Errorf("%s model is required", role)
	}
	allowed, err := key.IsModelAllowed(modelName)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("%s model %s is not allowed by key %d", role, modelName, keyID)
	}
	return nil
}

func GetFusionConfigByUserAndAlias(userId int, alias string) (*FusionConfig, error) {
	if userId == 0 || strings.TrimSpace(alias) == "" {
		return nil, errors.New("user id and alias are required")
	}
	config := &FusionConfig{}
	err := DB.Where("user_id = ? AND model_alias = ?", userId, strings.TrimSpace(alias)).First(config).Error
	if err != nil {
		return nil, err
	}
	return config, nil
}

func GetFusionConfigByUserAndId(userId int, id int) (*FusionConfig, error) {
	if userId == 0 || id == 0 {
		return nil, errors.New("user id and config id are required")
	}
	config := &FusionConfig{}
	err := DB.Where("user_id = ? AND id = ?", userId, id).First(config).Error
	if err != nil {
		return nil, err
	}
	return config, nil
}

func GetFusionConfigsByUserId(userId int) ([]*FusionConfig, error) {
	var configs []*FusionConfig
	err := DB.Where("user_id = ?", userId).Order("id desc").Find(&configs).Error
	return configs, err
}

func CountFusionConfigsByUserId(userId int) (int64, error) {
	var total int64
	err := DB.Model(&FusionConfig{}).Where("user_id = ?", userId).Count(&total).Error
	return total, err
}

func IsFusionConfigAliasDuplicated(userId int, id int, alias string) (bool, error) {
	alias = strings.TrimSpace(alias)
	if userId == 0 || alias == "" {
		return false, nil
	}
	var total int64
	err := DB.Model(&FusionConfig{}).
		Where("user_id = ? AND model_alias = ? AND id <> ?", userId, alias, id).
		Count(&total).Error
	return total > 0, err
}

func (config *FusionConfig) Insert() error {
	config.Normalize()
	return DB.Create(config).Error
}

func (config *FusionConfig) Update() error {
	config.Normalize()
	return DB.Model(&FusionConfig{}).
		Where("id = ? AND user_id = ?", config.Id, config.UserId).
		Updates(map[string]interface{}{
			"name":                    config.Name,
			"model_alias":             config.ModelAlias,
			"enabled":                 config.Enabled,
			"candidates":              config.Candidates,
			"candidate_key_ids":       config.CandidateKeyIDs,
			"candidate_models":        config.CandidateModels,
			"judge_key_id":            config.JudgeKeyID,
			"judge_model":             config.JudgeModel,
			"routing_mode":            config.RoutingMode,
			"direct_key_id":           config.DirectKeyID,
			"direct_model":            config.DirectModel,
			"quality_mode":            config.QualityMode,
			"ranker_key_id":           config.RankerKeyID,
			"ranker_model":            config.RankerModel,
			"escalation_key_id":       config.EscalationKeyID,
			"escalation_model":        config.EscalationModel,
			"quality_threshold":       config.QualityThreshold,
			"ranker_top_k":            config.RankerTopK,
			"candidate_sampling_mode": config.CandidateSamplingMode,
			"strategy":                config.Strategy,
			"timeout_ms":              config.TimeoutMS,
			"max_parallel":            config.MaxParallel,
			"min_successes":           config.MinSuccesses,
			"judge_prompt":            config.JudgePrompt,
		}).Error
}

func DeleteFusionConfigByUserAndId(userId int, id int) error {
	config, err := GetFusionConfigByUserAndId(userId, id)
	if err != nil {
		return err
	}
	return DB.Delete(config).Error
}
