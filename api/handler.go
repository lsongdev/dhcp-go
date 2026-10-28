package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/lsongdev/dhcp-go/server"
	"github.com/lsongdev/dhcp-go/store"
)

// Handler serves the API. Successful responses are {"data": ...}; failures
// are {"error": {"code": ..., "message": ...}} with a matching status.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		st, err := s.Status(r.Context())
		respond(w, http.StatusOK, st, err)
	})
	mux.HandleFunc("GET /config", func(w http.ResponseWriter, r *http.Request) {
		respond(w, http.StatusOK, s.Config(), nil)
	})
	mux.HandleFunc("PUT /config", func(w http.ResponseWriter, r *http.Request) {
		var cfg server.Config
		if decode(w, r, &cfg) {
			st, err := s.SetConfig(r.Context(), cfg)
			respond(w, http.StatusOK, st, err)
		}
	})
	mux.HandleFunc("GET /interfaces", func(w http.ResponseWriter, r *http.Request) {
		list, err := s.Interfaces()
		respond(w, http.StatusOK, list, err)
	})
	mux.HandleFunc("GET /leases", func(w http.ResponseWriter, r *http.Request) {
		list, err := s.Leases(r.Context())
		respond(w, http.StatusOK, list, err)
	})
	mux.HandleFunc("DELETE /leases/{ip}", func(w http.ResponseWriter, r *http.Request) {
		ip, err := parseIP(r.PathValue("ip"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		respond(w, http.StatusNoContent, nil, s.RemoveLease(ip))
	})
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		respond(w, http.StatusOK, s.Events(), nil)
	})
	mux.HandleFunc("GET /reservations", func(w http.ResponseWriter, r *http.Request) {
		list, err := s.Reservations(r.Context())
		respond(w, http.StatusOK, list, err)
	})
	mux.HandleFunc("POST /reservations", func(w http.ResponseWriter, r *http.Request) {
		var in reservationInput
		if decode(w, r, &in) {
			res, err := s.CreateReservation(r.Context(), in.reservation())
			respond(w, http.StatusCreated, res, err)
		}
	})
	mux.HandleFunc("PUT /reservations/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in reservationInput
		if id, ok := pathID(w, r); ok && decode(w, r, &in) {
			res, err := s.UpdateReservation(r.Context(), id, in.reservation())
			respond(w, http.StatusOK, res, err)
		}
	})
	mux.HandleFunc("DELETE /reservations/{id}", func(w http.ResponseWriter, r *http.Request) {
		if id, ok := pathID(w, r); ok {
			respond(w, http.StatusNoContent, nil, s.DeleteReservation(r.Context(), id))
		}
	})
	mux.HandleFunc("GET /summary", func(w http.ResponseWriter, r *http.Request) {
		sum, err := s.Summary(r.Context())
		respond(w, http.StatusOK, sum, err)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such API endpoint")
	})
	return mux
}

// reservationInput creates or updates a reservation.
type reservationInput struct {
	MAC         string `json:"mac"`
	IP          string `json:"ip"`
	Hostname    string `json:"hostname"`
	Description string `json:"description"`
}

func (in reservationInput) reservation() server.Reservation {
	return server.Reservation{MAC: in.MAC, IP: in.IP, Hostname: in.Hostname, Description: in.Description}
}

func respond(w http.ResponseWriter, status int, data any, err error) {
	switch {
	case err == nil:
	case errors.Is(err, server.ErrInvalid):
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", err.Error())
		return
	default:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if status == http.StatusNoContent || data == nil {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, map[string]any{"data": data})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid id")
		return 0, false
	}
	return id, true
}
