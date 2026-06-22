package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDistributeMarksFusionChatCompletionWithoutSelectingChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(Distribute())
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		assert.True(t, common.GetContextKeyBool(c, constant.ContextKeyIsFusionRequest))
		assert.Equal(t, "fusion:research", common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
		_, hasChannel := common.GetContextKey(c, constant.ContextKeyChannelId)
		assert.False(t, hasChannel)
		c.Status(http.StatusNoContent)
	})

	body := bytes.NewBufferString(`{"model":"fusion:research","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func setupDistributorProtocolTestDB(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	t.Cleanup(func() {
		model.DB = originalDB
	})
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, model.DB.AutoMigrate(&model.UpstreamProtocolTemplate{}, &model.ChannelProtocolBinding{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestSetupContextForSelectedChannelUsesProtocolTemplateRoute(t *testing.T) {
	setupDistributorProtocolTestDB(t)
	template := &model.UpstreamProtocolTemplate{
		Name:             "OpenAI Chat To Claude",
		ProviderLabel:    "Claude",
		Protocol:         "openai_chat_to_claude",
		ClientPath:       "/v1/chat/completions",
		EndpointPath:     "/v1/messages",
		AuthType:         model.UpstreamProtocolAuthTypeHeader,
		AuthHeader:       "x-api-key",
		DefaultQuery:     `{"version":"2023-06-01"}`,
		RequestConverter: model.UpstreamProtocolConverterOpenAIChatCompletionsToAnthropicMessages,
		Enabled:          true,
	}
	require.NoError(t, template.Insert())
	require.NoError(t, model.ReplaceChannelProtocolBindings(10, []model.ChannelProtocolBinding{
		{
			TemplateID:     template.Id,
			Enabled:        true,
			UpstreamConfig: `{"query":{"beta":"true"}}`,
		},
	}))

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	baseURL := "https://upstream.example"
	channel := &model.Channel{
		Id:      10,
		Type:    constant.ChannelTypeOpenAI,
		Name:    "templated",
		Key:     "sk-test",
		BaseURL: &baseURL,
	}

	setupErr := SetupContextForSelectedChannel(ctx, channel, "gpt-4o-mini")
	require.Nil(t, setupErr)
	assert.Equal(t, constant.APITypeAdvancedCustom, common.GetContextKeyInt(ctx, constant.ContextKeyChannelAPITypeOverride))

	otherSettings, ok := common.GetContextKeyType[dto.ChannelOtherSettings](ctx, constant.ContextKeyChannelOtherSetting)
	require.True(t, ok)
	require.NotNil(t, otherSettings.AdvancedCustom)
	require.Len(t, otherSettings.AdvancedCustom.Routes, 1)
	route := otherSettings.AdvancedCustom.Routes[0]
	assert.Equal(t, "/v1/chat/completions", route.IncomingPath)
	assert.Equal(t, model.UpstreamProtocolConverterOpenAIChatCompletionsToAnthropicMessages, route.Converter)
	require.NotNil(t, route.Auth)
	assert.Equal(t, dto.AdvancedCustomAuthTypeHeader, route.Auth.Type)
	assert.Equal(t, "x-api-key", route.Auth.Name)
	assert.Equal(t, "{api_key}", route.Auth.Value)

	upstreamPath, parseErr := url.Parse(route.UpstreamPath)
	require.NoError(t, parseErr)
	assert.Equal(t, "/v1/messages", upstreamPath.Path)
	assert.Equal(t, "true", upstreamPath.Query().Get("beta"))
	assert.Equal(t, "2023-06-01", upstreamPath.Query().Get("version"))
}

func TestBuildProtocolAdvancedCustomConfigFallsBackForNativeTemplate(t *testing.T) {
	advancedCustomConfig, err := buildProtocolAdvancedCustomConfig(model.ChannelProtocolResolvedConfig{
		ClientPath:       "/v1/messages",
		EndpointPath:     "/v1/messages",
		AuthType:         model.UpstreamProtocolAuthTypeHeader,
		AuthHeader:       "x-api-key",
		RequestConverter: model.UpstreamProtocolConverterOpenAIChatCompletionsToAnthropicMessages,
	})
	require.NoError(t, err)
	require.Len(t, advancedCustomConfig.Routes, 1)
	assert.Equal(t, dto.AdvancedCustomConverterNone, advancedCustomConfig.Routes[0].Converter)
}

func TestBuildProtocolAdvancedCustomConfigRejectsUnknownConverter(t *testing.T) {
	_, err := buildProtocolAdvancedCustomConfig(model.ChannelProtocolResolvedConfig{
		ClientPath:       "/v1/chat/completions",
		EndpointPath:     "/v1/chat/completions",
		AuthType:         model.UpstreamProtocolAuthTypeBearer,
		AuthHeader:       "Authorization",
		RequestConverter: "unknown_converter",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "converter is not registered")
}
