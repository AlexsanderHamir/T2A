package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/logctx"
)

type jsonCodedErrorBody struct {
	Error     string `json:"error"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func (h *Handler) gitService() gitwork.Service {
	if h.git != nil {
		return h.git
	}
	return gitwork.New()
}

func gitErrHTTP(err error) (status int, code, msg string) {
	status = http.StatusInternalServerError
	msg = "internal server error"
	if err == nil {
		return status, code, msg
	}
	if c := domain.GitErrCode(err); c != "" {
		code = c
		msg = err.Error()
		switch c {
		case domain.GitCodeRepositoryNotFound, domain.GitCodeWorktreeNotFound, domain.GitCodeBranchNotFound:
			status = http.StatusNotFound
		case domain.GitCodeNotARepository, domain.GitCodePathExists, domain.GitCodeBranchExists,
			domain.GitCodeBranchCheckedOut, domain.GitCodeHasRunningTask, domain.GitCodeDuplicate:
			status = http.StatusConflict
		default:
			status = http.StatusBadRequest
		}
		return status, code, msg
	}
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "", "not found"
	case errors.Is(err, domain.ErrInvalidInput):
		return http.StatusBadRequest, "", invalidInputDetail(err)
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, "", conflictDetail(err)
	default:
		return status, code, msg
	}
}

func writeGitStoreError(w http.ResponseWriter, r *http.Request, op string, err error) {
	status, code, msg := gitErrHTTP(err)
	if code != "" {
		writeJSONCodedError(w, r, op, status, code, msg)
		return
	}
	if status >= 500 {
		writeJSONError(w, r, op, status, msg)
		return
	}
	writeJSONError(w, r, op, status, msg)
}

func writeJSONCodedError(w http.ResponseWriter, r *http.Request, op string, status int, code, msg string) {
	setJSONHeaders(w)
	w.WriteHeader(status)
	body := jsonCodedErrorBody{Error: msg, Code: code}
	if r != nil {
		body.RequestID = logctx.RequestIDFromContext(r.Context())
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(body)
}

func parseGitProjectID(r *http.Request) (string, error) {
	return parseTaskPathID(r.PathValue("id"))
}
