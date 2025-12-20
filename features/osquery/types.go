package osquery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// EnrollmentRequest is the request body for the /enroll endpoint.
type EnrollmentRequest struct {
	EnrollSecret   string          `json:"enroll_secret"`
	HostIdentifier string          `json:"host_identifier"`
	HostDetails    json.RawMessage `json:"host_details"`
}

// EnrollmentResponse is the response body for the /enroll endpoint.
type EnrollmentResponse struct {
	NodeKey     string `json:"node_key"`
	NodeInvalid bool   `json:"node_invalid"`
}

// ConfigRequest is the request body for the /config endpoint.
type ConfigRequest struct {
	NodeKey string `json:"node_key"`
}

// ConfigResponse is the response body for the /config endpoint.
type ConfigResponse struct {
	Options     map[string]any            `json:"options,omitempty"`
	Schedule    map[string]ScheduledQuery `json:"schedule,omitempty"`
	Decorators  map[string][]string       `json:"decorators,omitempty"`
	NodeInvalid bool                      `json:"node_invalid,omitempty"`
}

type ScheduledQuery struct {
	Query    string `json:"query"`
	Interval int    `json:"interval"`
}

// LoggerRequest is the request body for the /logger endpoint.
type LoggerRequest struct {
	NodeKey string            `json:"node_key"`
	LogType string            `json:"log_type"`
	Data    []json.RawMessage `json:"data"`
}

// LoggerResponse is the response body for the /logger endpoint.
type LoggerResponse struct {
	NodeInvalid bool `json:"node_invalid,omitempty"`
}

type UnixTime int64

func (u *UnixTime) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) || len(data) == 0 {
		*u = 0
		return nil
	}

	// Some osquery log plugins emit unixTime as a JSON string, others
	// emit it as a JSON number (sometimes with .0).
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("unmarshal unixTime string: %w", err)
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("parse unixTime string: %w", err)
		}
		*u = UnixTime(int64(f))
		return nil
	}

	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("unmarshal unixTime number: %w", err)
	}
	f, err := n.Float64()
	if err != nil {
		return fmt.Errorf("parse unixTime number: %w", err)
	}
	*u = UnixTime(int64(f))
	return nil
}

type ResultLog struct {
	Name           string            `json:"name"`
	HostIdentifier string            `json:"hostIdentifier"`
	CalendarTime   string            `json:"calendarTime"`
	UnixTime       UnixTime          `json:"unixTime"`
	Action         string            `json:"action"`
	Columns        map[string]string `json:"columns"`
}

type StatusLog struct {
	Line         int      `json:"line"`
	Message      string   `json:"message"`
	Severity     int      `json:"severity"`
	Filename     string   `json:"filename"`
	CalendarTime string   `json:"calendarTime"`
	UnixTime     UnixTime `json:"unixTime"`
}

// DistributedReadRequest is the request body for the /distributed_read endpoint.
type DistributedReadRequest struct {
	NodeKey string `json:"node_key"`
}

// DistributedReadResponse is the response body for the /distributed_read endpoint.
type DistributedReadResponse struct {
	Queries     map[string]string `json:"queries"`
	Discovery   map[string]string `json:"discovery,omitempty"`
	NodeInvalid bool              `json:"node_invalid,omitempty"`
}

// DistributedWriteRequest is the request body for the /distributed_write endpoint.
type DistributedWriteRequest struct {
	NodeKey  string                         `json:"node_key"`
	Queries  map[string][]map[string]string `json:"queries"`
	Statuses map[string]int                 `json:"statuses"`
}

// DistributedWriteResponse is the response body for the /distributed_write endpoint.
type DistributedWriteResponse struct {
	NodeInvalid bool `json:"node_invalid,omitempty"`
}
