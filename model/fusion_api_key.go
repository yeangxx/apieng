package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	FusionProviderOpenAICompatible = "openai_compatible"
	FusionKeyStatusEnabled         = 1
	FusionKeyStatusDisabled        = 2
)

type FusionAPIKey struct {
	Id               int            `json:"id"`
	UserId           int            `json:"user_id" gorm:"index;index:idx_fusion_key_user_status;index:idx_fusion_key_user_fingerprint"`
	Name             string         `json:"name" gorm:"type:varchar(80);index"`
	Provider         string         `json:"provider" gorm:"type:varchar(32)"`
	BaseURL          string         `json:"base_url" gorm:"type:varchar(255)"`
	DefaultModel     string         `json:"default_model" gorm:"type:varchar(128)"`
	Models           string         `json:"models" gorm:"type:text"`
	APIKeyCiphertext string         `json:"-" gorm:"type:text;column:api_key_ciphertext"`
	APIKeyHint       string         `json:"api_key_hint" gorm:"type:varchar(32)"`
	KeyFingerprint   string         `json:"-" gorm:"type:varchar(128);index:idx_fusion_key_user_fingerprint"`
	Status           int            `json:"status" gorm:"default:1;index:idx_fusion_key_user_status"`
	LastTestTime     int64          `json:"last_test_time" gorm:"bigint"`
	LastError        string         `json:"last_error" gorm:"type:varchar(512)"`
	CreatedAt        int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

func (key *FusionAPIKey) Normalize() {
	key.Name = strings.TrimSpace(key.Name)
	key.Provider = strings.TrimSpace(key.Provider)
	if key.Provider == "" {
		key.Provider = FusionProviderOpenAICompatible
	}
	key.BaseURL = strings.TrimSpace(key.BaseURL)
	key.DefaultModel = strings.TrimSpace(key.DefaultModel)
	if key.Status == 0 {
		key.Status = FusionKeyStatusEnabled
	}
}

func (key *FusionAPIKey) SetPlainAPIKey(plain string) error {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return errors.New("api key is empty")
	}
	encrypted, err := common.EncryptSecret(plain)
	if err != nil {
		return err
	}
	key.APIKeyCiphertext = encrypted
	key.APIKeyHint = common.MaskSecret(plain)
	key.KeyFingerprint = common.FingerprintSecret(plain)
	return nil
}

func (key *FusionAPIKey) DecryptAPIKey() (string, error) {
	return common.DecryptSecret(key.APIKeyCiphertext)
}

func (key *FusionAPIKey) SetModels(models []string) error {
	normalized := make([]string, 0, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName != "" {
			normalized = append(normalized, modelName)
		}
	}
	data, err := common.Marshal(normalized)
	if err != nil {
		return err
	}
	key.Models = string(data)
	return nil
}

func (key *FusionAPIKey) GetModels() ([]string, error) {
	if strings.TrimSpace(key.Models) == "" {
		return []string{}, nil
	}
	var models []string
	if err := common.UnmarshalJsonStr(key.Models, &models); err != nil {
		return nil, err
	}
	return models, nil
}

func (key *FusionAPIKey) IsModelAllowed(modelName string) (bool, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false, nil
	}
	models, err := key.GetModels()
	if err != nil {
		return false, err
	}
	if len(models) == 0 {
		return true, nil
	}
	for _, allowed := range models {
		if strings.TrimSpace(allowed) == modelName {
			return true, nil
		}
	}
	return false, nil
}

func GetFusionAPIKeysByUserId(userId int) ([]*FusionAPIKey, error) {
	var keys []*FusionAPIKey
	err := DB.Where("user_id = ?", userId).Order("id desc").Find(&keys).Error
	return keys, err
}

func GetFusionAPIKeyByUserAndId(userId int, id int) (*FusionAPIKey, error) {
	if userId == 0 || id == 0 {
		return nil, errors.New("user id and key id are required")
	}
	key := &FusionAPIKey{}
	err := DB.Where("user_id = ? AND id = ?", userId, id).First(key).Error
	if err != nil {
		return nil, err
	}
	return key, nil
}

func CountFusionAPIKeysByUserId(userId int) (int64, error) {
	var total int64
	err := DB.Model(&FusionAPIKey{}).Where("user_id = ?", userId).Count(&total).Error
	return total, err
}
