package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheSeparatesVersionedAssetsFromApplicationEntries(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		path         string
		cacheControl string
		pragma       string
	}{
		{
			name:         "root document is not stored",
			path:         "/",
			cacheControl: "no-store, no-cache, must-revalidate, private, max-age=0",
			pragma:       "no-cache",
		},
		{
			name:         "SPA route is not stored",
			path:         "/model-status",
			cacheControl: "no-store, no-cache, must-revalidate, private, max-age=0",
			pragma:       "no-cache",
		},
		{
			name:         "mutable public asset is not stored",
			path:         "/logo.png",
			cacheControl: "no-store, no-cache, must-revalidate, private, max-age=0",
			pragma:       "no-cache",
		},
		{
			name:         "fingerprinted build asset is immutable",
			path:         "/static/js/index.a3ad7bf17e.js",
			cacheControl: "public, max-age=31536000, immutable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(Cache())
			router.GET("/*path", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusNoContent, response.Code)
			assert.Equal(t, test.cacheControl, response.Header().Get("Cache-Control"))
			assert.Equal(t, test.pragma, response.Header().Get("Pragma"))
			assert.Equal(t, common.BuildCommit, response.Header().Get("Cache-Version"))
		})
	}
}
