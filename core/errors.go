package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
)

// The stable codes of failures an app can handle, NSH's and Node's: the
// message is for people and may change.
const (
	// CodeAccessDenied says the capabilities do not grant what was asked,
	// Error's Permission and Resource saying which and of what.
	CodeAccessDenied = "ERR_ACCESS_DENIED"
	// CodeUnavailable says this platform does not do what was asked.
	CodeUnavailable = "ERR_FEATURE_UNAVAILABLE_ON_PLATFORM"
	// CodeQuotaExceeded says a runtime quota ran out: calls waiting at
	// once, storage.
	CodeQuotaExceeded = "ERR_NSH_QUOTA_EXCEEDED"
	// CodeHostError says the embedding app answered with an error.
	CodeHostError = "ERR_NSH_HOST_ERROR"
	// CodeTimedOut says an answer did not come in time.
	CodeTimedOut = "ETIMEDOUT"
	// CodeNotFound and CodeDenied are what the system answered: never
	// the capabilities, which are CodeAccessDenied.
	CodeNotFound = "ENOENT"
	CodeDenied   = "EACCES"
)

// Error is a failure with a stable Code.
type Error struct {
	Code    string
	Message string
	// Permission is the capability a refusal lacks and Resource the path,
	// URL or name asked for, with CodeAccessDenied.
	Permission, Resource string
	// Err is what failed underneath, if anything.
	Err error
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Unwrap() error { return e.Err }

// AccessDenied is the failure of what a capability does not grant.
func AccessDenied(permission, resource string) *Error {
	return &Error{Code: CodeAccessDenied, Permission: permission, Resource: resource,
		Message: fmt.Sprintf("access to %q needs the %q capability", resource, permission)}
}

// Unavailable is the failure of what this platform does not do.
func Unavailable(message string) *Error { return &Error{Code: CodeUnavailable, Message: message} }

// ErrorCode is the stable code of a failure, "" for one with none.
func ErrorCode(err error) string {
	var coded *Error
	var full *StorageFullError
	switch {
	case errors.As(err, &coded):
		return coded.Code
	case errors.As(err, &full):
		return CodeQuotaExceeded
	case errors.Is(err, ErrHostTimeout), errors.Is(err, context.DeadlineExceeded):
		return CodeTimedOut
	case errors.Is(err, fs.ErrNotExist):
		return CodeNotFound
	case errors.Is(err, fs.ErrPermission):
		return CodeDenied
	}
	return ""
}
