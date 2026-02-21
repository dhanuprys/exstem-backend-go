package validator

import (
	"errors"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	govalidator "github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
)

// trans is the singleton English translator for validation errors.
var trans ut.Translator

// Setup registers the validator with English translations on Gin's binding engine.
// Call once during application startup.
func Setup() {
	if v, ok := binding.Validator.Engine().(*govalidator.Validate); ok {
		// Use JSON tag name for field names in error messages.
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})

		// Register English translations.
		enLocale := en.New()
		uni := ut.New(enLocale, enLocale)
		trans, _ = uni.GetTranslator("en")
		en_translations.RegisterDefaultTranslations(v, trans)
	}
}

// TranslateErrors takes a binding/validation error and returns a map of
// field name → human-readable error message. If the error is not a
// validation error, it returns a single-key map with "detail".
func TranslateErrors(err error) map[string]string {
	fields := make(map[string]string)

	var ve govalidator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			fields[fe.Field()] = fe.Translate(trans)
		}
		return fields
	}

	// Not a validation error (e.g., JSON syntax error).
	fields["detail"] = err.Error()
	return fields
}

// Bind binds and validates the request body into dst.
// Returns nil on success or a translated field error map on failure.
func Bind(c *gin.Context, dst interface{}) map[string]string {
	if err := c.ShouldBindJSON(dst); err != nil {
		return TranslateErrors(err)
	}
	return nil
}
