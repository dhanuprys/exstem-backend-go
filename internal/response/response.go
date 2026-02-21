package response

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Response is the standardized API response envelope.
type Response struct {
	Data       interface{} `json:"data"`
	Error      *ErrorBody  `json:"error,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
	Metadata   Metadata    `json:"metadata"`
}

// ErrorBody represents a structured error response.
type ErrorBody struct {
	Code    ErrCode           `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// Pagination holds pagination information.
type Pagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// Metadata includes request tracing and timing.
type Metadata struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
}

// ────────────────────────────────────────────────────────────────────────────
// Helper builders
// ────────────────────────────────────────────────────────────────────────────

// Success sends a successful JSON response with the given status code and data.
func Success(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, Response{
		Data:     data,
		Metadata: buildMetadata(c),
	})
}

// SuccessWithPagination sends a successful response with pagination metadata.
func SuccessWithPagination(c *gin.Context, statusCode int, data interface{}, pagination *Pagination) {
	c.JSON(statusCode, Response{
		Data:       data,
		Pagination: pagination,
		Metadata:   buildMetadata(c),
	})
}

// Fail sends an error response with an error code and no field-level details.
func Fail(c *gin.Context, statusCode int, code ErrCode) {
	c.JSON(statusCode, Response{
		Data:     nil,
		Error:    &ErrorBody{Code: code, Message: GetMessage(code)},
		Metadata: buildMetadata(c),
	})
}

// FailWithFields sends an error response with field-level validation details.
func FailWithFields(c *gin.Context, statusCode int, code ErrCode, fields map[string]string) {
	c.JSON(statusCode, Response{
		Data:     nil,
		Error:    &ErrorBody{Code: code, Message: GetMessage(code), Fields: fields},
		Metadata: buildMetadata(c),
	})
}

// AbortFail aborts the middleware chain and sends an error response.
func AbortFail(c *gin.Context, statusCode int, code ErrCode) {
	c.AbortWithStatusJSON(statusCode, Response{
		Data:     nil,
		Error:    &ErrorBody{Code: code, Message: GetMessage(code)},
		Metadata: buildMetadata(c),
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ────────────────────────────────────────────────────────────────────────────

func buildMetadata(c *gin.Context) Metadata {
	reqID, _ := c.Get(ContextKeyRequestID)
	id, ok := reqID.(string)
	if !ok || id == "" {
		id = uuid.New().String() // Fallback if middleware not applied
	}
	return Metadata{
		RequestID: id,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}
