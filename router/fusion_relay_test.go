package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestFusionRelayRouteRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	SetRelayRouter(engine)

	found := false
	for _, route := range engine.Routes() {
		if route.Method == http.MethodPost && route.Path == "/v1/fusion/chat/completions" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestFusionRootRelayRouteRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	SetRelayRouter(engine)

	found := false
	for _, route := range engine.Routes() {
		if route.Method == http.MethodPost && route.Path == "/fusion" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestNormalChatCompletionsRouteStillRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	SetRelayRouter(engine)

	found := false
	for _, route := range engine.Routes() {
		if route.Method == http.MethodPost && route.Path == "/v1/chat/completions" {
			found = true
			break
		}
	}
	assert.True(t, found)
}
