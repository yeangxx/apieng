package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	UpstreamProtocolOpenAIResponses       = "openai_responses"
	UpstreamProtocolOpenAIChatCompatible  = "openai_chat_compatible"
	UpstreamProtocolOpenAIImageGeneration = "openai_image_generation"
	UpstreamProtocolAnthropicMessages     = "anthropic_messages"
	UpstreamProtocolGeminiGenerateContent = "gemini_generate_content"

	UpstreamProtocolAuthTypeBearer = "bearer"
	UpstreamProtocolAuthTypeHeader = "header"
	UpstreamProtocolAuthTypeQuery  = "query"
	UpstreamProtocolAuthTypeNone   = "none"

	UpstreamProtocolConverterNone                                         = "none"
	UpstreamProtocolConverterAnthropicMessagesToOpenAIChatCompletions     = "anthropic_messages_to_openai_chat_completions"
	UpstreamProtocolConverterOpenAIChatCompletionsToAnthropicMessages     = "openai_chat_completions_to_anthropic_messages"
	UpstreamProtocolConverterOpenAIChatCompletionsToOpenAIResponses       = "openai_chat_completions_to_openai_responses"
	UpstreamProtocolConverterGeminiGenerateContentToOpenAIChatCompletions = "gemini_generate_content_to_openai_chat_completions"
	UpstreamProtocolConverterOpenAIChatCompletionsToGeminiGenerateContent = "openai_chat_completions_to_gemini_generate_content"

	UpstreamProtocolConfigMaxBytes = 8192
)

const defaultUpstreamProtocolTemplatesJSON = `[
  {
    "name": "OpenAI Responses",
    "provider_label": "OpenAI",
    "protocol": "openai_responses",
    "client_path": "/v1/responses",
    "endpoint_path": "/v1/responses",
    "auth_type": "bearer",
    "auth_header": "Authorization",
    "request_converter": "none",
    "enabled": true,
    "sort": -30
  },
  {
    "name": "OpenAI Chat to Responses",
    "provider_label": "OpenAI",
    "protocol": "openai_responses",
    "client_path": "/v1/chat/completions",
    "endpoint_path": "/v1/responses",
    "auth_type": "bearer",
    "auth_header": "Authorization",
    "request_converter": "openai_chat_completions_to_openai_responses",
    "enabled": true,
    "sort": -10
  },
  {
    "name": "OpenAI Chat Completions",
    "provider_label": "OpenAI Compatible",
    "protocol": "openai_chat_compatible",
    "client_path": "/v1/chat/completions",
    "endpoint_path": "/v1/chat/completions",
    "auth_type": "bearer",
    "auth_header": "Authorization",
    "enabled": true,
    "sort": 0
  },
  {
    "name": "OpenAI Image Generations",
    "provider_label": "OpenAI",
    "protocol": "openai_image_generation",
    "client_path": "/v1/images/generations",
    "endpoint_path": "/v1/images/generations",
    "auth_type": "bearer",
    "auth_header": "Authorization",
    "request_converter": "none",
    "enabled": true,
    "sort": 5
  },
  {
    "name": "Anthropic Messages",
    "provider_label": "Anthropic",
    "protocol": "anthropic_messages",
    "client_path": "/v1/messages",
    "endpoint_path": "/v1/messages",
    "auth_type": "header",
    "auth_header": "x-api-key",
    "default_headers": {"anthropic-version": "2023-06-01"},
    "request_converter": "none",
    "enabled": true,
    "sort": 10
  },
  {
    "name": "OpenAI Chat to Anthropic Messages",
    "provider_label": "Anthropic",
    "protocol": "anthropic_messages",
    "client_path": "/v1/chat/completions",
    "endpoint_path": "/v1/messages",
    "auth_type": "header",
    "auth_header": "x-api-key",
    "default_headers": {"anthropic-version": "2023-06-01"},
    "request_converter": "openai_chat_completions_to_anthropic_messages",
    "enabled": true,
    "sort": 11
  },
  {
    "name": "Gemini GenerateContent",
    "provider_label": "Gemini",
    "protocol": "gemini_generate_content",
    "client_path": "/v1beta/models/{model}:generateContent",
    "endpoint_path": "/v1beta/models/{model}:generateContent",
    "auth_type": "query",
    "auth_query_name": "key",
    "request_converter": "none",
    "enabled": true,
    "sort": 20
  },
  {
    "name": "OpenAI Chat to Gemini GenerateContent",
    "provider_label": "Gemini",
    "protocol": "gemini_generate_content",
    "client_path": "/v1/chat/completions",
    "endpoint_path": "/v1beta/models/{model}:generateContent",
    "auth_type": "query",
    "auth_query_name": "key",
    "request_converter": "openai_chat_completions_to_gemini_generate_content",
    "enabled": true,
    "sort": 21
  }
]`

type UpstreamProtocolConfig struct {
	EndpointPath  string                 `json:"endpoint_path,omitempty"`
	Headers       map[string]string      `json:"headers,omitempty"`
	Query         map[string]string      `json:"query,omitempty"`
	BodyOverrides map[string]interface{} `json:"body_overrides,omitempty"`
}

type UpstreamProtocolDetectRule struct {
	StatusCodes     []int                  `json:"status_codes"`
	ErrorContains   []string               `json:"error_contains"`
	SuggestedConfig UpstreamProtocolConfig `json:"suggested_config"`
	Message         string                 `json:"message"`
}

type UpstreamProtocolTemplateSeed struct {
	Name                 string                       `json:"name"`
	ProviderLabel        string                       `json:"provider_label"`
	Protocol             string                       `json:"protocol"`
	ClientPath           string                       `json:"client_path"`
	EndpointPath         string                       `json:"endpoint_path"`
	AuthType             string                       `json:"auth_type"`
	AuthHeader           string                       `json:"auth_header"`
	AuthQueryName        string                       `json:"auth_query_name"`
	DefaultHeaders       map[string]string            `json:"default_headers"`
	DefaultQuery         map[string]string            `json:"default_query"`
	DefaultBodyOverrides map[string]interface{}       `json:"default_body_overrides"`
	RequestConverter     string                       `json:"request_converter"`
	ResponseConverter    string                       `json:"response_converter"`
	StreamConverter      string                       `json:"stream_converter"`
	DetectRules          []UpstreamProtocolDetectRule `json:"detect_rules"`
	Enabled              bool                         `json:"enabled"`
	Sort                 int                          `json:"sort"`
}

type UpstreamProtocolTemplate struct {
	Id                   int            `json:"id"`
	Name                 string         `json:"name" gorm:"type:varchar(80);index"`
	ProviderLabel        string         `json:"provider_label" gorm:"type:varchar(80);index"`
	Protocol             string         `json:"protocol" gorm:"type:varchar(64);index"`
	ClientPath           string         `json:"client_path" gorm:"type:varchar(255);index"`
	EndpointPath         string         `json:"endpoint_path" gorm:"type:varchar(255)"`
	AuthType             string         `json:"auth_type" gorm:"type:varchar(32)"`
	AuthHeader           string         `json:"auth_header" gorm:"type:varchar(80)"`
	AuthQueryName        string         `json:"auth_query_name" gorm:"type:varchar(80)"`
	DefaultHeaders       string         `json:"default_headers" gorm:"type:text"`
	DefaultQuery         string         `json:"default_query" gorm:"type:text"`
	DefaultBodyOverrides string         `json:"default_body_overrides" gorm:"type:text"`
	RequestConverter     string         `json:"request_converter" gorm:"type:varchar(128)"`
	ResponseConverter    string         `json:"response_converter" gorm:"type:varchar(128)"`
	StreamConverter      string         `json:"stream_converter" gorm:"type:varchar(128)"`
	DetectRules          string         `json:"detect_rules" gorm:"type:text"`
	Enabled              bool           `json:"enabled" gorm:"index"`
	Sort                 int            `json:"sort" gorm:"column:sort_order"`
	CreatedAt            int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt            gorm.DeletedAt `json:"-" gorm:"index"`
}

type UpstreamProtocolConverter struct {
	Id          string `json:"id"`
	Label       string `json:"label"`
	Direction   string `json:"direction"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	Description string `json:"description"`
}

type legacyFusionUpstreamTemplate struct {
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

var upstreamProtocolHeaderForbiddenKeys = map[string]struct{}{
	"authorization": {},
	"api-key":       {},
	"api_key":       {},
	"x-api-key":     {},
	"key":           {},
}

var upstreamProtocolCoreForbiddenKeys = map[string]struct{}{
	"authorization": {},
	"api-key":       {},
	"api_key":       {},
	"base_url":      {},
	"key":           {},
	"input":         {},
	"instructions":  {},
	"messages":      {},
	"model":         {},
	"stream":        {},
}

func (legacyFusionUpstreamTemplate) TableName() string {
	return "fusion_upstream_templates"
}

func ListUpstreamProtocolConverters() []UpstreamProtocolConverter {
	return []UpstreamProtocolConverter{
		{Id: UpstreamProtocolConverterNone, Label: "None", Direction: "any", Source: "same", Target: "same"},
		{Id: UpstreamProtocolConverterAnthropicMessagesToOpenAIChatCompletions, Label: "Anthropic Messages to OpenAI Chat Completions", Direction: "request", Source: "anthropic_messages", Target: "openai_chat_compatible"},
		{Id: UpstreamProtocolConverterOpenAIChatCompletionsToAnthropicMessages, Label: "OpenAI Chat Completions to Anthropic Messages", Direction: "request", Source: "openai_chat_compatible", Target: "anthropic_messages"},
		{Id: UpstreamProtocolConverterOpenAIChatCompletionsToOpenAIResponses, Label: "OpenAI Chat Completions to OpenAI Responses", Direction: "request", Source: "openai_chat_compatible", Target: "openai_responses"},
		{Id: UpstreamProtocolConverterGeminiGenerateContentToOpenAIChatCompletions, Label: "Gemini GenerateContent to OpenAI Chat Completions", Direction: "request", Source: "gemini_generate_content", Target: "openai_chat_compatible"},
		{Id: UpstreamProtocolConverterOpenAIChatCompletionsToGeminiGenerateContent, Label: "OpenAI Chat Completions to Gemini GenerateContent", Direction: "request", Source: "openai_chat_compatible", Target: "gemini_generate_content"},
	}
}

func IsRegisteredUpstreamProtocolConverter(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return true
	}
	for _, converter := range ListUpstreamProtocolConverters() {
		if converter.Id == id {
			return true
		}
	}
	return false
}

func IsSupportedFusionExecutionProtocol(protocol string) bool {
	switch strings.TrimSpace(protocol) {
	case UpstreamProtocolOpenAIResponses, UpstreamProtocolOpenAIChatCompatible, UpstreamProtocolAnthropicMessages:
		return true
	default:
		return false
	}
}

func DefaultUpstreamProtocolTemplate() *UpstreamProtocolTemplate {
	defaults := DefaultUpstreamProtocolTemplates()
	if len(defaults) == 0 {
		return &UpstreamProtocolTemplate{}
	}
	return defaults[0]
}

func DefaultUpstreamProtocolTemplates() []*UpstreamProtocolTemplate {
	var seeds []UpstreamProtocolTemplateSeed
	if err := common.UnmarshalJsonStr(defaultUpstreamProtocolTemplatesJSON, &seeds); err != nil {
		common.SysError("failed to parse upstream protocol template seed: " + err.Error())
		return []*UpstreamProtocolTemplate{}
	}
	templates := make([]*UpstreamProtocolTemplate, 0, len(seeds))
	for _, seed := range seeds {
		template := upstreamProtocolTemplateFromSeed(seed)
		template.Normalize()
		templates = append(templates, template)
	}
	return templates
}

func upstreamProtocolTemplateFromSeed(seed UpstreamProtocolTemplateSeed) *UpstreamProtocolTemplate {
	headers, _ := common.Marshal(seed.DefaultHeaders)
	query, _ := common.Marshal(seed.DefaultQuery)
	bodyOverrides, _ := common.Marshal(seed.DefaultBodyOverrides)
	detectRules, _ := common.Marshal(seed.DetectRules)
	return &UpstreamProtocolTemplate{
		Name:                 seed.Name,
		ProviderLabel:        seed.ProviderLabel,
		Protocol:             seed.Protocol,
		ClientPath:           seed.ClientPath,
		EndpointPath:         seed.EndpointPath,
		AuthType:             seed.AuthType,
		AuthHeader:           seed.AuthHeader,
		AuthQueryName:        seed.AuthQueryName,
		DefaultHeaders:       string(headers),
		DefaultQuery:         string(query),
		DefaultBodyOverrides: string(bodyOverrides),
		RequestConverter:     seed.RequestConverter,
		ResponseConverter:    seed.ResponseConverter,
		StreamConverter:      seed.StreamConverter,
		DetectRules:          string(detectRules),
		Enabled:              seed.Enabled,
		Sort:                 seed.Sort,
	}
}

func (template *UpstreamProtocolTemplate) Normalize() {
	template.Name = strings.TrimSpace(template.Name)
	template.ProviderLabel = strings.TrimSpace(template.ProviderLabel)
	template.Protocol = strings.TrimSpace(template.Protocol)
	template.ClientPath = strings.TrimSpace(template.ClientPath)
	template.EndpointPath = strings.TrimSpace(template.EndpointPath)
	template.AuthType = strings.TrimSpace(template.AuthType)
	template.AuthHeader = strings.TrimSpace(template.AuthHeader)
	template.AuthQueryName = strings.TrimSpace(template.AuthQueryName)
	template.RequestConverter = normalizeUpstreamProtocolConverter(template.RequestConverter)
	template.ResponseConverter = normalizeUpstreamProtocolConverter(template.ResponseConverter)
	template.StreamConverter = normalizeUpstreamProtocolConverter(template.StreamConverter)
	if template.ProviderLabel == "" {
		template.ProviderLabel = template.Name
	}
	if template.Protocol == "" {
		template.Protocol = UpstreamProtocolOpenAIResponses
	}
	if template.EndpointPath == "" {
		template.EndpointPath = defaultUpstreamProtocolEndpointPath(template.Protocol)
	}
	if template.ClientPath == "" {
		template.ClientPath = template.EndpointPath
	}
	if template.AuthType == "" {
		template.AuthType = UpstreamProtocolAuthTypeBearer
	}
	if template.AuthHeader == "" && (template.AuthType == UpstreamProtocolAuthTypeBearer || template.AuthType == UpstreamProtocolAuthTypeHeader) {
		template.AuthHeader = "Authorization"
	}
	template.DefaultHeaders = normalizeUpstreamProtocolJSONObjectText(template.DefaultHeaders)
	template.DefaultQuery = normalizeUpstreamProtocolJSONObjectText(template.DefaultQuery)
	template.DefaultBodyOverrides = normalizeUpstreamProtocolJSONObjectText(template.DefaultBodyOverrides)
	template.DetectRules = normalizeUpstreamProtocolJSONArrayText(template.DetectRules)
}

func normalizeUpstreamProtocolConverter(converter string) string {
	converter = strings.TrimSpace(converter)
	if converter == "" {
		return UpstreamProtocolConverterNone
	}
	return converter
}

func defaultUpstreamProtocolEndpointPath(protocol string) string {
	switch strings.TrimSpace(protocol) {
	case UpstreamProtocolOpenAIChatCompatible:
		return "/v1/chat/completions"
	case UpstreamProtocolOpenAIImageGeneration:
		return "/v1/images/generations"
	case UpstreamProtocolAnthropicMessages:
		return "/v1/messages"
	case UpstreamProtocolGeminiGenerateContent:
		return "/v1beta/models/{model}:generateContent"
	case UpstreamProtocolOpenAIResponses:
		return "/v1/responses"
	default:
		return "/v1/responses"
	}
}

func (template *UpstreamProtocolTemplate) Validate() error {
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
	if template.Protocol == "" {
		return errors.New("protocol is required")
	}
	if len(template.Protocol) > 64 {
		return errors.New("protocol must be at most 64 characters")
	}
	if !strings.HasPrefix(template.ClientPath, "/") {
		return errors.New("client_path must start with /")
	}
	if !strings.HasPrefix(template.EndpointPath, "/") {
		return errors.New("endpoint_path must start with /")
	}
	if err := validateUpstreamProtocolAuth(template.AuthType, template.AuthHeader, template.AuthQueryName); err != nil {
		return err
	}
	if err := validateUpstreamProtocolConverters(template.RequestConverter, template.ResponseConverter, template.StreamConverter); err != nil {
		return err
	}
	if _, err := normalizeUpstreamProtocolStringMapJSON(template.DefaultHeaders, upstreamProtocolHeaderForbiddenKeys); err != nil {
		return fmt.Errorf("default_headers: %w", err)
	}
	if _, err := normalizeUpstreamProtocolStringMapJSON(template.DefaultQuery, upstreamProtocolCoreForbiddenKeys); err != nil {
		return fmt.Errorf("default_query: %w", err)
	}
	if _, err := normalizeUpstreamProtocolBodyOverridesJSON(template.DefaultBodyOverrides); err != nil {
		return fmt.Errorf("default_body_overrides: %w", err)
	}
	if _, err := template.GetDetectRules(); err != nil {
		return fmt.Errorf("detect_rules: %w", err)
	}
	return nil
}

func validateUpstreamProtocolConverters(converters ...string) error {
	for _, converter := range converters {
		if !IsRegisteredUpstreamProtocolConverter(converter) {
			return fmt.Errorf("unsupported converter: %s", converter)
		}
	}
	return nil
}

func validateUpstreamProtocolAuth(authType string, authHeader string, authQueryName string) error {
	switch authType {
	case UpstreamProtocolAuthTypeBearer, UpstreamProtocolAuthTypeHeader:
		if strings.TrimSpace(authHeader) == "" {
			return errors.New("auth_header is required")
		}
	case UpstreamProtocolAuthTypeQuery:
		if strings.TrimSpace(authQueryName) == "" {
			return errors.New("auth_query_name is required")
		}
	case UpstreamProtocolAuthTypeNone:
		return nil
	default:
		return fmt.Errorf("unsupported auth_type: %s", authType)
	}
	return nil
}

func normalizeUpstreamProtocolJSONObjectText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "null" {
		return "{}"
	}
	return value
}

func normalizeUpstreamProtocolJSONArrayText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "null" {
		return "[]"
	}
	return value
}

func NormalizeUpstreamProtocolConfigJSON(value string) (string, error) {
	config, err := ParseUpstreamProtocolConfigJSON(value)
	if err != nil {
		return "", err
	}
	data, err := common.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ParseUpstreamProtocolConfigJSON(value string) (UpstreamProtocolConfig, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "{}"
	}
	if len(value) > UpstreamProtocolConfigMaxBytes {
		return UpstreamProtocolConfig{}, fmt.Errorf("upstream_config must be at most %d bytes", UpstreamProtocolConfigMaxBytes)
	}
	var config UpstreamProtocolConfig
	if err := common.UnmarshalJsonStr(value, &config); err != nil {
		return UpstreamProtocolConfig{}, err
	}
	config.Normalize()
	if err := config.Validate(); err != nil {
		return UpstreamProtocolConfig{}, err
	}
	return config, nil
}

func (config *UpstreamProtocolConfig) Normalize() {
	config.EndpointPath = strings.TrimSpace(config.EndpointPath)
	config.Headers = normalizeUpstreamProtocolStringMap(config.Headers)
	config.Query = normalizeUpstreamProtocolStringMap(config.Query)
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

func (config UpstreamProtocolConfig) Validate() error {
	if config.EndpointPath != "" && !strings.HasPrefix(config.EndpointPath, "/") {
		return errors.New("endpoint_path must start with /")
	}
	for key := range config.Headers {
		if _, forbidden := upstreamProtocolHeaderForbiddenKeys[strings.ToLower(strings.TrimSpace(key))]; forbidden {
			return fmt.Errorf("headers cannot override %s", key)
		}
	}
	for key := range config.Query {
		if _, forbidden := upstreamProtocolCoreForbiddenKeys[strings.ToLower(strings.TrimSpace(key))]; forbidden {
			return fmt.Errorf("query cannot override %s", key)
		}
	}
	for key := range config.BodyOverrides {
		if _, forbidden := upstreamProtocolCoreForbiddenKeys[strings.ToLower(strings.TrimSpace(key))]; forbidden {
			return fmt.Errorf("body_overrides cannot override %s", key)
		}
	}
	return nil
}

func normalizeUpstreamProtocolStringMap(values map[string]string) map[string]string {
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

func normalizeUpstreamProtocolStringMapJSON(value string, forbidden map[string]struct{}) (string, error) {
	value = normalizeUpstreamProtocolJSONObjectText(value)
	var parsed map[string]string
	if err := common.UnmarshalJsonStr(value, &parsed); err != nil {
		return "", err
	}
	parsed = normalizeUpstreamProtocolStringMap(parsed)
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

func normalizeUpstreamProtocolBodyOverridesJSON(value string) (string, error) {
	value = normalizeUpstreamProtocolJSONObjectText(value)
	var parsed map[string]interface{}
	if err := common.UnmarshalJsonStr(value, &parsed); err != nil {
		return "", err
	}
	config := UpstreamProtocolConfig{BodyOverrides: parsed}
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

func (template *UpstreamProtocolTemplate) GetDefaultHeaders() (map[string]string, error) {
	normalized, err := normalizeUpstreamProtocolStringMapJSON(template.DefaultHeaders, upstreamProtocolHeaderForbiddenKeys)
	if err != nil {
		return nil, err
	}
	var headers map[string]string
	if err := common.UnmarshalJsonStr(normalized, &headers); err != nil {
		return nil, err
	}
	return headers, nil
}

func (template *UpstreamProtocolTemplate) GetDefaultQuery() (map[string]string, error) {
	normalized, err := normalizeUpstreamProtocolStringMapJSON(template.DefaultQuery, upstreamProtocolCoreForbiddenKeys)
	if err != nil {
		return nil, err
	}
	var query map[string]string
	if err := common.UnmarshalJsonStr(normalized, &query); err != nil {
		return nil, err
	}
	return query, nil
}

func (template *UpstreamProtocolTemplate) GetDefaultBodyOverrides() (map[string]interface{}, error) {
	normalized, err := normalizeUpstreamProtocolBodyOverridesJSON(template.DefaultBodyOverrides)
	if err != nil {
		return nil, err
	}
	var overrides map[string]interface{}
	if err := common.UnmarshalJsonStr(normalized, &overrides); err != nil {
		return nil, err
	}
	return overrides, nil
}

func (template *UpstreamProtocolTemplate) GetDetectRules() ([]UpstreamProtocolDetectRule, error) {
	value := normalizeUpstreamProtocolJSONArrayText(template.DetectRules)
	var rules []UpstreamProtocolDetectRule
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

func (template *UpstreamProtocolTemplate) Insert() error {
	if err := template.Validate(); err != nil {
		return err
	}
	return DB.Create(template).Error
}

func (template *UpstreamProtocolTemplate) Update() error {
	if err := template.Validate(); err != nil {
		return err
	}
	return DB.Model(&UpstreamProtocolTemplate{}).
		Where("id = ?", template.Id).
		Updates(map[string]interface{}{
			"name":                   template.Name,
			"provider_label":         template.ProviderLabel,
			"protocol":               template.Protocol,
			"client_path":            template.ClientPath,
			"endpoint_path":          template.EndpointPath,
			"auth_type":              template.AuthType,
			"auth_header":            template.AuthHeader,
			"auth_query_name":        template.AuthQueryName,
			"default_headers":        template.DefaultHeaders,
			"default_query":          template.DefaultQuery,
			"default_body_overrides": template.DefaultBodyOverrides,
			"request_converter":      template.RequestConverter,
			"response_converter":     template.ResponseConverter,
			"stream_converter":       template.StreamConverter,
			"detect_rules":           template.DetectRules,
			"enabled":                template.Enabled,
			"sort_order":             template.Sort,
		}).Error
}

func EnsureDefaultUpstreamProtocolTemplate() (*UpstreamProtocolTemplate, error) {
	if err := migrateLegacyFusionUpstreamTemplates(); err != nil {
		return nil, err
	}
	if err := syncUpstreamProtocolTemplateSequence(); err != nil {
		return nil, err
	}
	if err := reconcileKnownDefaultUpstreamProtocolTemplates(); err != nil {
		return nil, err
	}
	defaults := DefaultUpstreamProtocolTemplates()
	for _, defaultTemplate := range defaults {
		template := &UpstreamProtocolTemplate{}
		err := DB.Where("protocol = ? AND name = ?", defaultTemplate.Protocol, defaultTemplate.Name).
			First(template).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err := defaultTemplate.Insert(); err != nil {
			return nil, err
		}
	}
	return GetDefaultEnabledUpstreamProtocolTemplate()
}

func reconcileKnownDefaultUpstreamProtocolTemplates() error {
	repairs := []struct {
		name       string
		protocol   string
		clientPath string
		endpoint   string
		converter  string
	}{
		{
			name:       "OpenAI Responses",
			protocol:   UpstreamProtocolOpenAIResponses,
			clientPath: "/v1/responses",
			endpoint:   "/v1/responses",
			converter:  UpstreamProtocolConverterOpenAIChatCompletionsToOpenAIResponses,
		},
		{
			name:       "Anthropic Messages",
			protocol:   UpstreamProtocolAnthropicMessages,
			clientPath: "/v1/messages",
			endpoint:   "/v1/messages",
			converter:  UpstreamProtocolConverterOpenAIChatCompletionsToAnthropicMessages,
		},
		{
			name:       "Gemini GenerateContent",
			protocol:   UpstreamProtocolGeminiGenerateContent,
			clientPath: "/v1beta/models/{model}:generateContent",
			endpoint:   "/v1beta/models/{model}:generateContent",
			converter:  UpstreamProtocolConverterOpenAIChatCompletionsToGeminiGenerateContent,
		},
	}
	for _, repair := range repairs {
		if err := DB.Model(&UpstreamProtocolTemplate{}).
			Where("name = ? AND protocol = ? AND client_path = ? AND endpoint_path = ? AND request_converter = ?",
				repair.name,
				repair.protocol,
				repair.clientPath,
				repair.endpoint,
				repair.converter,
			).
			Updates(map[string]interface{}{"request_converter": UpstreamProtocolConverterNone}).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyFusionUpstreamTemplates() error {
	if !DB.Migrator().HasTable("fusion_upstream_templates") {
		return nil
	}
	var legacyTemplates []legacyFusionUpstreamTemplate
	if err := DB.Unscoped().Find(&legacyTemplates).Error; err != nil {
		return err
	}
	for _, legacyTemplate := range legacyTemplates {
		if legacyTemplate.Id <= 0 {
			continue
		}
		var count int64
		if err := DB.Model(&UpstreamProtocolTemplate{}).Where("id = ?", legacyTemplate.Id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		template := upstreamProtocolTemplateFromLegacy(legacyTemplate)
		if err := template.Insert(); err != nil {
			return err
		}
	}
	return nil
}

func syncUpstreamProtocolTemplateSequence() error {
	if !common.UsingPostgreSQL || !DB.Migrator().HasTable(&UpstreamProtocolTemplate{}) {
		return nil
	}
	var sequenceValue int64
	return DB.Raw(`
		SELECT setval(
			pg_get_serial_sequence('upstream_protocol_templates', 'id'),
			GREATEST(COALESCE((SELECT MAX(id) FROM upstream_protocol_templates), 0), 1),
			COALESCE((SELECT MAX(id) FROM upstream_protocol_templates), 0) > 0
		)
	`).Scan(&sequenceValue).Error
}

func upstreamProtocolTemplateFromLegacy(legacyTemplate legacyFusionUpstreamTemplate) *UpstreamProtocolTemplate {
	template := &UpstreamProtocolTemplate{
		Id:                   legacyTemplate.Id,
		Name:                 legacyTemplate.Name,
		ProviderLabel:        legacyTemplate.ProviderLabel,
		Protocol:             legacyTemplate.Protocol,
		ClientPath:           legacyTemplate.EndpointPath,
		EndpointPath:         legacyTemplate.EndpointPath,
		AuthType:             legacyTemplate.AuthType,
		AuthHeader:           legacyTemplate.AuthHeader,
		AuthQueryName:        legacyTemplate.AuthQueryName,
		DefaultHeaders:       legacyTemplate.DefaultHeaders,
		DefaultQuery:         legacyTemplate.DefaultQuery,
		DefaultBodyOverrides: legacyTemplate.DefaultBodyOverrides,
		RequestConverter:     defaultUpstreamProtocolRequestConverter(legacyTemplate.Protocol),
		ResponseConverter:    UpstreamProtocolConverterNone,
		StreamConverter:      UpstreamProtocolConverterNone,
		DetectRules:          legacyTemplate.DetectRules,
		Enabled:              legacyTemplate.Enabled,
		Sort:                 legacyTemplate.Sort,
		CreatedAt:            legacyTemplate.CreatedAt,
		UpdatedAt:            legacyTemplate.UpdatedAt,
		DeletedAt:            legacyTemplate.DeletedAt,
	}
	template.Normalize()
	return template
}

func defaultUpstreamProtocolRequestConverter(protocol string) string {
	return UpstreamProtocolConverterNone
}

func GetDefaultEnabledUpstreamProtocolTemplate() (*UpstreamProtocolTemplate, error) {
	template := &UpstreamProtocolTemplate{}
	err := DB.Where("enabled = ?", true).
		Order("sort_order asc, id asc").
		First(template).Error
	if err != nil {
		return nil, err
	}
	return template, nil
}

func GetUpstreamProtocolTemplateByID(id int) (*UpstreamProtocolTemplate, error) {
	if id <= 0 {
		return nil, errors.New("template_id is required")
	}
	template := &UpstreamProtocolTemplate{}
	err := DB.Where("id = ?", id).First(template).Error
	if err != nil {
		return nil, err
	}
	return template, nil
}

func GetEnabledUpstreamProtocolTemplateByID(id int) (*UpstreamProtocolTemplate, error) {
	template, err := GetUpstreamProtocolTemplateByID(id)
	if err != nil {
		return nil, err
	}
	if !template.Enabled {
		return nil, errors.New("upstream protocol template is disabled")
	}
	return template, nil
}

func GetUpstreamProtocolTemplateForFusionKey(key *FusionAPIKey) (*UpstreamProtocolTemplate, error) {
	if key == nil {
		return nil, errors.New("fusion api key is required")
	}
	if key.TemplateID > 0 {
		return GetEnabledUpstreamProtocolTemplateByID(key.TemplateID)
	}
	template, err := GetDefaultEnabledUpstreamProtocolTemplate()
	if err == nil {
		return template, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return DefaultUpstreamProtocolTemplate(), nil
}

func ListUpstreamProtocolTemplates(includeDisabled bool) ([]*UpstreamProtocolTemplate, error) {
	var templates []*UpstreamProtocolTemplate
	query := DB.Model(&UpstreamProtocolTemplate{})
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}
	err := query.Order("sort_order asc, id asc").Find(&templates).Error
	return templates, err
}

func DeleteUpstreamProtocolTemplateByID(id int) error {
	template, err := GetUpstreamProtocolTemplateByID(id)
	if err != nil {
		return err
	}
	var total int64
	if err := DB.Model(&FusionAPIKey{}).Where("template_id = ?", id).Count(&total).Error; err != nil {
		return err
	}
	if total > 0 {
		return errors.New("upstream protocol template is used by fusion keys")
	}
	if err := DB.Model(&ChannelProtocolBinding{}).Where("template_id = ?", id).Count(&total).Error; err != nil {
		return err
	}
	if total > 0 {
		return errors.New("upstream protocol template is used by channels")
	}
	return DB.Delete(template).Error
}

func UpstreamProtocolPathMatches(configuredPath string, requestPath string) bool {
	configuredPath = strings.TrimSpace(configuredPath)
	requestPath = strings.TrimSpace(requestPath)
	if configuredPath == "" || requestPath == "" {
		return false
	}
	if upstreamProtocolPathTemplateMatches(configuredPath, requestPath) {
		return true
	}
	if strings.Contains(configuredPath, ":generateContent") {
		streamPath := strings.Replace(configuredPath, ":generateContent", ":streamGenerateContent", 1)
		return upstreamProtocolPathTemplateMatches(streamPath, requestPath)
	}
	return false
}

func upstreamProtocolPathTemplateMatches(configuredPath string, requestPath string) bool {
	if !strings.Contains(configuredPath, "{model}") {
		return configuredPath == requestPath
	}
	parts := strings.Split(configuredPath, "{model}")
	if len(parts) != 2 {
		return false
	}
	if !strings.HasPrefix(requestPath, parts[0]) || !strings.HasSuffix(requestPath, parts[1]) {
		return false
	}
	modelName := strings.TrimSuffix(strings.TrimPrefix(requestPath, parts[0]), parts[1])
	return strings.TrimSpace(modelName) != ""
}
