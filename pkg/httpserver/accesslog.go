package httpserver

import (
	"fmt"
	"net/http"

	apachelog "github.com/lestrrat-go/apache-logformat/v2"
	"golift.io/rotatorr"
	"golift.io/rotatorr/timerotator"
)

// Apache Combined Log Format: host ident user time "request" status bytes "referer" "user-agent".
const (
	combinedLogFormat  = `%h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-Agent}i"`
	accessLogFileMode  = 0o644
	accessLogMegabytes = 1 << 21 // 2MB
)

// newAccessLog creates a rotating access log writer from config.
// Returns nil when AccessLog is empty, disabling access logging.
func newAccessLog(config *Config) *rotatorr.Logger {
	if config.AccessLog == "" || config.AccessLogMB < 0 || config.AccessLogFiles < 0 {
		return nil
	}

	return rotatorr.NewMust(&rotatorr.Config{
		Filepath: config.AccessLog,
		FileSize: config.AccessLogMB * accessLogMegabytes,
		FileMode: accessLogFileMode,
		Rotatorr: &timerotator.Layout{
			FileCount: config.AccessLogFiles,
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

	apache, err := apachelog.New(combinedLogFormat)
	if err != nil {
		panic(fmt.Sprintf("building apache access log format: %v", err))
	}

	return apache.Wrap(handler, logger)
}
