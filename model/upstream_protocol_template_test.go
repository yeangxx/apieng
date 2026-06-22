package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUpstreamProtocolModelTestDB(t *testing.T) {
	t.Helper()
	originalDB := DB
	t.Cleanup(func() {
		DB = originalDB
	})

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, DB.AutoMigrate(&UpstreamProtocolTemplate{}, &FusionAPIKey{}, &ChannelProtocolBinding{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestEnsureDefaultUpstreamProtocolTemplatesIdempotentAndPreservesChanges(t *testing.T) {
	setupUpstreamProtocolModelTestDB(t)

	_, err := EnsureDefaultUpstreamProtocolTemplate()
	require.NoError(t, err)

	var initialCount int64
	require.NoError(t, DB.Model(&UpstreamProtocolTemplate{}).Count(&initialCount).Error)
	assert.Equal(t, int64(len(DefaultUpstreamProtocolTemplates())), initialCount)

	var responses UpstreamProtocolTemplate
	require.NoError(t, DB.Where("protocol = ? AND name = ?", UpstreamProtocolOpenAIResponses, "OpenAI Responses").First(&responses).Error)
	assert.Equal(t, UpstreamProtocolConverterNone, responses.RequestConverter)
	require.NoError(t, DB.Model(&responses).Updates(map[string]interface{}{
		"endpoint_path": "/custom/responses",
		"enabled":       false,
	}).Error)

	_, err = EnsureDefaultUpstreamProtocolTemplate()
	require.NoError(t, err)

	var finalCount int64
	require.NoError(t, DB.Model(&UpstreamProtocolTemplate{}).Count(&finalCount).Error)
	assert.Equal(t, initialCount, finalCount)

	var updated UpstreamProtocolTemplate
	require.NoError(t, DB.First(&updated, responses.Id).Error)
	assert.Equal(t, "/custom/responses", updated.EndpointPath)
	assert.False(t, updated.Enabled)
}

func TestUpstreamProtocolTemplateValidationRejectsUnsafeConfig(t *testing.T) {
	template := &UpstreamProtocolTemplate{
		Name:             "Bad Converter",
		ProviderLabel:    "Bad",
		Protocol:         "bad_converter",
		ClientPath:       "/v1/chat/completions",
		EndpointPath:     "/v1/chat/completions",
		AuthType:         UpstreamProtocolAuthTypeBearer,
		AuthHeader:       "Authorization",
		RequestConverter: "unknown_converter",
		Enabled:          true,
	}
	err := template.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported converter")

	template.RequestConverter = UpstreamProtocolConverterNone
	template.DefaultHeaders = `{"Authorization":"Bearer bad"}`
	err = template.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "default_headers")

	template.DefaultHeaders = `{}`
	template.DefaultQuery = `{"api_key":"bad"}`
	err = template.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "default_query")

	template.DefaultQuery = `{}`
	template.DefaultBodyOverrides = `{"model":"bad"}`
	err = template.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "default_body_overrides")
}

func TestChannelProtocolBindingResolvedConfigMergesTemplateThenInstance(t *testing.T) {
	setupUpstreamProtocolModelTestDB(t)
	template := &UpstreamProtocolTemplate{
		Name:                 "Merge Test",
		ProviderLabel:        "Merge",
		Protocol:             "merge_test",
		ClientPath:           "/v1/chat/completions",
		EndpointPath:         "/template/chat",
		AuthType:             UpstreamProtocolAuthTypeHeader,
		AuthHeader:           "x-api-key",
		DefaultHeaders:       `{"X-Template":"template"}`,
		DefaultQuery:         `{"region":"us"}`,
		DefaultBodyOverrides: `{"mode":"template","keep":true}`,
		RequestConverter:     UpstreamProtocolConverterNone,
		Enabled:              true,
	}
	require.NoError(t, template.Insert())

	require.NoError(t, ReplaceChannelProtocolBindings(123, []ChannelProtocolBinding{
		{
			TemplateID: template.Id,
			Enabled:    true,
			UpstreamConfig: `{
				"endpoint_path": "/binding/chat",
				"headers": {"X-Binding":"binding"},
				"query": {"region":"eu"},
				"body_overrides": {"mode":"binding"}
			}`,
		},
	}))

	resolved, matched, err := GetChannelProtocolResolvedConfig(123, "/v1/chat/completions")
	require.NoError(t, err)
	require.True(t, matched)
	assert.Equal(t, "/v1/chat/completions", resolved.ClientPath)
	assert.Equal(t, "/binding/chat", resolved.EndpointPath)
	assert.Equal(t, UpstreamProtocolAuthTypeHeader, resolved.AuthType)
	assert.Equal(t, "x-api-key", resolved.AuthHeader)
	assert.Equal(t, "template", resolved.Headers["X-Template"])
	assert.Equal(t, "binding", resolved.Headers["X-Binding"])
	assert.Equal(t, "eu", resolved.Query["region"])
	assert.Equal(t, "binding", resolved.BodyOverrides["mode"])
	assert.Equal(t, true, resolved.BodyOverrides["keep"])
}

func TestDeleteUpstreamProtocolTemplateRejectsChannelBinding(t *testing.T) {
	setupUpstreamProtocolModelTestDB(t)
	template := &UpstreamProtocolTemplate{
		Name:          "In Use",
		ProviderLabel: "In Use",
		Protocol:      "in_use",
		ClientPath:    "/v1/chat/completions",
		EndpointPath:  "/v1/chat/completions",
		AuthType:      UpstreamProtocolAuthTypeBearer,
		AuthHeader:    "Authorization",
		Enabled:       true,
	}
	require.NoError(t, template.Insert())
	require.NoError(t, ReplaceChannelProtocolBindings(123, []ChannelProtocolBinding{
		{TemplateID: template.Id, Enabled: true, UpstreamConfig: `{}`},
	}))

	err := DeleteUpstreamProtocolTemplateByID(template.Id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "used by channels")
}

func TestUpstreamProtocolPathMatchesGeminiStreamVariant(t *testing.T) {
	assert.True(t, UpstreamProtocolPathMatches(
		"/v1beta/models/{model}:generateContent",
		"/v1beta/models/gemini-2.5-pro:streamGenerateContent",
	))
	assert.False(t, UpstreamProtocolPathMatches(
		"/v1beta/models/{model}:generateContent",
		"/v1beta/models/:streamGenerateContent",
	))
}
