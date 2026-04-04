package httpserver

import (
	"fmt"
	"net/http"

	apachelog "github.com/lestrrat-go/apache-logformat/v2"
	"golift.io/rotatorr"
	"golift.io/rotatorr/timerotator"
)

const (
	// Access log file mode.
	LogFileMode = 0o644
	// Apache Combined Log Format: host ident user time "request" status bytes "referer" "user-agent".
	CombinedLogFormat = `%h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-Agent}i"`
	MaxStupidValue    = uint(9999999) // for comparing with config.
)

// newAccessLog creates a rotating access log writer from config.
// Returns nil when AccessLog is empty, disabling access logging.
func newAccessLog(config *Config) *rotatorr.Logger {
	if config.AccessLog == "" {
		return nil
	}

	return rotatorr.NewMust(&rotatorr.Config{
		Filepath: config.AccessLog,
		FileSize: int64(config.AccessLogMB * OneMB), //nolint:gosec // size is protected by config.
		FileMode: LogFileMode,
		Rotatorr: &timerotator.Layout{
			FileCount: int(config.AccessLogFiles), //nolint:gosec // count is protected by config.
		},
	})
}

// wrapWithAccessLog wraps handler with Apache Combined-format access logging.
// It panics if the log format string is invalid (it is a package constant so this
// should never happen outside a programming error).
func wrapWithAccessLog(handler http.Handler, logger *rotatorr.Logger) http.Handler {
	if logger == nil {
		return handler
	}

	apache, err := apachelog.New(CombinedLogFormat)
	if err != nil {
		panic(fmt.Sprintf("building apache access log format: %v", err))
	}

	return apache.Wrap(handler, logger)
}
