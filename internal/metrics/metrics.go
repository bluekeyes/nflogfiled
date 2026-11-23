package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/bluekeyes/nflogfiled/internal/message"
	"github.com/bluekeyes/nflogfiled/internal/version"
)

const namespace = "nflogfiled"

// DropReason identifies why the daemon dropped a message.
type DropReason string

const (
	DropChannelFull DropReason = "channel_full"
	DropParseError  DropReason = "parse_error"
)

const (
	nativeHistogramBucketFactor    = 1.1
	nativeHistogramMaxBucketNumber = 100
	nativeHistogramMinResetDur     = time.Hour
)

// Metrics records daemon metrics. A nil *Metrics pointer is safe to use and
// disables all metrics.
type Metrics struct {
	registry *prometheus.Registry

	received       *prometheus.CounterVec
	dropped        *prometheus.CounterVec
	kernelOverflow *prometheus.CounterVec
	writeErrors    *prometheus.CounterVec
	writeDuration  *prometheus.HistogramVec
	messageLatency *prometheus.HistogramVec

	reloads           prometheus.Counter
	reloadErrors      prometheus.Counter
	lastReloadSuccess prometheus.Gauge
}

// New creates a Metrics with a new registry that also includes the standard
// Go runtime and process collectors.
func New() *Metrics {
	m := Metrics{
		received: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "messages_received_total",
			Help:      "Total number of nflog messages received and parsed.",
		}, []string{"group", "prefix", "protocol", "in_device", "out_device"}),

		dropped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "messages_dropped_total",
			Help:      "Total number of nflog messages received but not written.",
		}, []string{"group", "reason"}),

		kernelOverflow: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "kernel_buffer_overflows_total",
			Help:      "Total number of times the kernel reported dropping nflog messages because the socket buffer was full.",
		}, []string{"group"}),

		writeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "write_errors_total",
			Help:      "Total number of errors writing messages to output files.",
		}, []string{"file"}),

		writeDuration: newNativeHistogramVec(
			"write_duration_seconds",
			"Time to format and write a single message to an output file.",
			"file",
		),

		messageLatency: newNativeHistogramVec(
			"message_latency_seconds",
			"Time between receiving a message and completing its write to an output file.",
			"file",
		),

		reloads: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "reloads_total",
			Help:      "Total number of reloads.",
		}),

		reloadErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "reload_errors_total",
			Help:      "Total number of reloads that failed to reopen at least one output file.",
		}),

		lastReloadSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_reload_success_timestamp_seconds",
			Help:      "Unix timestamp of the last successful reload.",
		}),
	}

	buildInfo := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "build_info",
		Help:        "A metric with a constant value of 1 labeled by the nflogfiled version.",
		ConstLabels: prometheus.Labels{"version": version.Version()},
	})
	buildInfo.Set(1)

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		buildInfo,
		m.received,
		m.dropped,
		m.kernelOverflow,
		m.writeErrors,
		m.writeDuration,
		m.messageLatency,
		m.reloads,
		m.reloadErrors,
		m.lastReloadSuccess,
	)
	m.registry = registry

	return &m
}

func newNativeHistogramVec(name, help string, labels ...string) *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:                       namespace,
		Name:                            name,
		Help:                            help,
		NativeHistogramBucketFactor:     nativeHistogramBucketFactor,
		NativeHistogramMaxBucketNumber:  nativeHistogramMaxBucketNumber,
		NativeHistogramMinResetDuration: nativeHistogramMinResetDur,
	}, labels)
}

// Handler returns an HTTP handler that serves the metrics.
func (m *Metrics) Handler() http.Handler {
	if m == nil {
		return http.NotFoundHandler()
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		Registry: m.registry,
	})
}

// MessageReceived records a parsed message.
func (m *Metrics) MessageReceived(msg message.Message) {
	if m == nil {
		return
	}

	var protocol, inDevice, outDevice string
	if msg.Fields.Has(message.FieldPacketProtocol) {
		protocol = message.FormatProtocol(msg.Packet.Protocol)
	}
	if msg.Fields.Has(message.FieldInDevice) {
		inDevice = message.FormatDevice(msg.InDeviceName, msg.InDevice)
	}
	if msg.Fields.Has(message.FieldOutDevice) {
		outDevice = message.FormatDevice(msg.OutDeviceName, msg.OutDevice)
	}

	m.received.WithLabelValues(groupLabel(msg.Group), msg.Prefix, protocol, inDevice, outDevice).Inc()
}

// MessageDropped records a message that the daemon received but did not
// write.
func (m *Metrics) MessageDropped(group groups.Group, reason DropReason) {
	if m == nil {
		return
	}
	m.dropped.WithLabelValues(groupLabel(group), string(reason)).Inc()
}

// KernelOverflow records that the kernel dropped messages for a group.
func (m *Metrics) KernelOverflow(group groups.Group) {
	if m == nil {
		return
	}
	m.kernelOverflow.WithLabelValues(groupLabel(group)).Inc()
}

// MessageWritten records an attempt to write msg to file that started at
// start. If err is not nil, the write failed.
func (m *Metrics) MessageWritten(file string, msg message.Message, start time.Time, err error) {
	if m == nil {
		return
	}

	end := time.Now()
	m.writeDuration.WithLabelValues(file).Observe(end.Sub(start).Seconds())

	if err != nil {
		m.writeErrors.WithLabelValues(file).Inc()
		return
	}
	m.messageLatency.WithLabelValues(file).Observe(end.Sub(msg.ReceivedAt).Seconds())
}

// Reloaded records a reload. If err is not nil, the reload failed.
func (m *Metrics) Reloaded(err error) {
	if m == nil {
		return
	}

	m.reloads.Inc()
	if err != nil {
		m.reloadErrors.Inc()
		return
	}
	m.lastReloadSuccess.SetToCurrentTime()
}

// RegisterQueue registers gauges that report the length and capacity of the
// message queue for an output file.
func (m *Metrics) RegisterQueue(file string, queue chan message.Message) {
	if m == nil {
		return
	}

	labels := prometheus.Labels{"file": file}
	m.registry.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        "queue_messages",
			Help:        "Number of messages waiting to be written to an output file.",
			ConstLabels: labels,
		}, func() float64 { return float64(len(queue)) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        "queue_capacity_messages",
			Help:        "Maximum number of messages waiting to be written to an output file.",
			ConstLabels: labels,
		}, func() float64 { return float64(cap(queue)) }),
	)
}

func groupLabel(g groups.Group) string {
	return strconv.FormatUint(uint64(g), 10)
}
