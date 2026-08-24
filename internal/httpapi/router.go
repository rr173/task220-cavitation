// Package httpapi 提供 HTTP 层：路由注册、请求解析与 JSON 响应。
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/service"
)

// Server HTTP 服务器。
type Server struct {
	app *service.App
}

// New 构造 HTTP 服务器。
func New(app *service.App) *Server { return &Server{app: app} }

// Handler 注册全部路由并返回处理器。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// 试验生命周期。
	mux.HandleFunc("POST /api/trials", s.createTrial)
	mux.HandleFunc("GET /api/trials", s.listTrials)
	mux.HandleFunc("GET /api/trials/{id}", s.getTrial)
	mux.HandleFunc("POST /api/trials/{id}/start", s.startAcquisition)
	mux.HandleFunc("POST /api/trials/{id}/finish", s.finishAcquisition)
	mux.HandleFunc("POST /api/trials/{id}/confirm", s.confirmTrial)

	// 声纹片段与通道校准。
	mux.HandleFunc("POST /api/trials/{id}/segments", s.ingestSegment)
	mux.HandleFunc("GET /api/trials/{id}/segments", s.listSegments)
	mux.HandleFunc("POST /api/segments/{id}/noisy", s.markNoisy)
	mux.HandleFunc("POST /api/trials/{id}/calibrate", s.calibrateChannels)
	mux.HandleFunc("GET /api/trials/{id}/delays", s.listDelays)

	// 分析与空化事件。
	mux.HandleFunc("POST /api/trials/{id}/analyze", s.analyzeTrial)
	mux.HandleFunc("GET /api/trials/{id}/events", s.listEvents)
	mux.HandleFunc("GET /api/events/{id}", s.getEvent)
	mux.HandleFunc("POST /api/events/{id}/reject", s.rejectEvent)
	mux.HandleFunc("POST /api/events/{id}/advance", s.advanceEvent)
	mux.HandleFunc("GET /api/trials/{id}/features", s.listFeatures)

	// 结论包与阈值。
	mux.HandleFunc("POST /api/trials/{id}/packages", s.publishPackage)
	mux.HandleFunc("GET /api/trials/{id}/packages", s.listPackages)
	mux.HandleFunc("GET /api/packages/{id}", s.getPackage)
	mux.HandleFunc("GET /api/thresholds", s.listThresholds)
	mux.HandleFunc("POST /api/thresholds", s.addThreshold)

	// 统计与健康。
	mux.HandleFunc("GET /api/stats", s.stats)
	mux.HandleFunc("GET /api/health", s.health)

	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "task220-cavitation"})
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	st, err := s.app.Stats()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// writeJSON 输出 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError 把领域错误映射为 HTTP 状态码并输出。
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, model.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, model.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, model.ErrInvalidState), errors.Is(err, model.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, model.ErrSealed):
		status = http.StatusConflict
	case errors.Is(err, model.ErrDuplicate):
		status = http.StatusConflict
	case errors.Is(err, model.ErrInsufficientData):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, model.ErrCalibrationFailed):
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// decode 解析 JSON 请求体。
func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
