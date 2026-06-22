package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"gorm.io/gorm"
)

type FusionResponseState struct {
	Id               int            `json:"id"`
	ResponseID       string         `json:"response_id" gorm:"type:varchar(96);uniqueIndex;index:idx_fusion_response_state_scope"`
	UserID           int            `json:"user_id" gorm:"index:idx_fusion_response_state_scope"`
	TokenID          int            `json:"token_id" gorm:"index:idx_fusion_response_state_scope"`
	ModelAlias       string         `json:"model_alias" gorm:"type:varchar(80);index"`
	ParentResponseID string         `json:"parent_response_id" gorm:"type:varchar(96);index"`
	StateCiphertext  string         `json:"-" gorm:"type:text"`
	ExpiresAt        int64          `json:"expires_at" gorm:"index"`
	CreatedAt        int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

type FusionResponseStatePayload struct {
	Messages       []dto.Message         `json:"messages"`
	Outputs        []dto.ResponsesOutput `json:"outputs"`
	CallIDByItemID map[string]string     `json:"call_id_by_item_id"`
}

func (state *FusionResponseState) Normalize() {
	state.ResponseID = strings.TrimSpace(state.ResponseID)
	state.ModelAlias = strings.TrimSpace(state.ModelAlias)
	state.ParentResponseID = strings.TrimSpace(state.ParentResponseID)
}

func (state *FusionResponseState) SetStatePayload(payload FusionResponseStatePayload) error {
	data, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	encrypted, err := common.EncryptSecret(string(data))
	if err != nil {
		return err
	}
	state.StateCiphertext = encrypted
	return nil
}

func (state *FusionResponseState) GetStatePayload() (FusionResponseStatePayload, error) {
	if strings.TrimSpace(state.StateCiphertext) == "" {
		return FusionResponseStatePayload{}, errors.New("fusion response state is empty")
	}
	plain, err := common.DecryptSecret(state.StateCiphertext)
	if err != nil {
		return FusionResponseStatePayload{}, err
	}
	payload := FusionResponseStatePayload{}
	if err := common.UnmarshalJsonStr(plain, &payload); err != nil {
		return FusionResponseStatePayload{}, err
	}
	if payload.CallIDByItemID == nil {
		payload.CallIDByItemID = map[string]string{}
	}
	return payload, nil
}

func (state *FusionResponseState) Insert() error {
	state.Normalize()
	if state.ResponseID == "" {
		return errors.New("response id is required")
	}
	if state.UserID == 0 || state.TokenID == 0 {
		return errors.New("user id and token id are required")
	}
	if state.ExpiresAt <= 0 {
		return errors.New("expires_at is required")
	}
	return DB.Create(state).Error
}

func GetFusionResponseState(responseID string, userID int, tokenID int) (*FusionResponseState, error) {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" || userID == 0 || tokenID == 0 {
		return nil, errors.New("response id, user id, and token id are required")
	}
	state := &FusionResponseState{}
	err := DB.Where(
		"response_id = ? AND user_id = ? AND token_id = ? AND expires_at >= ?",
		responseID,
		userID,
		tokenID,
		time.Now().Unix(),
	).First(state).Error
	if err != nil {
		return nil, err
	}
	return state, nil
}

func DeleteExpiredFusionResponseStates(now int64) (int64, error) {
	result := DB.Where("expires_at < ?", now).Delete(&FusionResponseState{})
	return result.RowsAffected, result.Error
}
