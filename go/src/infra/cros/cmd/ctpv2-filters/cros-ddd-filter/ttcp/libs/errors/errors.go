// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package errors

import (
	"bytes"
	"fmt"
	"runtime/debug"
	"strings"
)

func NotImplemented() error {
	return NewError("Not implemented")
}

type ApiErrorStruct struct {
	ApiError error
	Stack    string
}

func (nErr *ApiErrorStruct) Error() string {
	return fmt.Sprintf("ApiError: %s \n Stack: %s", nErr.ApiError, nErr.Stack)
}

// ApiError wraps an error from an thrid party API and attaches a stack trace.
func ApiError(err error) error {
	stacktrace := debug.Stack()
	stacktraceStr := strings.Join(strings.Split(string(stacktrace), "\n")[5:], "\n")
	if err == nil {
		return nil
	}
	return &ApiErrorStruct{
		ApiError: err,
		Stack:    stacktraceStr,
	}
}

func NewError(msg string) error {
	stacktrace := debug.Stack()
	stacktraceStr := strings.Join(strings.Split(string(stacktrace), "\n")[5:], "\n")
	return fmt.Errorf("Error: %s \n Stack:%s", msg, stacktraceStr)
}

func NewErrorf(format string, args ...any) error {
	stacktrace := debug.Stack()
	stacktraceStr := strings.Join(strings.Split(string(stacktrace), "\n")[5:], "\n")
	errorMsg := fmt.Sprintf(format, args...)
	return fmt.Errorf("Error: %s \n Stack:%s", errorMsg, stacktraceStr)
}

type NestedError struct {
	ErrorMessage string
	InnerError   error
}

func (nErr *NestedError) Error() string {
	return fmt.Sprintf("Error: %s \n InnerError: %s", nErr.ErrorMessage, nErr.InnerError.Error())
}

func JoinError(outerError string, innerError error) error {
	if innerError == nil {
		return nil
	}
	return &NestedError{
		ErrorMessage: outerError,
		InnerError:   innerError,
	}
}

type MultivalidationError struct {
	Errors []error
}

func (me *MultivalidationError) Error() string {
	var buffer bytes.Buffer

	for _, e := range me.Errors {
		buffer.WriteString(e.Error())
		buffer.WriteString("\n")
	}
	return buffer.String()
}

func MultiCheck(errorArgs ...error) error {
	errorList := &MultivalidationError{}
	for _, e := range errorArgs {
		if e != nil {
			errorList.Errors = append(errorList.Errors, e)
		}
	}
	if len(errorList.Errors) == 0 {
		return nil
	} else {
		return errorList
	}
}
