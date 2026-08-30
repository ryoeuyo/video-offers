package domain

import (
	"errors"
	"fmt"
)

// Kind — класс доменной ошибки. Транспортный слой мапит его в HTTP-статус,
// больше никто про статусы не знает.
type Kind int

const (
	KindInternal Kind = iota
	KindInvalidInput
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindUnprocessable
	KindRateLimited
)

// Сентинелы для errors.Is: errors.Is(err, domain.ErrNotFound).
var (
	ErrInvalidInput  = &Error{Kind: KindInvalidInput, Code: "invalid_input", Message: "некорректные данные"}
	ErrUnauthorized  = &Error{Kind: KindUnauthorized, Code: "unauthorized", Message: "требуется авторизация"}
	ErrForbidden     = &Error{Kind: KindForbidden, Code: "forbidden", Message: "недостаточно прав"}
	ErrNotFound      = &Error{Kind: KindNotFound, Code: "not_found", Message: "не найдено"}
	ErrConflict      = &Error{Kind: KindConflict, Code: "conflict", Message: "конфликт состояния"}
	ErrUnprocessable = &Error{Kind: KindUnprocessable, Code: "unprocessable", Message: "не удалось обработать"}
	ErrRateLimited   = &Error{Kind: KindRateLimited, Code: "rate_limited", Message: "слишком много запросов"}
	ErrInternal      = &Error{Kind: KindInternal, Code: "internal", Message: "внутренняя ошибка"}
)

// Error — доменная ошибка с машиночитаемым кодом. Code уходит клиенту как есть,
// поэтому он часть контракта API: менять существующие коды нельзя.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Details map[string]any
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

// Is считает ошибки одинаковыми по Kind, чтобы errors.Is(err, ErrNotFound)
// срабатывал и на уточнённых через With* копиях.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return e.Kind == t.Kind
}

// Все With* возвращают копию: сентинелы должны оставаться неизменяемыми.
func (e *Error) WithCode(code, message string) *Error {
	c := *e
	c.Code, c.Message = code, message
	return &c
}

func (e *Error) WithDetails(details map[string]any) *Error {
	c := *e
	c.Details = details
	return &c
}

func (e *Error) Wrap(cause error) *Error {
	c := *e
	c.cause = cause
	return &c
}

// AsError достаёт доменную ошибку из цепочки. Если её там нет — значит,
// ошибка неожиданная, и наружу пойдёт 500 без подробностей.
func AsError(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}
