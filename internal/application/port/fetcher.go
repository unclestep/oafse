package port

import (
	"context"
	"errors"

	"oafse/internal/domain/model"
)

var (
	ErrNotText         = errors.New("not text page")
	ErrStatusCodeNotOK = errors.New("status code not ok")
)

type Fetcher interface {
	Fetch(ctx context.Context, u *model.URL) (*FetchResult, error)
}

type FetchResult struct {
	Status      FetchStatus
	ContentType string
	Raw         []byte
}

type FetchStatus int

const (
	FetchImpossible FetchStatus = iota // 200 (body is not HTML), 301, 308, 403, 404, 410, 501 and others
	FetchOK                            // 200, body is HTML
	FetchRetry                         // 408, 429, 500, 502, 503, 504
	FetchManual                        // 400, 401, 411
)

func ClassifyStatus(statusCode int) FetchStatus {
	switch statusCode {
	case 200:
		return FetchOK
	case 408, 429, 500, 502, 503, 504:
		return FetchRetry
	case 400, 401, 411:
		return FetchManual
	default:
		return FetchImpossible
	}
}
