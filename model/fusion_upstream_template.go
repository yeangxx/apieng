package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	FusionProtocolOpenAIChatCompatible = "openai_chat_compatible"

	FusionAuthTypeBearer = "bearer"
	FusionAuthTypeHeader = "header"
	FusionAuthTypeQuery  = "query"
	FusionAuthTypeNone   = "none"

	FusionUpstreamConfigMaxBytes = 8192
)

type FusionUpstreamConfig struct {
	EndpointPath  string                 `json:"endpoint_path,omitempty"`
	Headers       map[string]string      `json:"headers,omitempty"`
	Query         map[string]string      `json:"query,omitempty"`
	BodyOverrides map[string]interface{} `json:"body_overrides,omitempty"`
}

type FusionUpstreamDetectRule struct {
	StatusCodes     []int                `json:"status_codes"`
	ErrorContains   []string             `json:"error_contains"`
	SuggestedConfig FusionUpstreamConfig `json:"suggested_config"`
	Message         string               `json:"message"`
}

type FusionUpstreamTemplate struct {
	Id                   int            `json:"id"`
	Name                 string         `json:"name" gorm:"type:varchar(80);index"`
	ProviderLabel        string         `json:"provider_label" gorm:"type:varchar(80);index"`
	Protocol             string         `json:"protocol" gorm:"type:varchar(64);index"`
	EndpointPath         string         `json:"endpoint_path" gorm:"type:varchar(255)"`
	AuthType             string         `json:"auth_type" gorm:"type:varchar(32)"`
	AuthHeader           string         `json:"auth_header" gorm:"type:varchar(80)"`
	AuthQueryName        string         `json:"auth_query_name" gorm:"type:varchar(80)"`
	DefaultHeaders       string         `json:"default_headers" gorm:"type:text"`
	DefaultQuery         string         `json:"default_query" gorm:"type:text"`
	DefaultBodyOverrides string         `json:"default_body_overrides" gorm:"type:text"`
	DetectRules          string         `json:"detect_rules" gorm:"type:text"`
	Enabled              bool           `json:"enabled" gorm:"index"`
	Sort                 int            `json:"sort" gorm:"column:sort_order"`
	CreatedAt            int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt            gorm.DeletedAt `json:"-" gorm:"index"`
}

var fusionUpstreamHeaderForbiddenKeys = map[string]struct{}{
	"authorization": {},
	"api-key":       {},
	"api_key":       {},
	"x-api-key":     {},
	"key":           {},
}

var fusionUpstreamCoreForbiddenKeys = map[string]struct{}{
	"authorization": {},
	"api-key":       {},
	"api_key":       {},
	"base_url":      {},
	"key":           {},
	"messages":      {},
	"model":         {},
	"stream":        {},
}

func DefaultFusionUpstreamTemplate() *FusionUpstreamTemplate {
	template := &FusionUpstreamTemplate{
		Name:          "OpenAI Compatible",
		ProviderLabel: "OpenAI Compatible",
		Protocol:      FusionProtocolOpenAIChatCompatible,
		EndpointPath:  "/v1/chat/completions",
		AuthType:      FusionAuthTypeBearer,
		AuthHeader:    "Authorization",
		Enabled:       true,
		Sort:          0,
	}
	template.Normalize()
	return template
}

func (template *FusionUpstreamTemplate) Normalize() {
	template.Name = strings.TrimSpace(template.Name)
	template.ProviderLabel = strings.TrimSpace(template.ProviderLabel)
	template.Protocol = strings.TrimSpace(template.Protocol)
	template.EndpointPath = strings.TrimSpace(template.EndpointPath)
	template.AuthType = strings.TrimSpace(template.AuthType)
	template.AuthHeader = strings.TrimSpace(template.AuthHeader)
	template.AuthQueryName = strings.TrimSpace(template.AuthQueryName)
	if template.ProviderLabel == "" {
		template.ProviderLabel = template.Name
	}
	if template.Protocol == "" {
		template.Protocol = FusionProtocolOpenAIChatCompatible
	}
	if template.EndpointPath == "" {
		template.EndpointPath = "/v1/chat/completions"
	}
	if template.AuthType == "" {
		template.AuthType = FusionAuthTypeBearer
	}
	if template.AuthHeader == "" && (template.AuthType == FusionAuthTypeBearer || template.AuthType == FusionAuthTypeHeader) {
		template.AuthHeader = "Authorization"
	}
	template.DefaultHeaders = normalizeFusionJSONObjectText(template.DefaultHeaders)
	template.DefaultQuery = normalizeFusionJSONObjectText(template.DefaultQuery)
	template.DefaultBodyOverrides = normalizeFusionJSONObjectText(template.DefaultBodyOverrides)
	template.DetectRules = normalizeFusionJSONArrayText(template.DetectRules)
}

func (template *FusionUpstreamTemplate) Validate() error {
	template.Normalize()
	if template.Name == "" {
		return errors.New("template name is required")
	}
	if len(template.Name) > 80 {
		return errors.New("template name must be at most 80 characters")
	}
	if template.ProviderLabel == "" {
		return errors.New("provider_label is required")
	}
	if len(template.ProviderLabel) > 80 {
		return errors.New("provider_label must be at most 80 characters")
	}
	if template.Protocol != FusionProtocolOpenAIChatCompatible {
		return fmt.Errorf("unsupported fusion upstream protocol: %s", template.Protocol)
	}
	if !strings.HasPrefix(template.EndpointPath, "/") {
		return errors.New("endpoint_path must start with /")
	}
	if err := validateFusionAuth(template.AuthType, template.AuthHeader, template.AuthQueryName); err != nil {
		return err
	}
	if _, err := normalizeFusionStringMapJSON(template.DefaultHeaders, fusionUpstreamHeaderForbiddenKeys); err != nil {
		return fmt.Errorf("default_headers: %w", err)
	}
	if _, err := normalizeFusionStringMapJSON(template.DefaultQuery, fusionUpstreamCoreForbiddenKeys); err != nil {
		return fmt.Errorf("default_query: %w", err)
	}
	if _, err := normalizeFusionBodyOverridesJSON(template.DefaultBodyOverrides); err != nil {
		return fmt.Errorf("default_body_overrides: %w", err)
	}
	if _, err := template.GetDetectRules(); err != nil {
		return fmt.Errorf("detect_rules: %w", err)
	}
	return nil
}

func validateFusionAuth(authType string, authHeader string, authQueryName string) error {
	switch authType {
	case FusionAuthTypeBearer, FusionAuthTypeHeader:
		if strings.TrimSpace(authHeader) == "" {
			return errors.New("auth_header is required")
		}
	case FusionAuthTypeQuery:
		if strings.TrimSpace(authQueryName) == "" {
			return errors.New("auth_query_name is required")
		}
	case FusionAuthTypeNone:
		return nil
	default:
		return fmt.Errorf("unsupported auth_type: %s", authType)
	}
	return nil
}

func normalizeFusionJSONObjectText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "{}"
	}
	return value
}

func normalizeFusionJSONArrayText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "[]"
	}
	return value
}

func NormalizeFusionUpstreamConfigJSON(value string) (string, error) {
	config, err := ParseFusionUpstreamConfigJSON(value)
	if err != nil {
		return "", err
	}
	data, err := common.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ParseFusionUpstreamConfigJSON(value string) (FusionUpstreamConfig, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "{}"
	}
	if len(value) > FusionUpstreamConfigMaxBytes {
		return FusionUpstreamConfig{}, fmt.Errorf("upstream_config must be at most %d bytes", FusionUpstreamConfigMaxBytes)
	}
	var config FusionUpstreamConfig
	if err := common.UnmarshalJsonStr(value, &config); err != nil {
		return FusionUpstreamConfig{}, err
	}
	config.Normalize()
	if err := config.Validate(); err != nil {
		return FusionUpstreamConfig{}, err
	}
	return config, nil
}

func (config *FusionUpstreamConfig) Normalize() {
	config.EndpointPath = strings.TrimSpace(config.EndpointPath)
	config.Headers = normalizeFusionStringMap(config.Headers)
	config.Query = normalizeFusionStringMap(config.Query)
	if config.BodyOverrides == nil {
		config.BodyOverrides = map[string]interface{}{}
	}
	normalized := make(map[string]interface{}, len(config.BodyOverrides))
	for key, value := range config.BodyOverrides {
		key = strings.TrimSpace(key)
		if key != "" {
			normalized[key] = value
		}
	}
	config.BodyOverrides = normalized
}

func (config FusionUpstreamConfig) Validate() error {
	if config.EndpointPath != "" && !strings.HasPrefix(config.EndpointPath, "/") {
		return errors.New("endpoint_path must start with /")
	}
	for key := range config.Headers {
		if _, forbidden := fusionUpstreamHeaderForbiddenKeys[strings.ToLower(strings.TrimSpace(key))]; forbidden {
			return fmt.Errorf("headers cannot override %s", key)
		}
	}
	for key := range config.Query {
		if _, forbidden := fusionUpstreamCoreForbiddenKeys[strings.ToLower(strings.TrimSpace(key))]; forbidden {
			return fmt.Errorf("query cannot override %s", key)
		}
	}
	for key := range config.BodyOverrides {
		if _, forbidden := fusionUpstreamCoreForbiddenKeys[strings.ToLower(strings.TrimSpace(key))]; forbidden {
			return fmt.Errorf("body_overrides cannot override %s", key)
		}
	}
	return nil
}

func normalizeFusionStringMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	normalized := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			normalized[key] = value
		}
	}
	return normalized
}

func normalizeFusionStringMapJSON(value string, forbidden map[string]struct{}) (string, error) {
	value = normalizeFusionJSONObjectText(value)
	var parsed map[string]string
	if err := common.UnmarshalJsonStr(value, &parsed); err != nil {
		return "", err
	}
	parsed = normalizeFusionStringMap(parsed)
	for key := range parsed {
		if _, ok := forbidden[strings.ToLower(strings.TrimSpace(key))]; ok {
			return "", fmt.Errorf("cannot override %s", key)
		}
	}
	data, err := common.Marshal(parsed)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func normalizeFusionBodyOverridesJSON(value string) (string, error) {
	value = normalizeFusionJSONObjectText(value)
	var parsed map[string]interface{}
	if err := common.UnmarshalJsonStr(value, &parsed); err != nil {
		return "", err
	}
	config := FusionUpstreamConfig{BodyOverrides: parsed}
	config.Normalize()
	if err := config.Validate(); err != nil {
		return "", err
	}
	data, err := common.Marshal(config.BodyOverrides)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (template *FusionUpstreamTemplate) GetDefaultHeaders() (map[string]string, error) {
	normalized, err := normalizeFusionStringMapJSON(template.DefaultHeaders, fusionUpstreamHeaderForbiddenKeys)
	if err != nil {
		return nil, err
	}
	var headers map[string]string
	if err := common.UnmarshalJsonStr(normalized, &headers); err != nil {
		return nil, err
	}
	return headers, nil
}

func (template *FusionUpstreamTemplate) GetDefaultQuery() (map[string]string, error) {
	normalized, err := normalizeFusionStringMapJSON(template.DefaultQuery, fusionUpstreamCoreForbiddenKeys)
	if err != nil {
		return nil, err
	}
	var query map[string]string
	if err := common.UnmarshalJsonStr(normalized, &query); err != nil {
		return nil, err
	}
	return query, nil
}

func (template *FusionUpstreamTemplate) GetDefaultBodyOverrides() (map[string]interface{}, error) {
	normalized, err := normalizeFusionBodyOverridesJSON(template.DefaultBodyOverrides)
	if err != nil {
		return nil, err
	}
	var overrides map[string]interface{}
	if err := common.UnmarshalJsonStr(normalized, &overrides); err != nil {
		return nil, err
	}
	return overrides, nil
}

func (template *FusionUpstreamTemplate) GetDetectRules() ([]FusionUpstreamDetectRule, error) {
	value := normalizeFusionJSONArrayText(template.DetectRules)
	var rules []FusionUpstreamDetectRule
	if err := common.UnmarshalJsonStr(value, &rules); err != nil {
		return nil, err
	}
	for index := range rules {
		rules[index].Message = strings.TrimSpace(rules[index].Message)
		for textIndex := range rules[index].ErrorContains {
			rules[index].ErrorContains[textIndex] = strings.TrimSpace(rules[index].ErrorContains[textIndex])
		}
		rules[index].SuggestedConfig.Normalize()
		if err := rules[index].SuggestedConfig.Validate(); err != nil {
			return nil, err
		}
	}
	return rules, nil
}

func (template *FusionUpstreamTemplate) Insert() error {
	if err := template.Validate(); err != nil {
		return err
	}
	return DB.Create(template).Error
}

func (template *FusionUpstreamTemplate) Update() error {
	if err := template.Validate(); err != nil {
		return err
	}
	return DB.Model(&FusionUpstreamTemplate{}).
		Where("id = ?", template.Id).
		Updates(map[string]interface{}{
			"name":                   template.Name,
			"provider_label":         template.ProviderLabel,
			"protocol":               template.Protocol,
			"endpoint_path":          template.EndpointPath,
			"auth_type":              template.AuthType,
			"auth_header":            template.AuthHeader,
			"auth_query_name":        template.AuthQueryName,
			"default_headers":        template.DefaultHeaders,
			"default_query":          template.DefaultQuery,
			"default_body_overrides": template.DefaultBodyOverrides,
			"detect_rules":           template.DetectRules,
			"enabled":                template.Enabled,
			"sort_order":             template.Sort,
		}).Error
}

func EnsureDefaultFusionUpstreamTemplate() (*FusionUpstreamTemplate, error) {
	template := &FusionUpstreamTemplate{}
	err := DB.Where("protocol = ? AND name = ?", FusionProtocolOpenAIChatCompatible, "OpenAI Compatible").
		First(template).Error
	if err == nil {
		return template, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	template = DefaultFusionUpstreamTemplate()
	if err := template.Insert(); err != nil {
		return nil, err
	}
	return template, nil
}

func GetDefaultEnabledFusionUpstreamTemplate() (*FusionUpstreamTemplate, error) {
	template := &FusionUpstreamTemplate{}
	err := DB.Where("protocol = ? AND enabled = ?", FusionProtocolOpenAIChatCompatible, true).
		Order("sort_order asc, id asc").
		First(template).Error
	if err != nil {
		return nil, err
	}
	return template, nil
}

func GetFusionUpstreamTemplateByID(id int) (*FusionUpstreamTemplate, error) {
	if id <= 0 {
		return nil, errors.New("template_id is required")
	}
	template := &FusionUpstreamTemplate{}
	err := DB.Where("id = ?", id).First(template).Error
	if err != nil {
		return nil, err
	}
	return template, nil
}

func GetEnabledFusionUpstreamTemplateByID(id int) (*FusionUpstreamTemplate, error) {
	template, err := GetFusionUpstreamTemplateByID(id)
	if err != nil {
		return nil, err
	}
	if !template.Enabled {
		return nil, errors.New("fusion upstream template is disabled")
	}
	return template, nil
}

func GetFusionTemplateForKey(key *FusionAPIKey) (*FusionUpstreamTemplate, error) {
	if key == nil {
		return nil, errors.New("fusion api key is required")
	}
	if key.TemplateID > 0 {
		return GetEnabledFusionUpstreamTemplateByID(key.TemplateID)
	}
	template, err := GetDefaultEnabledFusionUpstreamTemplate()
	if err == nil {
		return template, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return DefaultFusionUpstreamTemplate(), nil
}

func ListFusionUpstreamTemplates(includeDisabled bool) ([]*FusionUpstreamTemplate, error) {
	var templates []*FusionUpstreamTemplate
	query := DB.Model(&FusionUpstreamTemplate{})
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}
	err := query.Order("sort_order asc, id asc").Find(&templates).Error
	return templates, err
}

func DeleteFusionUpstreamTemplateByID(id int) error {
	template, err := GetFusionUpstreamTemplateByID(id)
	if err != nil {
		return err
	}
	var total int64
	if err := DB.Model(&FusionAPIKey{}).Where("template_id = ?", id).Count(&total).Error; err != nil {
		return err
	}
	if total > 0 {
		return errors.New("fusion upstream template is used by fusion keys")
	}
	return DB.Delete(template).Error
}
