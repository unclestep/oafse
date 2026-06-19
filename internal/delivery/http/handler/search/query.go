package search

import (
	"net/http"
	"strconv"
	"time"

	"oafse/internal/application/port"
	j "oafse/internal/delivery/http/json"
	"oafse/internal/infrastructure/metrics"
)

type QueryHandler struct {
	uc port.QueryUseCase
}

func NewQueryHandler(uc port.QueryUseCase) *QueryHandler {
	return &QueryHandler{
		uc: uc,
	}
}

// Search performs semantic similarity search over indexed pages.
//
// @Summary      Semantic search
// @Description  Embeds the query via SBERT and returns the most similar pages ranked by dot product.
// @Tags         search
// @Produce      json
// @Param        q      query    string  true   "Search query"           example(machine learning)
// @Param        limit  query    int     false  "Max results (default 10)" example(10)
// @Success      200    {object} json.SearchResponse
// @Failure      400    {object} json.ErrorResponse
// @Failure      500    {object} json.ErrorResponse
// @Router       /search [get]
func (h *QueryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		metrics.SearchDuration.Observe(time.Since(start).Seconds())
	}()

	writeError := func(msg string, status int) {
		metrics.SearchRequests.WithLabelValues(strconv.Itoa(status)).Inc()
		j.WriteError(w, msg, status)
	}

	query := r.URL.Query().Get("q")
	limitStr := r.URL.Query().Get("limit")

	if query == "" {
		writeError("missing query parameter 'q'", http.StatusBadRequest)
		return
	}

	var limit int
	if limitStr == "" {
		limit = 10
	} else {
		limitTmp, err := strconv.Atoi(limitStr)
		if err != nil || limitTmp <= 0 {
			writeError("limit should be a natural number", http.StatusBadRequest)
			return
		}
		limit = limitTmp
	}

	pages, err := h.uc.Execute(r.Context(), query, limit)
	if err != nil {
		writeError(err.Error(), http.StatusInternalServerError)
		return
	}

	if len(pages) == 0 {
		metrics.EmptySearchResults.Inc()
	}

	metrics.SearchRequests.WithLabelValues(strconv.Itoa(http.StatusOK)).Inc()
	j.WriteJSON(w, j.ToSearchResponse(pages), http.StatusOK)
}
