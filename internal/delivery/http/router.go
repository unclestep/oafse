package http

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"
)

type Router struct {
	mux *http.ServeMux
}

func NewRouter(queryHandler http.Handler) *Router {
	mux := http.NewServeMux()

	mux.Handle("/swagger/", httpSwagger.WrapHandler)

	mux.Handle("GET /search", queryHandler)

	return &Router{mux: mux}
}

func (r *Router) Handler() http.Handler {
	return r.mux
}
