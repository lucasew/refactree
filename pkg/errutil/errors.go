package errutil

import (
	"log"
)

// ReportError is the centralized error reporting function for the project.
// It logs the error and could be wired to Sentry or another error tracking service.
func ReportError(err error, metadata map[string]interface{}) {
	if err == nil {
		return
	}
	// TODO: wire to Sentry if available in the future.
	log.Printf("ERROR: %v | metadata: %v", err, metadata)
}
