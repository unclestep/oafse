package handler

import (
	"net/http"

	"oafse/internal/application/port"
	j "oafse/internal/delivery/http/json"
	"oafse/internal/domain/model"
)

type CrawlHandler struct {
	urlRepo port.URLRepo
}

func NewCrawlHandler(urlRepo port.URLRepo) *CrawlHandler {
	return &CrawlHandler{urlRepo: urlRepo}
}

// Seed adds a new site for the crawler to start from.
//
// @Summary      Seed a crawl
// @Description  Adds url to the crawl queue so the crawler starts discovering pages from it. reindex=true wipes the entire shared crawl cache first - it is not scoped to just this url's site.
// @Tags         crawl
// @Produce      json
// @Param        url      query    string  true   "Site URL to start crawling from"          example(https://books.toscrape.com)
// @Param        reindex  query    bool    false   "Wipe the entire crawl cache before seeding" example(false)
// @Success      202      {object} json.CrawlResponse
// @Failure      400      {object} json.ErrorResponse
// @Failure      500      {object} json.ErrorResponse
// @Router       /crawl [post]
func (h *CrawlHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		j.WriteError(w, "missing query parameter 'url'", http.StatusBadRequest)
		return
	}

	baseURL, err := model.NormalizeStartURL(rawURL)
	if err != nil {
		j.WriteError(w, "invalid url: "+err.Error(), http.StatusBadRequest)
		return
	}

	if r.URL.Query().Get("reindex") == "true" {
		if err := h.urlRepo.ResetCrawlCache(r.Context()); err != nil {
			j.WriteError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := h.urlRepo.Start(r.Context(), baseURL); err != nil {
		j.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	j.WriteJSON(w, j.CrawlResponse{Status: "queued", URL: baseURL}, http.StatusAccepted)
}
