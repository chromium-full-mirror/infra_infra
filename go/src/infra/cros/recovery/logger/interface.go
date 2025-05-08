package logger

import (
	"context"
	"io"
)

// Logger represents a simple interface for logging data.
type Logger interface {
	// Debugf log message at Debugf level.
	Debugf(format string, args ...any)
	// Infof is like Debug, but logs at Infof level.
	Infof(format string, args ...any)
	// Warningf is like Debug, but logs at Warningf level.
	Warningf(format string, args ...any)
	// Errorf is like Debug, but logs at Errorf level.
	Errorf(format string, args ...any)
}

// LogIndenter represents a simple interface to provide option to set indent logs.
// Interface mostly used for local debugging and testing.
type LogIndenter interface {
	// Indent increment indentation for logger.
	Indent()
	// Dedent decrement indentation for logger.
	Dedent()
}

// StepLogCloser represents a simple function to unregister/close step's log in the logger.
type StepLogCloser func()

// StepLogRegister represents a simple interface to provide option to add step's log to the logger.
type StepLogRegister interface {
	// RegisterStepLog add step's io.Writer as new logger.
	RegisterStepLog(ctx context.Context, wr io.Writer) (StepLogCloser, error)
}
