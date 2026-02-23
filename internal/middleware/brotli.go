// middleware/brotli.go
package middleware

import (
	"net/http"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
)

type BrotliConfig struct {
	Quality   int
	Skipper   func(c *gin.Context) bool
	MinLength int
}

var DefaultBrotliConfig = BrotliConfig{
	Quality:   brotli.DefaultCompression,
	MinLength: 1024,
	Skipper:   nil,
}

type brotliWriter struct {
	gin.ResponseWriter
	writer     *brotli.Writer
	buf        []byte
	minLength  int
	once       sync.Once
	compressed bool
}

func (bw *brotliWriter) Write(data []byte) (int, error) {
	bw.buf = append(bw.buf, data...)

	if len(bw.buf) >= bw.minLength {
		bw.once.Do(func() {
			bw.compressed = true
			bw.ResponseWriter.Header().Set("Content-Encoding", "br")
			bw.ResponseWriter.Header().Del("Content-Length")
		})
		n, err := bw.writer.Write(bw.buf)
		bw.buf = bw.buf[:0]
		return n, err
	}

	return len(data), nil
}

func (bw *brotliWriter) WriteString(s string) (int, error) {
	return bw.Write([]byte(s))
}

// Flush is called by SSE and streaming endpoints.
// Drains buffer as plain text and forwards flush to underlying writer.
func (bw *brotliWriter) Flush() {
	if len(bw.buf) > 0 {
		_, _ = bw.ResponseWriter.Write(bw.buf)
		bw.buf = bw.buf[:0]
	}
	bw.ResponseWriter.Flush()
}

func (bw *brotliWriter) flush() error {
	if len(bw.buf) == 0 {
		return nil
	}
	_, err := bw.ResponseWriter.Write(bw.buf)
	bw.buf = bw.buf[:0]
	return err
}

func Brotli() gin.HandlerFunc {
	return BrotliWithConfig(DefaultBrotliConfig)
}

func BrotliWithConfig(cfg BrotliConfig) gin.HandlerFunc {
	if cfg.Quality < 0 || cfg.Quality > 11 {
		cfg.Quality = brotli.DefaultCompression
	}
	if cfg.MinLength <= 0 {
		cfg.MinLength = DefaultBrotliConfig.MinLength
	}

	return func(c *gin.Context) {
		// Built-in skip for protocols that are incompatible with buffered compression
		if shouldSkip(c) {
			c.Next()
			return
		}

		// User-defined skip
		if cfg.Skipper != nil && cfg.Skipper(c) {
			c.Next()
			return
		}

		if !acceptsBrotli(c.Request) {
			c.Next()
			return
		}

		c.Header("Vary", "Accept-Encoding")

		bw := &brotliWriter{
			ResponseWriter: c.Writer,
			minLength:      cfg.MinLength,
			writer:         brotli.NewWriterLevel(c.Writer, cfg.Quality),
		}

		defer func() {
			if err := bw.flush(); err != nil {
				_ = c.Error(err)
			}
			if bw.compressed {
				bw.writer.Close()
			}
		}()

		c.Writer = bw
		c.Next()
	}
}

// shouldSkip returns true for protocols that are incompatible with
// buffered compression and must be passed through untouched.
func shouldSkip(c *gin.Context) bool {
	// SSE requires immediate streaming — buffering breaks it
	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
		return true
	}
	// WebSocket upgrades must not be intercepted — the Upgrade handshake
	// will fail if the response is wrapped or buffered
	if strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
		return true
	}
	return false
}

func acceptsBrotli(r *http.Request) bool {
	ae := r.Header.Get("Accept-Encoding")
	for _, enc := range strings.Split(ae, ",") {
		if strings.TrimSpace(strings.ToLower(enc)) == "br" {
			return true
		}
	}
	return false
}
