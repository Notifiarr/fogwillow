package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics contains the exported application metrics in prometheus format.
type Metrics struct {
	Uptime   prometheus.CounterFunc
	InMemory prometheus.GaugeFunc     // File buffers currently stored in memory.
	ChanBuff prometheus.GaugeFunc     // Size of the buffer between the packet reader and packet processor.
	FileBuff prometheus.GaugeFunc     // Size of the file system change channel buffer.
	Packets  prometheus.Counter       // UDP packets received.
	Bytes    prometheus.Counter       // Number of bytes written to disk.
	Errors   prometheus.Counter       // Number of errors the application has generated.
	Truncate prometheus.Counter       // Number of files truncated on command.
	Flushes  prometheus.Counter       // Number of file buffer that were flushed on command.
	Expires  prometheus.Counter       // Number of file buffer that were flushed due to expiry.
	Ages     *prometheus.HistogramVec // The age of file buffers in memory when they are flushed to disk.
	Durs     *prometheus.HistogramVec // The length of time it takes to write a file to disk.
}

// Funcs are all required to make this work.
type Funcs struct {
	InMemory func() float64
	ChanBuff func() float64
	FileBuff func() float64
}

// Get returns some metrics you can pass around and fill in.
func Get(fnc Funcs) *Metrics {
	start := time.Now()

	return &Metrics{
		Uptime: promauto.NewCounterFunc(prometheus.CounterOpts{
			Name: "fogwillow_uptime_seconds_total",
			Help: "Seconds Fog Willow has been running",
		}, func() float64 { return time.Since(start).Seconds() }),
		InMemory: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "fogwillow_file_buffers_in_memory",
			Help: "Count of file buffers currently stored in memory awaiting flush.",
		}, fnc.InMemory),
		ChanBuff: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "fogwillow_packet_processor_buffer",
			Help: "Size of the buffer between the packet reader and packet processor.",
		}, fnc.ChanBuff),
		FileBuff: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "fogwillow_filesystem_change_buffer",
			Help: "Size of the file system change channel buffer.",
		}, fnc.FileBuff),
		Packets: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fogwillow_packets_total",
			Help: "Number of UDP packets processed. Often 1 packet per log line.",
		}),
		Bytes: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fogwillow_bytes_written_total",
			Help: "Number of bytes written to disk.",
		}),
		Errors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fogwillow_app_errors_total",
			Help: "Number of errors the application has generated.",
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
		Ages: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "fogwillow_file_buffer_ages_seconds",
			Help:    "The age of file buffers in memory when they are flushed to disk.",
			Buckets: []float64{0.001, 0.01, 0.2, 1.2, 8, 30},
		}, []string{"kind"}), // kind="delete" | kind="file"
		Durs: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "fogwillow_file_write_duration_seconds",
			Help:    "The length of time it takes to delete or write a file buffer to disk.",
			Buckets: []float64{0.001, 0.1, 1, 5, 15},
		}, []string{"kind"}), // kind="delete" | kind="file"
	}
}
