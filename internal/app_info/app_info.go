package app_info

import (
	"github.com/prometheus/client_golang/prometheus"
)

var ()

type Opts struct {
	Namespace string
	Name      string
	Version   string
}

func GaugeFunc(opts Opts) prometheus.GaugeFunc {
	if opts.Namespace == "" {
		opts.Namespace = opts.Name
	}
	infoMetricOpts := prometheus.GaugeOpts{
		Namespace: opts.Namespace,
		Name:      "info",
		Help:      "Info about this speedtest-exporter",
		ConstLabels: prometheus.Labels{
			"app_name":    opts.Name,
			"app_version": opts.Version,
		},
	}
	return prometheus.NewGaugeFunc(
		infoMetricOpts,
		func() float64 { return 1 },
	)
}
