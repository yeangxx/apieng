package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChannelProtocolBinding struct {
	Id             int    `json:"id"`
	ChannelID      int    `json:"channel_id" gorm:"index;uniqueIndex:idx_channel_protocol_binding"`
	TemplateID     int    `json:"template_id" gorm:"index;uniqueIndex:idx_channel_protocol_binding"`
	Enabled        bool   `json:"enabled" gorm:"index"`
	UpstreamConfig string `json:"upstream_config" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type ChannelProtocolClientPath struct {
	ChannelID  int    `gorm:"column:channel_id"`
	ClientPath string `gorm:"column:client_path"`
}

type ChannelProtocolResolvedConfig struct {
	ClientPath        string
	EndpointPath      string
	AuthType          string
	AuthHeader        string
	AuthQueryName     string
	RequestConverter  string
	ResponseConverter string
	StreamConverter   string
	Headers           map[string]string
	Query             map[string]string
	BodyOverrides     map[string]interface{}
}

func (binding *ChannelProtocolBinding) Normalize() {
	if binding.UpstreamConfig == "" {
		binding.UpstreamConfig = "{}"
	}
}

func (binding *ChannelProtocolBinding) Validate() error {
	binding.Normalize()
	if binding.ChannelID <= 0 {
		return errors.New("channel_id is required")
	}
	if binding.TemplateID <= 0 {
		return errors.New("template_id is required")
	}
	if _, err := NormalizeUpstreamProtocolConfigJSON(binding.UpstreamConfig); err != nil {
		return err
	}
	template, err := GetUpstreamProtocolTemplateByID(binding.TemplateID)
	if err != nil {
		return err
	}
	if template.ClientPath == "" {
		return errors.New("template client_path is required")
	}
	return nil
}

func GetChannelProtocolBindings(channelID int) ([]ChannelProtocolBinding, error) {
	var bindings []ChannelProtocolBinding
	err := DB.Where("channel_id = ?", channelID).Order("id asc").Find(&bindings).Error
	return bindings, err
}

func AttachChannelProtocolBindings(channels []*Channel) error {
	if len(channels) == 0 {
		return nil
	}
	channelIDs := make([]int, 0, len(channels))
	channelByID := make(map[int]*Channel, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.Id <= 0 {
			continue
		}
		channelIDs = append(channelIDs, channel.Id)
		channelByID[channel.Id] = channel
	}
	if len(channelIDs) == 0 {
		return nil
	}
	var bindings []ChannelProtocolBinding
	if err := DB.Where("channel_id IN ?", channelIDs).Order("id asc").Find(&bindings).Error; err != nil {
		return err
	}
	for _, binding := range bindings {
		if channel := channelByID[binding.ChannelID]; channel != nil {
			channel.ProtocolBindings = append(channel.ProtocolBindings, binding)
		}
	}
	return nil
}

func ReplaceChannelProtocolBindings(channelID int, bindings []ChannelProtocolBinding) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return replaceChannelProtocolBindingsTx(tx, channelID, bindings)
	})
}

func replaceChannelProtocolBindingsTx(tx *gorm.DB, channelID int, bindings []ChannelProtocolBinding) error {
	if channelID <= 0 {
		return errors.New("channel_id is required")
	}
	if err := tx.Where("channel_id = ?", channelID).Delete(&ChannelProtocolBinding{}).Error; err != nil {
		return err
	}
	if len(bindings) == 0 {
		return nil
	}
	normalized := make([]ChannelProtocolBinding, 0, len(bindings))
	seen := make(map[int]struct{}, len(bindings))
	for _, binding := range bindings {
		binding.Id = 0
		binding.ChannelID = channelID
		binding.Normalize()
		if _, ok := seen[binding.TemplateID]; ok {
			continue
		}
		if err := binding.Validate(); err != nil {
			return err
		}
		seen[binding.TemplateID] = struct{}{}
		normalizedConfig, err := NormalizeUpstreamProtocolConfigJSON(binding.UpstreamConfig)
		if err != nil {
			return err
		}
		binding.UpstreamConfig = normalizedConfig
		normalized = append(normalized, binding)
	}
	if len(normalized) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&normalized).Error
}

func DeleteChannelProtocolBindingsByChannelIDs(tx *gorm.DB, channelIDs []int) error {
	if len(channelIDs) == 0 {
		return nil
	}
	db := DB
	if tx != nil {
		db = tx
	}
	return db.Where("channel_id IN ?", channelIDs).Delete(&ChannelProtocolBinding{}).Error
}

func GetChannelProtocolClientPaths(channelIDs []int) (map[int][]string, error) {
	result := make(map[int][]string)
	if len(channelIDs) == 0 {
		return result, nil
	}
	var rows []ChannelProtocolClientPath
	err := DB.Table("channel_protocol_bindings").
		Select("channel_protocol_bindings.channel_id, upstream_protocol_templates.client_path").
		Joins("JOIN upstream_protocol_templates ON upstream_protocol_templates.id = channel_protocol_bindings.template_id").
		Where("channel_protocol_bindings.enabled = ? AND upstream_protocol_templates.enabled = ? AND channel_protocol_bindings.channel_id IN ?", true, true, channelIDs).
		Scan(&rows).Error
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		result[row.ChannelID] = append(result[row.ChannelID], row.ClientPath)
	}
	return result, nil
}

func GetChannelProtocolResolvedConfig(channelID int, requestPath string) (ChannelProtocolResolvedConfig, bool, error) {
	result := ChannelProtocolResolvedConfig{
		Headers:       map[string]string{},
		Query:         map[string]string{},
		BodyOverrides: map[string]interface{}{},
	}
	requestPath = strings.TrimSpace(requestPath)
	if channelID <= 0 || requestPath == "" {
		return result, false, nil
	}
	bindings, err := GetChannelProtocolBindings(channelID)
	if err != nil {
		return result, false, err
	}
	for _, binding := range bindings {
		if !binding.Enabled {
			continue
		}
		template, err := GetEnabledUpstreamProtocolTemplateByID(binding.TemplateID)
		if err != nil {
			return result, false, err
		}
		if !UpstreamProtocolPathMatches(template.ClientPath, requestPath) {
			continue
		}
		result.ClientPath = template.ClientPath
		result.EndpointPath = template.EndpointPath
		result.AuthType = template.AuthType
		result.AuthHeader = template.AuthHeader
		result.AuthQueryName = template.AuthQueryName
		result.RequestConverter = template.RequestConverter
		result.ResponseConverter = template.ResponseConverter
		result.StreamConverter = template.StreamConverter
		if err := mergeUpstreamProtocolTemplateConfig(&result, template); err != nil {
			return result, false, err
		}
		config, err := ParseUpstreamProtocolConfigJSON(binding.UpstreamConfig)
		if err != nil {
			return result, false, err
		}
		mergeUpstreamProtocolConfig(&result, config)
		return result, true, nil
	}
	return result, false, nil
}

func mergeUpstreamProtocolTemplateConfig(result *ChannelProtocolResolvedConfig, template *UpstreamProtocolTemplate) error {
	headers, err := template.GetDefaultHeaders()
	if err != nil {
		return err
	}
	query, err := template.GetDefaultQuery()
	if err != nil {
		return err
	}
	bodyOverrides, err := template.GetDefaultBodyOverrides()
	if err != nil {
		return err
	}
	for key, value := range headers {
		result.Headers[key] = value
	}
	for key, value := range query {
		result.Query[key] = value
	}
	for key, value := range bodyOverrides {
		result.BodyOverrides[key] = value
	}
	return nil
}

func mergeUpstreamProtocolConfig(result *ChannelProtocolResolvedConfig, config UpstreamProtocolConfig) {
	if config.EndpointPath != "" {
		result.EndpointPath = config.EndpointPath
	}
	for key, value := range config.Headers {
		result.Headers[key] = value
	}
	for key, value := range config.Query {
		result.Query[key] = value
	}
	for key, value := range config.BodyOverrides {
		result.BodyOverrides[key] = value
	}
}

func ChannelProtocolPathsMatch(paths []string, requestPath string) bool {
	for _, path := range paths {
		if UpstreamProtocolPathMatches(path, requestPath) {
			return true
		}
	}
	return false
}
