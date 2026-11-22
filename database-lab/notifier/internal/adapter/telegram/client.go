package telegram

import (
	"net/http"
)

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

var _ httpClient = (*http.Client)(nil)
