package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
)

const (
	// FileMode is the mode for the file.
	FileMode = 0o640
	// DirMode is the mode for the directory.
	DirMode = 0o750
)

// statusError carries an HTTP status for errors returned to clients as JSON.
type statusError struct {
	Code int
	Msg  string
}

func (e *statusError) Error() string {
	return e.Msg
}

// uploadHandler handles PUT /api/file/{path} and writes the request body to that path.
// Parent directories are created as needed. Wildcards are not permitted.
func (a *API) uploadHandler(resp http.ResponseWriter, req *http.Request) {
	var se *statusError
	switch relPath, created, err := a.writeUploadedFile(mux.Vars(req)["path"], req.Body); {
	case errors.As(err, &se):
		writeError(resp, se.Code, se.Msg)
	case err != nil:
		writeError(resp, http.StatusInternalServerError, err.Error())
	case created:
		writeJSON(resp, http.StatusCreated, map[string]string{"path": relPath})
	default:
		writeJSON(resp, http.StatusOK, map[string]string{"path": relPath})
	}
}

func (a *API) validatePutFilePath(rawPath string) (string, bool, error) {
	path, err := a.safePath(rawPath)
	if err != nil {
		return "", false, &statusError{Code: http.StatusBadRequest, Msg: err.Error()}
	}

	if strings.ContainsRune(path, '*') {
		return "", false, &statusError{
			Code: http.StatusBadRequest,
			Msg:  "wildcards not allowed for file upload",
		}
	}

	info, statErr := os.Stat(path)
	if statErr == nil && info.IsDir() {
		return "", false, &statusError{Code: http.StatusBadRequest, Msg: "path is a directory; use /api/list/"}
	}

	created := errors.Is(statErr, os.ErrNotExist)
	if statErr != nil && !created {
		return "", false, fmt.Errorf("stat: %w", statErr)
	}

	return path, created, nil
}

func commitUploadedFile(tmpPath, destPath string, created bool) error {
	err := os.Chmod(tmpPath, FileMode) //nolint:gosec // temp path from CreateTemp under validated directory
	if err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	if !created {
		err = os.Remove(destPath) //nolint:gosec // destPath returned from safePath under outputPath
		if err != nil {
			return fmt.Errorf("removing existing file: %w", err)
		}
	}

	err = os.Rename(tmpPath, destPath) //nolint:gosec // paths validated; atomic replace after write
	if err != nil {
		return fmt.Errorf("renaming file: %w", err)
	}

	return nil
}

func (a *API) writeUploadedFile(rawPath string, body io.Reader) (string, bool, error) {
	filePath, created, err := a.validatePutFilePath(rawPath)
	if err != nil {
		return "", false, err
	}

	dir := filepath.Dir(filePath)

	err = os.MkdirAll(dir, DirMode) //nolint:gosec // dir is filepath.Dir of a path from safePath under outputPath
	if err != nil {
		return "", false, fmt.Errorf("creating parent directories: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, ".fogwillow-upload-*")
	if err != nil {
		return "", false, fmt.Errorf("temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	cleanupTmp := true

	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpPath) //nolint:gosec // temp file created in this function under validated dir
		}
	}()

	_, err = io.Copy(tmpFile, body)
	if err != nil {
		_ = tmpFile.Close()
		return "", false, fmt.Errorf("writing body: %w", err)
	}

	err = tmpFile.Close()
	if err != nil {
		return "", false, fmt.Errorf("closing temp file: %w", err)
	}

	err = commitUploadedFile(tmpPath, filePath, created)
	if err != nil {
		return "", false, err
	}

	cleanupTmp = false

	return strings.TrimPrefix(filePath, a.outputPath+string(filepath.Separator)), created, nil
}
