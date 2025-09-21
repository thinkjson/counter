package main

import (
	_ "embed"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed ui/index.html
var index []byte

//go:embed ui/css/vendor.css
var vendor []byte

//go:embed ui/css/style.css
var css []byte

//go:embed ui/js/app.js
var js []byte

type uiResource struct{}

func (rs uiResource) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", rs.Index)
	r.Get("/css/vendor.css", rs.Vendor)
	r.Get("/css/style.css", rs.CSS)
	r.Get("/js/app.js", rs.JS)
	return r
}

func (rs uiResource) Index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write(index)
}

func (rs uiResource) Vendor(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Write(vendor)
}

func (rs uiResource) CSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Write(css)
}

func (rs uiResource) JS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript")
	w.Write(js)
}
