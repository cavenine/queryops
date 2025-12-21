package osquery

import (
	"encoding/json"
	"testing"
)

func TestStatusLogUnmarshalNumbers(t *testing.T) {
	data := []byte(`{
		"hostIdentifier":"6c602cbc-9486-4789-acb4-d41c0193094e",
		"calendarTime":"Sat Dec 20 20:48:59 2025 UTC",
		"unixTime":1766263739,
		"severity":0,
		"filename":"tls.cpp",
		"line":263,
		"message":"TLS/HTTPS POST request to URI: https://example.com/osquery/logger",
		"version":"5.20.0",
		"decorations":{"host_uuid":"6c602cbc-9486-4789-acb4-d41c0193094e","hostname":"dakotaraptor"}
	}`)

	var log StatusLog
	if err := json.Unmarshal(data, &log); err != nil {
		t.Fatalf("unmarshal status log: %v", err)
	}

	if log.Severity != 0 {
		t.Fatalf("severity = %d, want 0", log.Severity)
	}
	if log.Line != 263 {
		t.Fatalf("line = %d, want 263", log.Line)
	}
	if got := int64(log.UnixTime); got != 1766263739 {
		t.Fatalf("unixTime = %d, want 1766263739", got)
	}
	if log.Filename != "tls.cpp" {
		t.Fatalf("filename = %q, want tls.cpp", log.Filename)
	}
	if log.Message == "" {
		t.Fatalf("message is empty")
	}
}
