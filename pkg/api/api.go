// Package api provides an HTTP API for listing, fetching, and deleting files
// written by the UDP ingest path. All operations are scoped to output_path.
package api

import (
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
)

// API handles HTTP routes for the file management API.
type API struct {
	outputPath string
	password   string
}

// New returns an API instance rooted at outputPath.
// password is compared against the X-Api-Key request header; an empty string disables auth.
func New(outputPath, password string) *API {
	return &API{
		outputPath: filepath.Clean(outputPath),
		password:   password,
	}
}

// Register mounts all API routes onto smuthe server muxx and is intended to be passed as the
// register callback to httpserver.New.
func (a *API) Register(smx *http.ServeMux) {
	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Use(a.authenticate)

	apiRouter.HandleFunc("/list/{path:.+}", a.listHandler).Methods(http.MethodGet)
	apiRouter.HandleFunc("/file/{path:.+}", a.fileHandler).Methods(http.MethodGet)
	apiRouter.HandleFunc("/all", a.deleteAllHandler).Methods(http.MethodDelete)
	apiRouter.HandleFunc("/{path:.+}", a.deleteHandler).Methods(http.MethodDelete)

	smx.Handle("/api/", router)
	smx.Handle("/api", router)
}
