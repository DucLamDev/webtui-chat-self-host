package http

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/duclamdev/application-chat/backend/internal/shared/constants"
	"github.com/gin-gonic/gin"
)

func TestICEServersIssuesShortLivedTURNRESTCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixedNow := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	handler := NewHandler(nil)
	handler.now = func() time.Time { return fixedNow }
	handler.SetICEConfiguration(
		[]map[string]any{{"urls": "stun:chat.example.com:3478"}},
		[]string{"turn:chat.example.com:3478?transport=udp"},
		"0123456789abcdef0123456789abcdef",
		10*time.Minute,
	)
	router := gin.New()
	router.GET("/api/v1/calls/ice-servers", func(c *gin.Context) {
		c.Set(constants.ContextUserID, "user-1")
		handler.ICEServers(c)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/calls/ice-servers", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data struct {
			ICEServers []map[string]any `json:"ice_servers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.ICEServers) != 2 {
		t.Fatalf("ice servers = %#v", envelope.Data.ICEServers)
	}
	turn := envelope.Data.ICEServers[1]
	username := "1785543000:user-1"
	mac := hmac.New(sha1.New, []byte("0123456789abcdef0123456789abcdef"))
	_, _ = mac.Write([]byte(username))
	if turn["username"] != username || turn["credential"] != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
		t.Fatalf("unexpected TURN credentials: %#v", turn)
	}
}
