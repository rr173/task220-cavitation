package httpapi

import (
	"net/http"
)

type createTrialReq struct {
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	ShaftSpeedRPM    float64 `json:"shaft_speed_rpm"`
	InflowPressureKPa float64 `json:"inflow_pressure_kpa"`
	ReferenceChannel int     `json:"reference_channel"`
}

func (s *Server) createTrial(w http.ResponseWriter, r *http.Request) {
	var req createTrialReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	t, err := s.app.Trials.Create(req.Name, req.Description, req.ShaftSpeedRPM, req.InflowPressureKPa, req.ReferenceChannel)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) listTrials(w http.ResponseWriter, r *http.Request) {
	trials, err := s.app.Trials.List()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, trials)
}

func (s *Server) getTrial(w http.ResponseWriter, r *http.Request) {
	t, err := s.app.Trials.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) startAcquisition(w http.ResponseWriter, r *http.Request) {
	t, err := s.app.Trials.StartAcquisition(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) finishAcquisition(w http.ResponseWriter, r *http.Request) {
	t, err := s.app.Trials.FinishAcquisition(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) confirmTrial(w http.ResponseWriter, r *http.Request) {
	t, err := s.app.Trials.Confirm(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}
