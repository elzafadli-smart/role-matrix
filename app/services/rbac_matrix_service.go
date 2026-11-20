package services

import (
	"fmt"
	"sort"
	"sync"

	"goravel/app/facades"
	"goravel/app/models"
)

type MatrixResponse struct {
	Roles        []string        `json:"roles"`
	Applications []AppMatrixNode `json:"applications"`
	Stats        MatrixStats     `json:"stats"`
	Source       string          `json:"source"` // "database" or "seeder_sql"
}

type MatrixStats struct {
	TotalApplications   int `json:"total_applications"`
	TotalModules        int `json:"total_modules"`
	TotalFeatures       int `json:"total_features"`
	TotalFeatureActions int `json:"total_feature_actions"`
	TotalRolePerms      int `json:"total_role_permissions"`
	TotalEndpoints      int `json:"total_endpoints"`
	TotalUiRoutes       int `json:"total_ui_routes"`
}

type AppMatrixNode struct {
	ID      string             `json:"id"`
	Code    string             `json:"code"`
	Name    string             `json:"name"`
	Modules []ModuleMatrixNode `json:"modules"`
}

type ModuleMatrixNode struct {
	ID       string              `json:"id"`
	Code     string              `json:"code"`
	Features []FeatureMatrixNode `json:"features"`
}

type FeatureMatrixNode struct {
	ID              string             `json:"id"`
	Code            string             `json:"code"`
	IsActionFeature bool               `json:"is_action_feature"`
	Actions         []ActionMatrixNode `json:"actions"`
}

type ActionMatrixNode struct {
	FeatureActionID string          `json:"feature_action_id"`
	Action          string          `json:"action"`
	Roles           map[string]bool `json:"roles"`
	Endpoints       []string        `json:"endpoints"`
	UiRoutes        []string        `json:"ui_routes"`
	PermissionKey   string          `json:"permission_key"`
}

type RbacMatrixService struct{}

var (
	matrixServiceInstance *RbacMatrixService
	matrixOnce            sync.Once
)

func NewRbacMatrixService() *RbacMatrixService {
	matrixOnce.Do(func() {
		matrixServiceInstance = &RbacMatrixService{}
	})
	return matrixServiceInstance
}

func (s *RbacMatrixService) BuildMatrix() (*MatrixResponse, error) {
	return s.buildMatrixFromDb()
}

func (s *RbacMatrixService) buildMatrixFromDb() (*MatrixResponse, error) {
	orm := facades.Orm()
	if orm == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	// 1. Fetch Master Entities
	var apps []models.Application
	if err := orm.Query().Order("code asc").Find(&apps); err != nil {
		return nil, err
	}
	if len(apps) == 0 {
		return nil, fmt.Errorf("no applications found in database")
	}

	var modules []models.Module
	if err := orm.Query().Order("code asc").Find(&modules); err != nil {
		return nil, err
	}

	var features []models.Feature
	if err := orm.Query().Order("code asc").Find(&features); err != nil {
		return nil, err
	}

	var actions []models.Action
	if err := orm.Query().Find(&actions); err != nil {
		return nil, err
	}

	var featureActions []models.FeatureAction
	if err := orm.Query().Find(&featureActions); err != nil {
		return nil, err
	}

	var roles []models.Role
	if err := orm.Query().Order("name asc").Find(&roles); err != nil {
		return nil, err
	}

	var rolePerms []models.RolePermission
	if err := orm.Query().Find(&rolePerms); err != nil {
		return nil, err
	}

	var endpoints []models.ApiEndpoint
	_ = orm.Query().Find(&endpoints)

	var uiRoutes []models.UiRoute
	_ = orm.Query().Find(&uiRoutes)

	// Index Lookups
	actionMap := make(map[string]string) // action.id -> code
	for _, a := range actions {
		actionMap[a.ID] = a.Code
	}

	// Build Role Names list
	var roleNames []string
	roleIdToName := make(map[string]string)
	roleExists := make(map[string]bool)
	for _, r := range roles {
		// Clean display name
		displayName := r.Name
		roleIdToName[r.ID] = displayName
		if !roleExists[displayName] {
			roleExists[displayName] = true
			roleNames = append(roleNames, displayName)
		}
	}
	sortRoleNames(roleNames)

	// roleId -> feature_action_id -> true
	rolePermMap := make(map[string]map[string]bool)
	for _, rp := range rolePerms {
		if rolePermMap[rp.RoleID] == nil {
			rolePermMap[rp.RoleID] = make(map[string]bool)
		}
		rolePermMap[rp.RoleID][rp.FeatureActionID] = true
	}

	// feature_action_id -> endpoints
	epMap := make(map[string][]string)
	for _, ep := range endpoints {
		label := ep.Method + " " + ep.Path
		if ep.ServiceName != "" {
			label += " (" + ep.ServiceName + ")"
		}
		epMap[ep.FeatureActionID] = append(epMap[ep.FeatureActionID], label)
	}

	// feature_action_id -> ui_routes
	urMap := make(map[string][]string)
	for _, ur := range uiRoutes {
		urMap[ur.FeatureActionID] = append(urMap[ur.FeatureActionID], ur.Permission)
	}

	// feature_id -> []FeatureAction
	featureToActions := make(map[string][]models.FeatureAction)
	for _, fa := range featureActions {
		featureToActions[fa.FeatureID] = append(featureToActions[fa.FeatureID], fa)
	}

	// module_id -> []Feature
	moduleToFeatures := make(map[string][]models.Feature)
	for _, f := range features {
		moduleToFeatures[f.ModuleID] = append(moduleToFeatures[f.ModuleID], f)
	}

	// application_id -> []Module
	appToModules := make(map[string][]models.Module)
	for _, m := range modules {
		appToModules[m.ApplicationID] = append(appToModules[m.ApplicationID], m)
	}

	// Assemble Hierarchical Tree
	var appNodes []AppMatrixNode
	for _, app := range apps {
		modList := appToModules[app.ID]
		var modNodes []ModuleMatrixNode

		for _, m := range modList {
			featList := moduleToFeatures[m.ID]
			var featNodes []FeatureMatrixNode

			for _, f := range featList {
				faList := featureToActions[f.ID]
				var actionNodes []ActionMatrixNode

				for _, fa := range faList {
					actCode := actionMap[fa.ActionID]
					if actCode == "" {
						continue
					}

					rolesWithAccess := make(map[string]bool)
					for _, r := range roles {
						hasPerm := false
						if rolePermMap[r.ID] != nil && rolePermMap[r.ID][fa.ID] {
							hasPerm = true
						}
						rolesWithAccess[r.Name] = hasPerm
					}

					actionNodes = append(actionNodes, ActionMatrixNode{
						FeatureActionID: fa.ID,
						Action:          actCode,
						Roles:           rolesWithAccess,
						Endpoints:       epMap[fa.ID],
						UiRoutes:        urMap[fa.ID],
						PermissionKey:   m.Code + ":" + f.Code + ":" + actCode,
					})
				}

				sort.Slice(actionNodes, func(i, j int) bool {
					return actionNodes[i].Action < actionNodes[j].Action
				})

				featNodes = append(featNodes, FeatureMatrixNode{
					ID:              f.ID,
					Code:            f.Code,
					IsActionFeature: f.IsActionFeature,
					Actions:         actionNodes,
				})
			}

			modNodes = append(modNodes, ModuleMatrixNode{
				ID:       m.ID,
				Code:     m.Code,
				Features: featNodes,
			})
		}

		appName := app.Code
		switch app.Code {
		case "GROAPP_ACCESS":
			appName = "GroApp Access Service (Identity & Multi-Tenancy)"
		case "GROAPP_ACCOUNTING":
			appName = "GroApp Accounting Service (Finance, Kas & Bank, Invoicing)"
		case "GROAPP_REPORTING":
			appName = "GroApp Reporting Service (Dashboard & Business Intelligence)"
		}

		appNodes = append(appNodes, AppMatrixNode{
			ID:      app.ID,
			Code:    app.Code,
			Name:    appName,
			Modules: modNodes,
		})
	}

	stats := MatrixStats{
		TotalApplications:   len(apps),
		TotalModules:        len(modules),
		TotalFeatures:       len(features),
		TotalFeatureActions: len(featureActions),
		TotalRolePerms:      len(rolePerms),
		TotalEndpoints:      len(endpoints),
		TotalUiRoutes:       len(uiRoutes),
	}

	return &MatrixResponse{
		Roles:        roleNames,
		Applications: appNodes,
		Stats:        stats,
		Source:       "database",
	}, nil
}

func (s *RbacMatrixService) GetEndpointsWithRoles() ([]map[string]any, error) {
	orm := facades.Orm()
	if orm == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	var endpoints []models.ApiEndpoint
	if err := orm.Query().Find(&endpoints); err != nil {
		return nil, err
	}

	var featureActions []models.FeatureAction
	if err := orm.Query().Find(&featureActions); err != nil {
		return nil, err
	}
	faMap := make(map[string]models.FeatureAction)
	for _, fa := range featureActions {
		faMap[fa.ID] = fa
	}

	var features []models.Feature
	if err := orm.Query().Find(&features); err != nil {
		return nil, err
	}
	featMap := make(map[string]models.Feature)
	for _, f := range features {
		featMap[f.ID] = f
	}

	var modules []models.Module
	if err := orm.Query().Find(&modules); err != nil {
		return nil, err
	}
	modMap := make(map[string]models.Module)
	for _, m := range modules {
		modMap[m.ID] = m
	}

	var actions []models.Action
	if err := orm.Query().Find(&actions); err != nil {
		return nil, err
	}
	actMap := make(map[string]string)
	for _, a := range actions {
		actMap[a.ID] = a.Code
	}

	var roles []models.Role
	if err := orm.Query().Find(&roles); err != nil {
		return nil, err
	}
	roleMap := make(map[string]string)
	for _, r := range roles {
		roleMap[r.ID] = r.Name
	}

	var rolePerms []models.RolePermission
	if err := orm.Query().Find(&rolePerms); err != nil {
		return nil, err
	}
	rolePermMap := make(map[string][]string)
	for _, rp := range rolePerms {
		if rName, ok := roleMap[rp.RoleID]; ok {
			rolePermMap[rp.FeatureActionID] = append(rolePermMap[rp.FeatureActionID], rName)
		}
	}

	for faID := range rolePermMap {
		sortRoleNames(rolePermMap[faID])
	}

	var results []map[string]any
	for _, ep := range endpoints {
		fa := faMap[ep.FeatureActionID]
		feat := featMap[fa.FeatureID]
		mod := modMap[feat.ModuleID]
		act := actMap[fa.ActionID]

		results = append(results, map[string]any{
			"method":           ep.Method,
			"path":             ep.Path,
			"service_name":     ep.ServiceName,
			"module":           mod.Code,
			"feature":          feat.Code,
			"action":           act,
			"description":      ep.Description,
			"authorized_roles": rolePermMap[ep.FeatureActionID],
		})
	}
	return results, nil
}

func (s *RbacMatrixService) GetUiRoutesWithRoles() ([]map[string]any, error) {
	orm := facades.Orm()
	if orm == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	var uiRoutes []models.UiRoute
	if err := orm.Query().Find(&uiRoutes); err != nil {
		return nil, err
	}

	var featureActions []models.FeatureAction
	if err := orm.Query().Find(&featureActions); err != nil {
		return nil, err
	}
	faMap := make(map[string]models.FeatureAction)
	for _, fa := range featureActions {
		faMap[fa.ID] = fa
	}

	var features []models.Feature
	if err := orm.Query().Find(&features); err != nil {
		return nil, err
	}
	featMap := make(map[string]models.Feature)
	for _, f := range features {
		featMap[f.ID] = f
	}

	var modules []models.Module
	if err := orm.Query().Find(&modules); err != nil {
		return nil, err
	}
	modMap := make(map[string]models.Module)
	for _, m := range modules {
		modMap[m.ID] = m
	}

	var actions []models.Action
	if err := orm.Query().Find(&actions); err != nil {
		return nil, err
	}
	actMap := make(map[string]string)
	for _, a := range actions {
		actMap[a.ID] = a.Code
	}

	var roles []models.Role
	if err := orm.Query().Find(&roles); err != nil {
		return nil, err
	}
	roleMap := make(map[string]string)
	for _, r := range roles {
		roleMap[r.ID] = r.Name
	}

	var rolePerms []models.RolePermission
	if err := orm.Query().Find(&rolePerms); err != nil {
		return nil, err
	}
	rolePermMap := make(map[string][]string)
	for _, rp := range rolePerms {
		if rName, ok := roleMap[rp.RoleID]; ok {
			rolePermMap[rp.FeatureActionID] = append(rolePermMap[rp.FeatureActionID], rName)
		}
	}

	for faID := range rolePermMap {
		sortRoleNames(rolePermMap[faID])
	}

	var results []map[string]any
	for _, ur := range uiRoutes {
		fa := faMap[ur.FeatureActionID]
		feat := featMap[fa.FeatureID]
		mod := modMap[feat.ModuleID]
		act := actMap[fa.ActionID]

		results = append(results, map[string]any{
			"permission":       ur.Permission,
			"module":           mod.Code,
			"feature":          feat.Code,
			"action":           act,
			"description":      ur.Description,
			"authorized_roles": rolePermMap[ur.FeatureActionID],
		})
	}
	return results, nil
}

// sortRoleNames sorts roles according to the business hierarchy:
// 1. Pemilik Usaha (Owner)
// 2. Administrator Perusahaan (Admin)
// 3. Admin Keuangan (Keuangan)
// 4. Admin Pengadaan (Pengadaan)
// 5. Sales (Sales)
func sortRoleNames(roles []string) {
	priority := map[string]int{
		"Pemilik Usaha":            1,
		"Owner":                    1,
		"Administrator Perusahaan": 2,
		"Admin Perusahaan":         2,
		"Admin":                    2,
		"Admin Keuangan":           3,
		"Keuangan":                 3,
		"Finance":                  3,
		"Admin Pengadaan":          4,
		"Pengadaan":                4,
		"Procurement":              4,
		"Sales":                    5,
		"Kasir":                    5,
	}

	sort.Slice(roles, func(i, j int) bool {
		p1, ok1 := priority[roles[i]]
		p2, ok2 := priority[roles[j]]
		if !ok1 {
			p1 = 99
		}
		if !ok2 {
			p2 = 99
		}
		if p1 != p2 {
			return p1 < p2
		}
		return roles[i] < roles[j]
	})
}
