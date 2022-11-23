package exporter

import (
	"context"
	"io"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
)

func init() {
	// log.Logger = zerolog.New(io.Discard)
}

func TestDescribe(t *testing.T) {
	assert := assert.New(t)
	e := New(Opts{})

	ch := make(chan *prometheus.Desc)
	received := 0
	go func() {
		assert.NotPanics(func() {
			e.Describe(ch)
		})
		close(ch)
	}()
	for elem := range ch {
		assert.NotEqual(&prometheus.Desc{}, elem)
		received++
	}
	assert.GreaterOrEqual(received, 5)
}

func TestCollect(t *testing.T) {
	assert := assert.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	e := New(Opts{
		Doer: NewTestClient(),
		Ctx:  ctx,
	})
	e.UpdateResults()
	ch := make(chan prometheus.Metric)
	received := 0
	go func() {
		assert.NotPanics(func() {
			e.Collect(ch)
		})
		close(ch)
	}()

	for elem := range ch {
		metric := &dto.Metric{}
		elem.Write(metric)

		assert.NotEqual(0, metric.GetGauge().GetValue())
		received++
	}
	assert.GreaterOrEqual(received, 5)

}

/* Example Output:
# HELP speedtest_download_speed_mbps Latency to Speedtest Server in seconds
# TYPE speedtest_download_speed_mbps gauge
speedtest_download_speed_mbps{country="United States",distance="1.0",lat="1.0",lon="-1.0",name="Anytown, USA",server_id="1",sponsor="Dat Sponsor Doh",url="http://speedtest.example.net:8080/speedtest/upload.php"} 716.7810615213976
# HELP speedtest_exporter_info Info about this speedtest-exporter
# TYPE speedtest_exporter_info gauge
speedtest_exporter_info{app_name="speedtest-exporter",app_version="x.x.x"} 1
# HELP speedtest_latency_ms Latency to Speedtest Server in seconds
# TYPE speedtest_latency_ms gauge
speedtest_latency_ms{country="United States",distance="1.0",lat="1.0",lon="-1.0",name="Anytown, USA",server_id="1",sponsor="Dat Sponsor Doh",url="http://speedtest.example.net:8080/speedtest/upload.php"} 4.13
# HELP speedtest_target_update_duration_ms Duration of speedtest runs in seconds
# TYPE speedtest_target_update_duration_ms gauge
speedtest_target_update_duration_ms 0.206249213
# HELP speedtest_test_duration_ms Duration of speedtest runs in seconds
# TYPE speedtest_test_duration_ms gauge
speedtest_test_duration_ms 6.257747472
# HELP speedtest_upload_speed_mbps Latency to Speedtest Server in seconds
# TYPE speedtest_upload_speed_mbps gauge
speedtest_upload_speed_mbps{country="United States",distance="1.0",lat="1.0",lon="-1.0",name="Anytown, USA",server_id="1",sponsor="Dat Sponsor Doh",url="http://speedtest.example.net:8080/speedtest/upload.php"} 724.4910836862521
*/

func TestAllMetricsPopulated(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	e := New(Opts{
		Doer: NewTestClient(),
		Ctx:  ctx,
	})
	e.UpdateResults()
	reg := prometheus.NewRegistry()
	reg.MustRegister(e)

	srv := httptest.NewServer(promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	defer srv.Close()

	c := srv.Client()
	resp, err := c.Get(srv.URL)
	require.Nil(err)
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	require.Nil(err)

	tests := []struct {
		desc  string
		match *regexp.Regexp
	}{
		{"speed_test_download_speed_desc", regexp.MustCompile(`(?m)^# HELP speedtest_download_speed_mbps .+$`)},
		{"speed_test_download_speed", regexp.MustCompile(`(?m)^speedtest_download_speed_mbps{country=".+",distance="[0-9\.]+",lat="[0-9\.\-]+",lon="[0-9\.\-]+",name=".+",server_id="[0-9]+",sponsor=".+",url=".+"} [0-9\.]+$`)},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert.True(tt.match.Match(buf), "Regex %s didn't match a line! buf: %s", tt.match.String(), string(buf))
		})
	}
}
