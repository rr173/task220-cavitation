package httpapi

import (
	"net/http"
)

type ingestSegmentReq struct {
	Channel     int       `json:"channel"`
	SampleRate  float64   `json:"sample_rate_hz"`
	StartTimeMs int64     `json:"start_time_ms"`
	Samples     []float64 `json:"samples"`
}

func (s *Server) ingestSegment(w http.ResponseWriter, r *http.Request) {
	trialID := r.PathValue("id")
	trial, err := s.app.Trials.Get(trialID)
	if err != nil {
		writeError(w, err)
		return
	}
	var req ingestSegmentReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	res, err := s.app.Segments.Ingest(trial, req.Channel, req.SampleRate, req.StartTimeMs, req.Samples)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) listSegments(w http.ResponseWriter, r *http.Request) {
	segs, err := s.app.Segments.ListSegments(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, segs)
}

func (s *Server) markNoisy(w http.ResponseWriter, r *http.Request) {
	if err := s.app.Segments.MarkNoisy(r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "marked_noisy"})
}

func (s *Server) calibrateChannels(w http.ResponseWriter, r *http.Request) {
	trial, err := s.app.Trials.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	delays, err := s.app.Segments.CalibrateChannels(trial)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, delays)
}

func (s *Server) listDelays(w http.ResponseWriter, r *http.Request) {
	trialID := r.PathValue("id")
	delays, err := s.app.Segments.ListDelays(trialID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, delays)
}
