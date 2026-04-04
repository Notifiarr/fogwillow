package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// Entry describes a file or directory returned by the list endpoint.
type Entry struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"isDir"`
	ModTime time.Time `json:"modTime"`
}

// DeleteResult describes the outcome of a delete request.
type DeleteResult struct {
	Deleted int      `json:"deleted"`
	Paths   []string `json:"paths"`
}

// listHandler handles GET /api/list/{path}.
// It returns the contents of all directories matching the (possibly glob) path.
// A final /* is always appended so the response contains entries rather than
// the matched directories themselves — clients should omit the trailing wildcard.
func (a *API) listHandler(resp http.ResponseWriter, r *http.Request) {
	rawPath := mux.Vars(r)["path"]

	pattern, err := a.safePath(rawPath)
	if err != nil {
		writeError(resp, http.StatusBadRequest, err.Error())
		return
	}

	// Append /* to list contents of each matched directory.
	pattern = filepath.Join(pattern, "*")

	matches, err := a.globSafe(pattern)
	if err != nil {
		writeError(resp, http.StatusInternalServerError, err.Error())
		return
	}

	entries := make([]Entry, 0, len(matches))

	for _, match := range matches {
		info, statErr := os.Lstat(match)
		if statErr != nil {
			continue
		}

		relPath := strings.TrimPrefix(match, a.outputPath+string(filepath.Separator))
		entries = append(entries, Entry{
			Path:    relPath,
			Size:    info.Size(),
			IsDir:   info.IsDir(),
			ModTime: info.ModTime().UTC(),
		})
	}

	writeJSON(resp, http.StatusOK, entries)
}

// fileHandler handles GET /api/file/{path} and streams the file contents.
// Wildcards are not permitted; use /api/list/ first to find file paths.
func (a *API) fileHandler(resp http.ResponseWriter, req *http.Request) {
	rawPath := mux.Vars(req)["path"]

	filePath, err := a.safePath(rawPath)
	if err != nil {
		writeError(resp, http.StatusBadRequest, err.Error())
		return
	}

	if strings.ContainsRune(filePath, '*') {
		writeError(resp, http.StatusBadRequest, "wildcards not allowed for file fetch; use /api/list/ first")
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(resp, http.StatusNotFound, "not found: "+filePath)
		} else {
			writeError(resp, http.StatusInternalServerError, "stat: "+err.Error())
		}

		return
	}

	if info.IsDir() {
		writeError(resp, http.StatusBadRequest, "path is a directory; use /api/list/")
		return
	}

	fileHandle, err := os.Open(filePath) //nolint:gosec // path validated against outputPath
	if err != nil {
		writeError(resp, http.StatusInternalServerError, "opening file: "+err.Error())
		return
	}
	defer fileHandle.Close()

	http.ServeContent(resp, req, info.Name(), info.ModTime(), fileHandle)
}

// deleteHandler handles DELETE /api/{path} and removes all files or directories
// matching the path. The path may contain * wildcards in any segment.
func (a *API) deleteHandler(resp http.ResponseWriter, r *http.Request) {
	rawPath := mux.Vars(r)["path"]

	pattern, err := a.safePath(rawPath)
	if err != nil {
		writeError(resp, http.StatusBadRequest, err.Error())
		return
	}

	matches, err := a.resolveForDelete(pattern)
	if err != nil {
		writeError(resp, http.StatusInternalServerError, err.Error())
		return
	}

	deleted := make([]string, 0, len(matches))

	for _, match := range matches {
		err = os.RemoveAll(match)
		if err == nil {
			deleted = append(deleted, strings.TrimPrefix(match, a.outputPath+string(filepath.Separator)))
		}
	}

	writeJSON(resp, http.StatusOK, DeleteResult{Deleted: len(deleted), Paths: deleted})
}

// resolveForDelete returns the filesystem paths to act on for the given pattern.
// When the pattern contains no wildcard it returns the pattern if it exists.
func (a *API) resolveForDelete(pattern string) ([]string, error) {
	if !strings.ContainsRune(pattern, '*') {
		_, err := os.Stat(pattern)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		if err != nil {
			return nil, fmt.Errorf("stat: %w", err)
		}

		return []string{pattern}, nil
	}

	return a.globSafe(pattern)
}

// deleteAllHandler handles DELETE /api/all and removes every entry directly
// inside outputPath, including directories.
func (a *API) deleteAllHandler(resp http.ResponseWriter, _ *http.Request) {
	entries, err := os.ReadDir(a.outputPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		writeError(resp, http.StatusInternalServerError, "reading output path: "+err.Error())
		return
	}

	var count int

	for _, entry := range entries {
		removeErr := os.RemoveAll(filepath.Join(a.outputPath, entry.Name()))
		if removeErr == nil {
			count++
		}
	}

	writeJSON(resp, http.StatusOK, map[string]int{"deleted": count})
}

// writeJSON writes data as a JSON response body with the given status code.
func writeJSON(resp http.ResponseWriter, status int, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		http.Error(resp, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(status)
	_, _ = resp.Write(body)
}

// writeError writes a JSON error response.
func writeError(resp http.ResponseWriter, status int, msg string) {
	writeJSON(resp, status, map[string]string{"error": msg})
}
