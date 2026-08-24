package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task220-cavitation/internal/service"
	"task220-cavitation/internal/store"
)

func TestHandlerHealthAndCreateTrial(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	app, err := service.New(db)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	h := New(app).Handler()

	health := httptest.NewRecorder()
	h.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", health.Code, http.StatusOK)
	}

	body := bytes.NewBufferString(`{"name":"http trial","description":"api test","shaft_speed_rpm":1500,"inflow_pressure_kpa":200,"reference_channel":0}`)
	create := httptest.NewRecorder()
	h.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/trials", body))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d: %s", create.Code, http.StatusCreated, create.Body.String())
	}
	var response struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if response.ID == "" || response.Status != "preparing" {
		t.Fatalf("unexpected create response: %+v", response)
	}
}
