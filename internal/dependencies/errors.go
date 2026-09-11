package dependencies

import (
	"fmt"
	"strings"
)

// NotFoundError reports that an executable could not be located. The message is
// written for the settings screen, where a "Locate executable" action follows.
type NotFoundError struct {
	Name           string
	ConfiguredPath string
}

func (e *NotFoundError) Error() string {
	if e.ConfiguredPath != "" {
		return fmt.Sprintf("%s was not found at the configured path %s", DisplayName(e.Name), e.ConfiguredPath)
	}
	return fmt.Sprintf("%s was not found in the application folder or on PATH", DisplayName(e.Name))
}

// ProbeError reports that an executable was found but could not be run.
type ProbeError struct {
	Name   string
	Path   string
	Output string
	Err    error
}

func (e *ProbeError) Error() string {
	message := fmt.Sprintf("%s could not be started (%s)", DisplayName(e.Name), e.Err)
	if output := strings.TrimSpace(e.Output); output != "" {
		message += ": " + firstLine(output)
	}
	return message
}

func (e *ProbeError) Unwrap() error { return e.Err }

func firstLine(text string) string {
	if index := strings.IndexAny(text, "\r\n"); index >= 0 {
		return text[:index]
	}
	return text
}
