package osquery

import "encoding/json"

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
	Schedule    map[string]ScheduledQuery `json:"schedule"`
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
