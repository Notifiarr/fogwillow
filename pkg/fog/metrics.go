package fog

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics contains the exported application metrics in prometheus format.
type Metrics struct {
	Uptime   prometheus.CounterFunc
	InMemory prometheus.GaugeFunc // File buffers currently stored in memory.
	ChanBuff prometheus.GaugeFunc // Size of the buffer between the packet reader and packet processor.
	FileBuff prometheus.Gauge     // Size of the file system change channel buffer. // XXX: no buffer yet
	Packets  prometheus.Counter   // UDP packets received.
	Files    prometheus.Counter   // Number of files flushed and written to disk.
	Bytes    prometheus.Counter   // Number of bytes written to disk.
	Errors   prometheus.Counter   // Number of errors the application has generated.
	Deletes  prometheus.Counter   // Number of delete commands issued.
	Truncate prometheus.Counter   // Number of files truncated on command.
	Flushes  prometheus.Counter   // Number of file buffer that were flushed on command.
	Expires  prometheus.Counter   // Number of file buffer that were flushed due to expiry.
}

func getMetrics(config *Config) *Metrics {
	start := time.Now()

	return &Metrics{
		Uptime: promauto.NewCounterFunc(prometheus.CounterOpts{
			Name: "fogwillow_uptime_seconds_total",
			Help: "Seconds Fog Willow has been running",
		}, func() float64 { return time.Since(start).Seconds() }),
		InMemory: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "fogwillow_file_buffers_in_memory",
			Help: "Count of file buffers currently stored in memory awaiting flush.",
		}, func() float64 { return float64(config.willow.Len()) }),
		ChanBuff: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "fogwillow_packet_processor_buffer",
			Help: "Size of the buffer between the packet reader and packet processor.",
		}, func() float64 { return float64(len(config.packets)) }),
		FileBuff: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "fogwillow_filesystem_change_buffer",
			Help: "Size of the file system change channel buffer.",
		}),
		Packets: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fogwillow_packets_total",
			Help: "Number of UDP packets processed. Often 1 packet per log line.",
		}),
		Files: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fogwillow_files_written_total",
			Help: "Number of files flushed and written to disk.",
		}),
		Bytes: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fogwillow_bytes_written_total",
			Help: "Number of bytes written to disk.",
		}),
		Errors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fogwillow_app_errors_total",
			Help: "Number of errors the application has generated.",
		}),
		Deletes: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fogwillow_file_buffer_deletes_total",
			Help: "Number of delete commands issued.",
		}),
		Truncate: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fogwillow_file_buffer_truncates_total",
			Help: "Number of files truncated on command.",
		}),
		Flushes: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fogwillow_file_buffer_flushes_total",
			Help: "Number of file buffer that were flushed on command.",
		}),
		Expires: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fogwillow_file_buffer_expires_total",
			Help: "Number of file buffer that were flushed due to expiry.",
		}),
	}
}
