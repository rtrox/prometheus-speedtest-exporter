package bandwidth_observer

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

type BandwidthObserver struct {
	T                  http.RoundTripper
	bytesUploaded      prometheus.Counter
	bytesDownloaded    prometheus.Counter
	unknownContentSize prometheus.Counter
}

func New(T http.RoundTripper) *BandwidthObserver {
	if T == nil {
		T = http.DefaultTransport
	}
	return &BandwidthObserver{
		T: T,
		bytesUploaded: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "speedtest_bytes_uploaded",
			Help: "Total bytes uploaded",
		}),
		bytesDownloaded: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "speedtest_bytes_downloaded",
			Help: "Total bytes downloaded",
		}),
		unknownContentSize: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "speedtest_unknown_content_size",
			Help: "Total number of times the content size was unknown",
		}),
	}
}

func (b *BandwidthObserver) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := b.T.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if req.ContentLength > 0 {
		if req.Body == nil {
			log.Debug().Msg("Ghosts in the Machine!")
		}
		b.bytesUploaded.Add(float64(req.ContentLength))
	} else if req.Body != nil {
		log.Debug().
			Int64("content-length", req.ContentLength).
			Str("method", req.Method).
			Str("url", req.URL.String()).
			Msg("unknown request content size")
		b.unknownContentSize.Inc()
	}
	if resp.ContentLength > 0 {
		b.bytesDownloaded.Add(float64(resp.ContentLength))
	} else {
		b.unknownContentSize.Inc()
		log.Debug().
			Int64("content-length", req.ContentLength).
			Str("method", req.Method).
			Str("url", req.URL.String()).
			Msg("unknown response content size")
	}
	return resp, err
}

func (b *BandwidthObserver) Describe(ch chan<- *prometheus.Desc) {
	ch <- b.bytesUploaded.Desc()
	ch <- b.bytesDownloaded.Desc()
	ch <- b.unknownContentSize.Desc()
}

func (b *BandwidthObserver) Collect(ch chan<- prometheus.Metric) {
	ch <- b.bytesUploaded
	ch <- b.bytesDownloaded
	ch <- b.unknownContentSize
}
