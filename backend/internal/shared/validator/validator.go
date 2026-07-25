package validator

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type Result struct {
	Errors []FieldError
}

func (r *Result) Add(field string, message string) {
	r.Errors = append(r.Errors, FieldError{
		Field:   field,
		Message: message,
	})
}

func (r Result) OK() bool {
	return len(r.Errors) == 0
}

func Required(value string) bool {
	return value != ""
}

func MaxLen(value string, max int) bool {
	return len([]rune(value)) <= max
}
