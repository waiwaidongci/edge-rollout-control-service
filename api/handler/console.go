package handler

import (
	"embed"
	"net/http"
)

//go:embed console/*
var consoleFiles embed.FS

func (h *Handler) console(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/console" {
		http.Redirect(w, r, "/console/", http.StatusTemporaryRedirect)
		return
	}
	http.FileServer(http.FS(consoleFiles)).ServeHTTP(w, r)
}
