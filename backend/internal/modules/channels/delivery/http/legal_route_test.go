package http

import (
	"bytes"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMixedCollaborationLegalPredicatesKeepSafetyCleanupOpen(t *testing.T) {
	tests := []struct {
		name      string
		predicate func(map[string]json.RawMessage) bool
		payload   string
		wantLegal bool
	}{
		{
			name:      "full collaboration lockdown stays open",
			predicate: collaborationSettingsNeedsLegalAcceptance,
			payload:   `{"room_mode":"internal","lobby_enabled":true,"chat_locked":true,"guest_microphone_enabled":false,"guest_camera_enabled":false,"default_participant_role":"listener"}`,
			wantLegal: false,
		},
		{
			name:      "collaboration expansion requires legal acceptance",
			predicate: collaborationSettingsNeedsLegalAcceptance,
			payload:   `{"room_mode":"public","lobby_enabled":false,"chat_locked":false,"guest_microphone_enabled":true,"guest_camera_enabled":true,"default_participant_role":"presenter"}`,
			wantLegal: true,
		},
		{
			name:      "partial collaboration payload fails closed",
			predicate: collaborationSettingsNeedsLegalAcceptance,
			payload:   `{"room_mode":"public","default_participant_role":"presenter"}`,
			wantLegal: true,
		},
		{name: "listener demotion stays open", predicate: collaborationRoleNeedsLegalAcceptance, payload: `{"role":"listener"}`, wantLegal: false},
		{name: "role promotion requires legal acceptance", predicate: collaborationRoleNeedsLegalAcceptance, payload: `{"role":"moderator"}`, wantLegal: true},
		{name: "missing role fails closed", predicate: collaborationRoleNeedsLegalAcceptance, payload: `{}`, wantLegal: true},
		{name: "malformed role fails closed", predicate: collaborationRoleNeedsLegalAcceptance, payload: `{"role":false}`, wantLegal: true},
		{name: "clear breakout assignments stays open", predicate: breakoutAssignmentsNeedLegalAcceptance, payload: `{"assigned_user_ids":[]}`, wantLegal: false},
		{name: "add breakout participant requires legal acceptance", predicate: breakoutAssignmentsNeedLegalAcceptance, payload: `{"assigned_user_ids":["user-1"]}`, wantLegal: true},
		{name: "missing breakout assignments fail closed", predicate: breakoutAssignmentsNeedLegalAcceptance, payload: `{}`, wantLegal: true},
		{name: "malformed breakout assignments fail closed", predicate: breakoutAssignmentsNeedLegalAcceptance, payload: `{"assigned_user_ids":false}`, wantLegal: true},
		{name: "disable recording stays open", predicate: recordingPolicyNeedsLegalAcceptance, payload: `{"enabled":false,"provider":"disabled"}`, wantLegal: false},
		{name: "enable recording requires legal acceptance", predicate: recordingPolicyNeedsLegalAcceptance, payload: `{"enabled":true,"provider":"jibri"}`, wantLegal: true},
		{name: "missing recording policy fails closed", predicate: recordingPolicyNeedsLegalAcceptance, payload: `{}`, wantLegal: true},
		{name: "withdraw recording consent stays open", predicate: recordingConsentNeedsLegalAcceptance, payload: `{"consented":false}`, wantLegal: false},
		{name: "positive recording consent requires legal acceptance", predicate: recordingConsentNeedsLegalAcceptance, payload: `{"consented":true}`, wantLegal: true},
		{name: "malformed recording consent fails closed", predicate: recordingConsentNeedsLegalAcceptance, payload: `{"consented":"false"}`, wantLegal: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var payload map[string]json.RawMessage
			if err := json.Unmarshal([]byte(test.payload), &payload); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			if got := test.predicate(payload); got != test.wantLegal {
				t.Fatalf("predicate = %v, want %v", got, test.wantLegal)
			}
		})
	}
}

func TestRequireLegalForJSONFailsClosedWithoutGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT(
		"/recording-policy",
		requireLegalForJSON(nil, recordingPolicyNeedsLegalAcceptance),
		func(c *gin.Context) { c.Status(nethttp.StatusNoContent) },
	)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(nethttp.MethodPut, "/recording-policy", bytes.NewBufferString(`{"enabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != nethttp.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, nethttp.StatusServiceUnavailable)
	}
}

func TestRequireLegalForJSONInvokesGateAndRestoresBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name            string
		body            string
		wantLegalCalled bool
	}{
		{name: "enable capture is gated", body: `{"enabled":true,"provider":"jibri"}`, wantLegalCalled: true},
		{name: "disable capture bypasses gate", body: `{"enabled":false,"provider":"disabled"}`, wantLegalCalled: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			legalCalled := false
			boundEnabled := true
			router := gin.New()
			router.PUT(
				"/recording-policy",
				requireLegalForJSON(func(c *gin.Context) {
					legalCalled = true
					c.Next()
				}, recordingPolicyNeedsLegalAcceptance),
				func(c *gin.Context) {
					var payload struct {
						Enabled bool `json:"enabled"`
					}
					if err := c.ShouldBindJSON(&payload); err != nil {
						c.AbortWithStatus(nethttp.StatusBadRequest)
						return
					}
					boundEnabled = payload.Enabled
					c.Status(nethttp.StatusNoContent)
				},
			)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(nethttp.MethodPut, "/recording-policy", bytes.NewBufferString(test.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)

			if recorder.Code != nethttp.StatusNoContent {
				t.Fatalf("status = %d, body was not restored for the handler", recorder.Code)
			}
			if legalCalled != test.wantLegalCalled {
				t.Fatalf("legal gate called = %v, want %v", legalCalled, test.wantLegalCalled)
			}
			if boundEnabled != test.wantLegalCalled {
				t.Fatalf("handler bound enabled = %v, want %v", boundEnabled, test.wantLegalCalled)
			}
		})
	}
}
