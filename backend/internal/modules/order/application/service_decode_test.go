package application

import (
	"encoding/json"
	"testing"
)

func TestServicesExpiringEnvelopeAcceptsEmptyByTypeArray(t *testing.T) {
	var envelope ServicesExpiringEnvelope
	if err := json.Unmarshal([]byte(`{
		"ok": true,
		"status": "success",
		"data": {"summary": {"total": 0, "by_type": []}, "items": []}
	}`), &envelope); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(envelope.Data.Summary.ByType) != 0 {
		t.Fatalf("by_type = %#v, want empty map", envelope.Data.Summary.ByType)
	}
}

func TestServicesExpiringEnvelopeAcceptsByTypeObject(t *testing.T) {
	var envelope ServicesExpiringEnvelope
	if err := json.Unmarshal([]byte(`{
		"ok": true,
		"status": "success",
		"data": {"summary": {"total": 3, "by_type": {"vps": 2, "proxy": 1}}}
	}`), &envelope); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if envelope.Data.Summary.ByType["vps"] != 2 || envelope.Data.Summary.ByType["proxy"] != 1 {
		t.Fatalf("by_type = %#v", envelope.Data.Summary.ByType)
	}
}

func TestServicesExpiringEnvelopeAcceptsByTypeRows(t *testing.T) {
	var envelope ServicesExpiringEnvelope
	if err := json.Unmarshal([]byte(`{
		"ok": true,
		"status": "success",
		"data": {"summary": {"by_type": [{"service_type_key": "vps", "count": 2}]}}
	}`), &envelope); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if envelope.Data.Summary.ByType["vps"] != 2 {
		t.Fatalf("by_type = %#v", envelope.Data.Summary.ByType)
	}
}
