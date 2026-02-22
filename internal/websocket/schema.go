package websocket

// ─── Actions (Client → Server) ──────────────────────────────────────

type Action string

const (
	ActionAutosave Action = "autosave"
	ActionSubmit   Action = "submit"
	ActionPing     Action = "ping"
	ActionCheat    Action = "cheat"
)

// RequestEnvelope is used to peek at the action before full parsing.
type RequestEnvelope struct {
	Action Action `json:"action"`
}

// AutosaveRequest is sent by the client to save a single answer.
type AutosaveRequest struct {
	Action Action `json:"action"`
	QID    string `json:"q_id"`
	Answer string `json:"ans"`
}

// CheatRequest is sent by the client to report a cheat event.
type CheatRequest struct {
	Action  Action `json:"action"`
	Payload string `json:"payload"` // Receives the JSON string directly
}

// SubmitRequest is sent by the client to finish and grade the exam.
type SubmitRequest struct {
	Action Action `json:"action"`
}

// ─── Events (Server → Client) ───────────────────────────────────────

type Event string

const (
	EventError   Event = "error"
	EventSuccess Event = "success"
	EventGraded  Event = "graded"
	EventPong    Event = "pong"
)

type AutosaveResponse struct {
	Event  Event  `json:"event"`
	Status string `json:"status"`
}

type GradedResponse struct {
	Event  Event   `json:"event"`
	Status string  `json:"status"`
	Score  float64 `json:"score"`
}

type ErrorResponse struct {
	Event Event  `json:"event"`
	Error string `json:"error"`
}

type PongResponse struct {
	Event Event `json:"event"`
}
