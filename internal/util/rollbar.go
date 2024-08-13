package util

import (
	"fmt"

	"github.com/rollbar/rollbar-go"
)

// NotifyLog sends an informational message to Rollbar with the given format and arguments.
// It uses fmt.Sprintf to format the message before sending it.
func NotifyLog(format string, args ...any) {
	rollbar.Info(fmt.Sprintf(format, args...))
}

// NotifyError sends an error message to Rollbar for logging purposes.
// This function should be used to report non-critical errors.
func NotifyError(args ...any) {
	rollbar.Error(args)
}

// NotifyCritical sends a critical error message to Rollbar.
// This function should be used to report errors that are considered critical.
func NotifyCritical(args ...any) {
	rollbar.Critical(args)
}
