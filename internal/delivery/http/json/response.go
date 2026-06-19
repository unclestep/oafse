package json

import (
	"encoding/json"
	"log"
	"net/http"
)

type SearchResponse struct {
	Pages []*Page `json:"pages"`
}

type Page struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type CrawlResponse struct {
	Status string `json:"status"`
	URL    string `json:"url"`
}

func WriteJSON(w http.ResponseWriter, v any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[ERR] write response json: %s", err.Error())
	}
}

func WriteError(w http.ResponseWriter, msg string, status int) {
	WriteJSON(w, ErrorResponse{Error: msg}, status)
}
