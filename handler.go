package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// Handler can handle apps HTTP requests.
type Handler struct {
	service *Service
}

// ServeHTTP is the main handler.
// It knows how to handle "GET /" requests, and returns 404 for the rest.
func (s *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet || req.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	s.GetContent(w, req)
}

// GetContent returns a list of content items for the `count` and `offset` query parameters.
func (h *Handler) GetContent(w http.ResponseWriter, req *http.Request) {
	count, offset, err := h.validateContentReq(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	items, err := h.service.GetContent(
		req.Context(),
		h.getIP(req),
		count,
		offset,
	)
	if err != nil {
		h.handleServerErr(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(items); err != nil {
		log.Printf("encoding response to http writer: %v", err)
	}
}

func (h *Handler) validateContentReq(req *http.Request) (count int, offset int, err error) {
	v, err := h.getIntParam("count", true, false, req)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid count parameter: %w", err)
	}
	count = v

	v, err = h.getIntParam("offset", false, true, req)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid offset parameter: %w", err)
	}
	offset = v

	return count, offset, nil
}

func (h *Handler) getIntParam(name string, required bool, allowZero bool, req *http.Request) (int, error) {
	s := req.URL.Query().Get(name)
	if s == "" {
		if required {
			return 0, errors.New("is empty")
		}
		return 0, nil
	}

	v, err := strconv.ParseInt(s, 10, 64)
	switch {
	case err != nil:
		return 0, errors.New("must be an integer")
	case v <= 0 && !allowZero:
		return 0, errors.New("must be positive or zero")
	case v < 0:
		return 0, errors.New("must be positive")
	}
	return int(v), nil
}

func (h *Handler) handleServerErr(w http.ResponseWriter, err error) {
	// We don't want to uncover error details to the client...
	http.Error(w, "internal server error", http.StatusInternalServerError)

	// ... but we want to have all the details in the logs.
	log.Printf("http server error: %v", err)
}

func (h *Handler) getIP(req *http.Request) string {
	v := req.RemoteAddr
	vs := strings.Split(v, ":")
	return vs[0]
}
