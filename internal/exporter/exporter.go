package exporter

import (
	"context"
	"fmt"
	"net/http"
	"sync"
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

type ResultCache struct {
	results speedtest.Servers
	mut     sync.RWMutex
}

func (r *ResultCache) Set(servers speedtest.Servers) {
	r.mut.Lock()
	defer r.mut.Unlock()
	r.results = servers
}

func (r *ResultCache) Get() speedtest.Servers {
	r.mut.RLock()
	defer r.mut.RUnlock()
	return r.results
}

func NewResultCache() *ResultCache {
	return &ResultCache{
		results: speedtest.Servers{},
		mut:     sync.RWMutex{},
	}
}

type SpeedtestExporter struct {
	ctx          context.Context
	done         chan struct{}
	speedtest    *speedtest.Speedtest
	cache        *ResultCache
	testInterval time.Duration
	testTimeout  time.Duration
	savingMode   bool

	testDuration      prometheus.Gauge
	getTargetDuration prometheus.Gauge
	testErrors        prometheus.Counter
	testsRun          prometheus.Counter
}

type Opts struct {
	Ctx          context.Context
	Doer         *http.Client
	TestTimeout  time.Duration
	TestInterval time.Duration
	SavingMode   bool
}

func New(opts Opts) *SpeedtestExporter {
	if opts.Ctx == nil {
		opts.Ctx = context.Background()
	}
	if opts.Doer == nil {
		opts.Doer = http.DefaultClient
	}
	if opts.TestTimeout == 0 {
		opts.TestTimeout = 1 * time.Minute
	}
	if opts.TestInterval == 0 {
		opts.TestInterval = 1 * time.Hour
	}
	ret := SpeedtestExporter{
		ctx:          opts.Ctx,
		speedtest:    speedtest.New(speedtest.WithDoer(opts.Doer)),
		cache:        NewResultCache(),
		testTimeout:  opts.TestTimeout,
		testInterval: opts.TestInterval,
		savingMode:   opts.SavingMode,
		testDuration: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "speedtest_test_duration_ms",
			Help: "Duration of speedtest runs in seconds",
		}),
		getTargetDuration: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "speedtest_target_update_duration_ms",
			Help: "Duration of speedtest runs in seconds",
		}),
		testErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "speedtest_test_errors_total",
			Help: "Number of errors during speedtest runs",
		}),
		testsRun: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "speedtest_tests_run_total",
			Help: "Number of speedtest runs",
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
	ch <- e.testErrors.Desc()
}

func (e *SpeedtestExporter) Collect(ch chan<- prometheus.Metric) {
	ch <- e.testDuration
	ch <- e.getTargetDuration
	for _, s := range e.cache.Get() {
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
		err = srv.DownloadTestContext(ctx, e.savingMode)
		if err != nil {
			return err
		}
		err = srv.UploadTestContext(ctx, e.savingMode)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *SpeedtestExporter) UpdateResults() {
	log.Debug().Msg("Collecting Speedtest Target")
	targets, err := e.getServers()
	if err != nil {
		e.cache.Set(speedtest.Servers{})
		log.Error().Err(err).Msg("Failed to get speedtest targets")
		e.testErrors.Inc()
		return
	}
	log.Debug().Interface("targets", targets).Msg("Running Speed Test")
	err = e.RunSpeedtest(targets)
	if err != nil {
		e.cache.Set(speedtest.Servers{})
		log.Error().Err(err).Msg("Failed to run speedtest")
		e.testErrors.Inc()
		return
	}
	log.Debug().Interface("results", targets).Msg("Returning Results")
	e.cache.Set(targets)
}

func (e *SpeedtestExporter) TestLoop() {
	e.UpdateResults()
	t := time.NewTicker(e.testInterval)
	for {
		select {
		case <-e.ctx.Done():
			close(e.done)
			return
		case <-t.C:
			e.UpdateResults()
			e.testsRun.Inc()
		}
	}
}
