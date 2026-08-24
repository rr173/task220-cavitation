package httpapi

import (
	"net/http"
)

func (s *Server) publishPackage(w http.ResponseWriter, r *http.Request) {
	pkg, err := s.app.Packages.Publish(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, pkg)
}

func (s *Server) listPackages(w http.ResponseWriter, r *http.Request) {
	pkgs, err := s.app.Packages.ListPackages(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pkgs)
}

func (s *Server) getPackage(w http.ResponseWriter, r *http.Request) {
	pkg, err := s.app.Packages.GetPackage(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pkg)
}

func (s *Server) listThresholds(w http.ResponseWriter, r *http.Request) {
	thrs, err := s.app.Packages.ListThresholds()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, thrs)
}

type addThresholdReq struct {
	GapRatioThreshold float64 `json:"gap_ratio_threshold"`
	EnergyFloor       float64 `json:"energy_floor"`
	ConfirmWindows    int     `json:"confirm_windows"`
}

func (s *Server) addThreshold(w http.ResponseWriter, r *http.Request) {
	var req addThresholdReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	cfg, err := s.app.Packages.AddThreshold(req.GapRatioThreshold, req.EnergyFloor, req.ConfirmWindows)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cfg)
}
