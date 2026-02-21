package websocket

// Action defines the type of action in a request.
type Action string

const (
	ActionAutosave Action = "autosave"
	ActionSubmit   Action = "submit"
	ActionPing     Action = "ping"
)

// Event defines the type of event in a response.
type Event string

const (
	EventError   Event = "error"
	EventSuccess Event = "success"
	EventGraded  Event = "graded"
)

// RequestPayload represents the standard incoming message structure.
// Specific payloads (like Autosave) can embed this or match this structure.
type RequestPayload struct {
	Action Action `json:"action"`
	// For flexibility, specific data fields are often flattened in simple protocols,
	// but a "data" field is more scalable.
	// However, to support existing flat structure:
	QID    string `json:"q_id,omitempty"`
	Answer string `json:"ans,omitempty"`
}

// ResponsePayload represents the standard outgoing message structure.
type ResponsePayload struct {
	Event Event       `json:"event"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
	// Deprecated: flattened fields for backward compatibility if needed,
	// but purely new code should use Data.
	Status string   `json:"status,omitempty"`
	Score  *float64 `json:"score,omitempty"`
}
