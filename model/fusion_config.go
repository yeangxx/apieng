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

var fusionModelAliasPattern = regexp.MustCompile(`^fusion:[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

type FusionConfig struct {
	Id              int            `json:"id"`
	UserId          int            `json:"user_id" gorm:"index;index:idx_fusion_config_user_alias;index:idx_fusion_config_user_enabled"`
	Name            string         `json:"name" gorm:"type:varchar(80);index"`
	ModelAlias      string         `json:"model_alias" gorm:"type:varchar(80);index:idx_fusion_config_user_alias"`
	Enabled         bool           `json:"enabled" gorm:"index:idx_fusion_config_user_enabled"`
	CandidateKeyIDs string         `json:"candidate_key_ids" gorm:"type:text"`
	CandidateModels string         `json:"candidate_models" gorm:"type:text"`
	JudgeKeyID      int            `json:"judge_key_id" gorm:"index"`
	JudgeModel      string         `json:"judge_model" gorm:"type:varchar(128)"`
	Strategy        string         `json:"strategy" gorm:"type:varchar(32)"`
	TimeoutMS       int            `json:"timeout_ms"`
	MaxParallel     int            `json:"max_parallel"`
	MinSuccesses    int            `json:"min_successes"`
	JudgePrompt     string         `json:"judge_prompt" gorm:"type:text"`
	CreatedAt       int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

func (config *FusionConfig) Normalize() {
	config.Name = strings.TrimSpace(config.Name)
	config.ModelAlias = strings.TrimSpace(config.ModelAlias)
	config.JudgeModel = strings.TrimSpace(config.JudgeModel)
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

	candidateIDs, err := config.GetCandidateKeyIDs()
	if err != nil {
		return err
	}
	if len(candidateIDs) == 0 {
		return errors.New("at least one candidate key is required")
	}
	if config.MinSuccesses < 1 || config.MinSuccesses > len(candidateIDs) {
		return errors.New("min_successes must be between 1 and candidate count")
	}

	candidateModels, err := config.GetCandidateModels()
	if err != nil {
		return err
	}
	seen := make(map[int]struct{}, len(candidateIDs))
	for _, keyID := range candidateIDs {
		if keyID <= 0 {
			return errors.New("candidate key id is invalid")
		}
		if _, ok := seen[keyID]; ok {
			return fmt.Errorf("duplicate candidate key id: %d", keyID)
		}
		seen[keyID] = struct{}{}

		key, err := GetFusionAPIKeyByUserAndId(userId, keyID)
		if err != nil {
			return fmt.Errorf("candidate key %d is not owned by user %d: %w", keyID, userId, err)
		}
		modelName := strings.TrimSpace(candidateModels[strconv.Itoa(keyID)])
		if modelName == "" {
			modelName = key.DefaultModel
		}
		allowed, err := key.IsModelAllowed(modelName)
		if err != nil {
			return err
		}
		if !allowed {
			return fmt.Errorf("candidate model %s is not allowed by key %d", modelName, keyID)
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
			"name":              config.Name,
			"model_alias":       config.ModelAlias,
			"enabled":           config.Enabled,
			"candidate_key_ids": config.CandidateKeyIDs,
			"candidate_models":  config.CandidateModels,
			"judge_key_id":      config.JudgeKeyID,
			"judge_model":       config.JudgeModel,
			"strategy":          config.Strategy,
			"timeout_ms":        config.TimeoutMS,
			"max_parallel":      config.MaxParallel,
			"min_successes":     config.MinSuccesses,
			"judge_prompt":      config.JudgePrompt,
		}).Error
}

func DeleteFusionConfigByUserAndId(userId int, id int) error {
	config, err := GetFusionConfigByUserAndId(userId, id)
	if err != nil {
		return err
	}
	return DB.Delete(config).Error
}
