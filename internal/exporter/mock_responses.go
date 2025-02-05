package exporter

import (
	"bytes"
	"io"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
)

// RoundTripFunc .
type roundTripFunc func(req *http.Request) *http.Response

// RoundTrip .
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient() *http.Client {
	return &http.Client{
		Transport: roundTripFunc(speedtestFunc),
	}
}

func speedtestFunc(req *http.Request) *http.Response {
	var ret *http.Response
	switch req.URL.Path {
	case "/speedtest-config.php":

		ret = &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body: io.NopCloser(bytes.NewBufferString(speedtestUserConfigMock)),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}
	case "/api/js/servers":
		ret = &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body: io.NopCloser(bytes.NewBufferString(speedtestServerListMock)),
			// Must be set to non-nil value or it panics
			Header:        make(http.Header),
			ContentLength: 100,
		}
	case "/speedtest/upload.php":
		ret = &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body: io.NopCloser(bytes.NewBufferString("OK")),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}
	case "/speedtest/latency.txt":
		ret = &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body: io.NopCloser(bytes.NewBufferString("test=test")),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}
	case "/speedtest/random2500x2500.jpg":
		fileBytes, err := os.ReadFile("test2500.jpg")
		if err != nil {
			log.Error().Str("url", req.URL.String()).Err(err).Msg("Could not read file")
		}
		ret = &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body: io.NopCloser(bytes.NewBuffer(fileBytes)),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}
	case "/speedtest/random750x750.jpg":
		fileBytes, err := os.ReadFile("test750.jpg")
		if err != nil {
			log.Error().Str("url", req.URL.String()).Err(err).Msg("Could not read file")
		}
		ret = &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body: io.NopCloser(bytes.NewBuffer(fileBytes)),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}
	case "/speedtest/random1000x1000.jpg":
		fileBytes, err := os.ReadFile("test1000.jpg")
		if err != nil {
			log.Error().Str("url", req.URL.String()).Err(err).Msg("Could not read file")
		}
		ret = &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body: io.NopCloser(bytes.NewBuffer(fileBytes)),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}
	default:
		log.Error().Str("url", req.URL.String()).Msg("Unhandled URL")
		ret = &http.Response{
			StatusCode: 200,
			// Send response to be tested
			Body: io.NopCloser(bytes.NewBufferString("Error: Unhandled URL")),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}
	}
	return ret
}

var (
	speedtestUserConfigMock = `
	<?xml version="1.0" encoding="UTF-8"?>
	<settings>
	<client ip="1.2.3.4" lat="1.1" lon="-1.1" isp="Dat Sponsor Doh" isprating="1" rating="0" ispdlavg="0" ispulavg="0" loggedin="0" country="US" />
	</settings>
`

	speedtestServerListMock = `[
  {
    "url": "http://speedtest.example.net:8080/speedtest/upload.php",
    "lat": "1.00",
    "lon": "-1.0",
    "distance": 5,
    "name": "Anytown, USA",
    "country": "United States",
    "cc": "US",
    "sponsor": "Dat Sponsor Doh",
    "id": "1",
    "preferred": 0,
    "https_functional": 1,
    "host": "speedtest1.example.net:8080",
    "force_ping_select": 1
  }
]`
)
