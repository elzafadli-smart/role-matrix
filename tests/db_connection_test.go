package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"goravel/app/facades"
	"goravel/app/models"
)

func TestDatabaseConnection(t *testing.T) {
	orm := facades.Orm()
	assert.NotNil(t, orm)

	// Query roles from live DB
	var roles []models.Role
	err := orm.Query().Find(&roles)
	if err != nil {
		t.Skipf("Skipping direct DB test (pgx auth not configured or DB offline): %v", err)
		return
	}

	t.Logf(">>> SUCCESS! Fetched %d roles from database!", len(roles))
	for _, r := range roles {
		company := "GLOBAL"
		if r.CompanyID != nil {
			company = *r.CompanyID
		}
		t.Logf("Role: ID=%s, Name=%s, Company=%s", r.ID, r.Name, company)
	}

	// Query applications
	var apps []models.Application
	err = orm.Query().Find(&apps)
	assert.NoError(t, err)
	t.Logf(">>> Fetched %d applications from database!", len(apps))
	for _, a := range apps {
		t.Logf("App: ID=%s, Code=%s", a.ID, a.Code)
	}

	// Query modules
	var modules []models.Module
	err = orm.Query().Find(&modules)
	assert.NoError(t, err)
	t.Logf(">>> Fetched %d modules from database!", len(modules))

	// Query features
	var features []models.Feature
	err = orm.Query().Find(&features)
	assert.NoError(t, err)
	t.Logf(">>> Fetched %d features from database!", len(features))

	// Query feature_actions
	var featureActions []models.FeatureAction
	err = orm.Query().Find(&featureActions)
	assert.NoError(t, err)
	t.Logf(">>> Fetched %d feature_actions from database!", len(featureActions))

	// Query role_permissions
	var rolePerms []models.RolePermission
	err = orm.Query().Find(&rolePerms)
	assert.NoError(t, err)
	t.Logf(">>> Fetched %d role_permissions from database!", len(rolePerms))

	// Query api_endpoints
	var endpoints []models.ApiEndpoint
	err = orm.Query().Find(&endpoints)
	assert.NoError(t, err)
	t.Logf(">>> Fetched %d api_endpoints from database!", len(endpoints))

	// Query ui_routes
	var uiRoutes []models.UiRoute
	err = orm.Query().Find(&uiRoutes)
	assert.NoError(t, err)
	t.Logf(">>> Fetched %d ui_routes from database!", len(uiRoutes))
}
