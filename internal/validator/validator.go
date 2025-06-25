package validator

import (
	"errors"
	"mime/multipart"
	"path/filepath"
	"regexp"
)

var (
	EmailRx = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	PhoneRX = regexp.MustCompile(`^\+?[\d\s\-\(\)]{10,20}$`)

)

type Validator struct {
	Errors map[string]string
}

func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

func PermittedValue[T comparable](value T, permittedValues ...T) bool {
	for i := range permittedValues {
		if value == permittedValues[i] {
			return true
		}
	}
	return false
}

func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

func Unique[T comparable](values []T) bool {
	uniqueValues := make(map[T]bool)

	for _, value := range values {
		uniqueValues[value] = true
	}

	return len(values) == len(uniqueValues)
}

func ValidatePDFFile(header *multipart.FileHeader) error {
	if filepath.Ext(header.Filename) != ".pdf" {
		return errors.New("file must be a PDF")
	}

	const maxPDFSize = 5 * 1024 * 1024 // 5MB
	if header.Size > maxPDFSize {
		return errors.New("file size must be less than 5MB")
	}

	return nil
}

func ValidateRequiredFields(title string, description string, category string, tags []string, submittedBy string) map[string]string {
	errors := make(map[string]string)

	if title == "" {
		errors["title"] = "must be provided"
	}

	if len(title) > 100 {
		errors["title"] = "must not be more than 100 bytes long"
	}

	if description == "" {
		errors["description"] = "must be provided"
	}

	if len(description) > 1000 {
		errors["description"] = "must not be more than 1000 bytes long"
	}

	if category == "" {
		errors["category"] = "must be provided"
	}

	if len(category) > 50 {
		errors["category"] = "must not be more than 50 bytes long"
	}

	if tags == nil {
		errors["tags"] = "must be provided"
	}

	if len(tags) < 1 {
		errors["tags"] = "must contain at least one tag"
	}

	if !Unique(tags) {
		errors["tags"] = "must not contain duplicate values"
	}

	if submittedBy == "" {
		errors["submitted_by"] = "must be provided"
	}
	return errors
}
