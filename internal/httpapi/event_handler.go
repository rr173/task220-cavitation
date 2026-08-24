package httpapi

import (
	"net/http"
)

func (s *Server) analyzeTrial(w http.ResponseWriter, r *http.Request) {
	trialID := r.PathValue("id")
	trial, err := s.app.Trials.Get(trialID)
	if err != nil {
		writeError(w, err)
		return
	}
	noisyChannels, err := s.app.Segments.CountNoisyChannels(trialID)
	if err != nil {
		writeError(w, err)
		return
	}
	cfg := s.app.Threshold().Current()
	res, err := s.app.Events.Analyze(trial, cfg, noisyChannels)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.app.Events.ListEvents(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) getEvent(w http.ResponseWriter, r *http.Request) {
	ev, err := s.app.Events.GetEvent(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ev)
}

type rejectEventReq struct {
	Reason string `json:"reason"`
}

func (s *Server) rejectEvent(w http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("id")
	ev, err := s.app.Events.GetEvent(eventID)
	if err != nil {
		writeError(w, err)
		return
	}
	trial, err := s.app.Trials.Get(ev.TrialID)
	if err != nil {
		writeError(w, err)
		return
	}
	var req rejectEventReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if err := s.app.Events.Reject(trial, eventID, req.Reason); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected", "id": eventID})
}

type advanceEventReq struct {
	Stage string `json:"stage"`
}

func (s *Server) advanceEvent(w http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("id")
	ev, err := s.app.Events.GetEvent(eventID)
	if err != nil {
		writeError(w, err)
		return
	}
	trial, err := s.app.Trials.Get(ev.TrialID)
	if err != nil {
		writeError(w, err)
		return
	}
	var req advanceEventReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if err := s.app.Events.Advance(trial, eventID, req.Stage); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "advanced", "id": eventID})
}

func (s *Server) listFeatures(w http.ResponseWriter, r *http.Request) {
	features, err := s.app.Events.ListFeatures(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, features)
}
