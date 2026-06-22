package model

const (
	FusionProtocolOpenAIResponses      = UpstreamProtocolOpenAIResponses
	FusionProtocolOpenAIChatCompatible = UpstreamProtocolOpenAIChatCompatible
	FusionProtocolAnthropicMessages    = UpstreamProtocolAnthropicMessages

	FusionAuthTypeBearer = UpstreamProtocolAuthTypeBearer
	FusionAuthTypeHeader = UpstreamProtocolAuthTypeHeader
	FusionAuthTypeQuery  = UpstreamProtocolAuthTypeQuery
	FusionAuthTypeNone   = UpstreamProtocolAuthTypeNone

	FusionUpstreamConfigMaxBytes = UpstreamProtocolConfigMaxBytes
)

type FusionUpstreamConfig = UpstreamProtocolConfig
type FusionUpstreamDetectRule = UpstreamProtocolDetectRule
type FusionUpstreamTemplate = UpstreamProtocolTemplate

func DefaultFusionUpstreamTemplate() *FusionUpstreamTemplate {
	return DefaultUpstreamProtocolTemplate()
}

func DefaultFusionUpstreamTemplates() []*FusionUpstreamTemplate {
	return filterFusionExecutionTemplates(DefaultUpstreamProtocolTemplates())
}

func IsSupportedFusionProtocol(protocol string) bool {
	return IsSupportedFusionExecutionProtocol(protocol)
}

func NormalizeFusionUpstreamConfigJSON(value string) (string, error) {
	return NormalizeUpstreamProtocolConfigJSON(value)
}

func ParseFusionUpstreamConfigJSON(value string) (FusionUpstreamConfig, error) {
	return ParseUpstreamProtocolConfigJSON(value)
}

func EnsureDefaultFusionUpstreamTemplate() (*FusionUpstreamTemplate, error) {
	return EnsureDefaultUpstreamProtocolTemplate()
}

func GetDefaultEnabledFusionUpstreamTemplate() (*FusionUpstreamTemplate, error) {
	return GetDefaultEnabledUpstreamProtocolTemplate()
}

func GetFusionUpstreamTemplateByID(id int) (*FusionUpstreamTemplate, error) {
	return GetUpstreamProtocolTemplateByID(id)
}

func GetEnabledFusionUpstreamTemplateByID(id int) (*FusionUpstreamTemplate, error) {
	return GetEnabledUpstreamProtocolTemplateByID(id)
}

func GetFusionTemplateForKey(key *FusionAPIKey) (*FusionUpstreamTemplate, error) {
	return GetUpstreamProtocolTemplateForFusionKey(key)
}

func ListFusionUpstreamTemplates(includeDisabled bool) ([]*FusionUpstreamTemplate, error) {
	templates, err := ListUpstreamProtocolTemplates(includeDisabled)
	if err != nil {
		return nil, err
	}
	return filterFusionExecutionTemplates(templates), nil
}

func DeleteFusionUpstreamTemplateByID(id int) error {
	return DeleteUpstreamProtocolTemplateByID(id)
}

func filterFusionExecutionTemplates(templates []*UpstreamProtocolTemplate) []*FusionUpstreamTemplate {
	filtered := make([]*FusionUpstreamTemplate, 0, len(templates))
	for _, template := range templates {
		if template != nil && IsSupportedFusionExecutionProtocol(template.Protocol) {
			filtered = append(filtered, template)
		}
	}
	return filtered
}
