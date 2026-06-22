package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
