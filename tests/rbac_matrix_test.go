package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"goravel/app/services"
)

func TestRbacMatrixParsing(t *testing.T) {
	matrixService := services.NewRbacMatrixService()
	res, err := matrixService.BuildMatrix()

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Greater(t, res.Stats.TotalModules, 0)
	assert.Greater(t, res.Stats.TotalFeatures, 0)
	assert.Greater(t, res.Stats.TotalFeatureActions, 0)
	assert.GreaterOrEqual(t, len(res.Roles), 5)

	t.Logf("Total Applications: %d", res.Stats.TotalApplications)
	t.Logf("Total Modules: %d", res.Stats.TotalModules)
	t.Logf("Total Features: %d", res.Stats.TotalFeatures)
	t.Logf("Total Feature Actions: %d", res.Stats.TotalFeatureActions)
	t.Logf("Total Endpoints: %d", res.Stats.TotalEndpoints)
	t.Logf("Total UI Routes: %d", res.Stats.TotalUiRoutes)
}
