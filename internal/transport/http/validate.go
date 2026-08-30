package httpapi

import (
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validate     *validator.Validate
	validateOnce sync.Once
)

func getValidator() *validator.Validate {
	validateOnce.Do(func() {
		validate = validator.New()
	})
	return validate
}
