package exporter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"github.com/showwin/speedtest-go/speedtest"
)

var (
	latency = prometheus.NewDesc(
		prometheus.BuildFQName("speedtest", "", "latency_ms"),
		"Latency to Speedtest Server in seconds",
		[]string{"server_id", "url", "name", "country", "sponsor", "lat", "lon", "distance"},
		nil,
	)
	dl_speed = prometheus.NewDesc(
		prometheus.BuildFQName("speedtest", "", "download_speed_mbps"),
		"Latency to Speedtest Server in seconds",
		[]string{"server_id", "url", "name", "country", "sponsor", "lat", "lon", "distance"},
		nil,
	)
	ul_speed = prometheus.NewDesc(
		prometheus.BuildFQName("speedtest", "", "upload_speed_mbps"),
		"Latency to Speedtest Server in seconds",
		[]string{"server_id", "url", "name", "country", "sponsor", "lat", "lon", "distance"},
		nil,
	)
)

type SpeedtestExporter struct {
	ctx               context.Context
	speedtest         *speedtest.Speedtest
	testTimeout       time.Duration
	testDuration      prometheus.Gauge
	getTargetDuration prometheus.Gauge
}

type Opts struct {
	Ctx         context.Context
	Doer        *http.Client
	TestTimeout time.Duration
}

func New(opts Opts) *SpeedtestExporter {
	if opts.Ctx == nil {
		opts.Ctx = context.Background()
	}
	if opts.Doer == nil {
		opts.Doer = &http.Client{}
	}
	if opts.TestTimeout == 0 {
		opts.TestTimeout = 10 * time.Second
	}
	ret := SpeedtestExporter{
		ctx:         opts.Ctx,
		speedtest:   speedtest.New(speedtest.WithDoer(opts.Doer)),
		testTimeout: opts.TestTimeout,
		testDuration: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "speedtest_test_duration_ms",
			Help: "Duration of speedtest runs in seconds",
		}),
		getTargetDuration: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "speedtest_target_update_duration_ms",
			Help: "Duration of speedtest runs in seconds",
		}),
	}
	return &ret
}

func (e *SpeedtestExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- latency
	ch <- dl_speed
	ch <- ul_speed
	ch <- e.testDuration.Desc()
	ch <- e.getTargetDuration.Desc()
}

func (e *SpeedtestExporter) getServers() (speedtest.Servers, error) {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(e.getTargetDuration.Set))
	defer timer.ObserveDuration()
	infoCtx, cancel := context.WithTimeout(e.ctx, 1500*time.Millisecond)
	defer cancel()

	user, err := e.speedtest.FetchUserInfoContext(infoCtx)
	if err != nil {
		return nil, err
	}
	log.Debug().Interface("user", user).Msg("Fetched user info")

	listCtx, cancel := context.WithTimeout(e.ctx, 1500*time.Millisecond)
	defer cancel()
	serverList, err := e.speedtest.FetchServerListContext(listCtx, user)
	if err != nil {
		return nil, err
	}
	log.Debug().Interface("serverList", serverList).Msg("Fetched server list")

	targets, err := serverList.FindServer([]int{})
	if err != nil {
		return nil, err
	}
	log.Debug().Interface("targets", targets).Msg("Found targets")
	return targets, nil
}

func (e *SpeedtestExporter) RunSpeedtest(targets speedtest.Servers) error {
	for _, srv := range targets {
		timer := prometheus.NewTimer(prometheus.ObserverFunc(e.testDuration.Set))
		defer timer.ObserveDuration()
		ctx, cancel := context.WithTimeout(e.ctx, e.testTimeout)
		defer cancel()
		err := srv.PingTestContext(ctx)
		if err != nil {
			return err
		}
		err = srv.DownloadTestContext(ctx, false)
		if err != nil {
			return err
		}
		err = srv.UploadTestContext(ctx, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *SpeedtestExporter) Collect(ch chan<- prometheus.Metric) {
	log.Debug().Msg("Collecting Speedtest Target")
	targets, err := e.getServers()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get speedtest targets")
		ch <- prometheus.NewInvalidMetric(latency, err)
		ch <- prometheus.NewInvalidMetric(dl_speed, err)
		ch <- prometheus.NewInvalidMetric(ul_speed, err)
		ch <- e.getTargetDuration
		return
	}
	log.Debug().Interface("targets", targets).Msg("Running Speed Test")
	err = e.RunSpeedtest(targets)
	if err != nil {
		log.Error().Err(err).Msg("Failed to run speedtest")
		ch <- prometheus.NewInvalidMetric(latency, err)
		ch <- prometheus.NewInvalidMetric(dl_speed, err)
		ch <- prometheus.NewInvalidMetric(ul_speed, err)
		ch <- e.getTargetDuration
		return
	}
	log.Debug().Interface("results", targets).Msg("Returning Results")
	ch <- e.testDuration
	ch <- e.getTargetDuration
	for _, s := range targets {
		ch <- prometheus.MustNewConstMetric(
			latency,
			prometheus.GaugeValue,
			float64(s.Latency.Microseconds())/1000,
			s.ID, s.URL, s.Name, s.Country, s.Sponsor, s.Lat, s.Lon, fmt.Sprintf("%f", s.Distance),
		)
		ch <- prometheus.MustNewConstMetric(
			dl_speed,
			prometheus.GaugeValue,
			s.DLSpeed,
			s.ID, s.URL, s.Name, s.Country, s.Sponsor, s.Lat, s.Lon, fmt.Sprintf("%f", s.Distance),
		)
		ch <- prometheus.MustNewConstMetric(
			ul_speed,
			prometheus.GaugeValue,
			s.ULSpeed,
			s.ID, s.URL, s.Name, s.Country, s.Sponsor, s.Lat, s.Lon, fmt.Sprintf("%f", s.Distance),
		)
	}
}
