package feature

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"goravel/app/facades"
	"goravel/tests"
)

type RbacApiTestSuite struct {
	tests.TestCase
}

func TestRbacApi(t *testing.T) {
	route := facades.Route()
	assert.NotNil(t, route)

	// Test GET /api/v1/rbac/stats
	req, _ := http.NewRequest("GET", "/api/v1/rbac/stats", nil)
	w := httptest.NewRecorder()
	route.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var statsRes map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &statsRes)
	assert.NoError(t, err)
	t.Logf("Stats API Response: %s", w.Body.String())

	// Test GET /api/v1/rbac/simulate
	simReq, _ := http.NewRequest("GET", "/api/v1/rbac/simulate?role=Pemilik+Usaha&module=CASH_AND_BANK&feature=ACCOUNT_LIST&action=VIEW", nil)
	simW := httptest.NewRecorder()
	route.ServeHTTP(simW, simReq)

	assert.Equal(t, http.StatusOK, simW.Code)
	t.Logf("Simulate API Response: %s", simW.Body.String())

	// Test Web Views
	webRoutes := []string{"/", "/matrix", "/endpoints", "/ui-routes"}
	for _, path := range webRoutes {
		r, _ := http.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		route.ServeHTTP(rec, r)
		assert.Equal(t, http.StatusOK, rec.Code, "Path %s should return 200 OK", path)
		t.Logf("Web Route %s rendered with status %d (size %d bytes)", path, rec.Code, rec.Body.Len())
	}
}
