package handler

import (
	"net/http"
	"strconv"

	"oafse/internal/application/port"
	j "oafse/internal/delivery/http/json"
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
	query := r.URL.Query().Get("q")
	limitStr := r.URL.Query().Get("limit")

	if query == "" {
		j.WriteError(w, "missing query parameter 'q'", http.StatusBadRequest)
		return
	}

	var limit int
	if limitStr == "" {
		limit = 10
	} else {
		limitTmp, err := strconv.Atoi(limitStr)
		if err != nil {
			j.WriteError(w, "limit should be a number", http.StatusBadRequest)
			return
		}
		limit = limitTmp
	}

	pages, err := h.uc.Execute(r.Context(), query, limit)
	if err != nil {
		j.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	j.WriteJSON(w, j.ToSearchResponse(pages), http.StatusOK)
}
