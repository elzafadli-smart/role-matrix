package services

import (
	"sort"
	"sync"
	"time"

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

type RbacMatrixService struct {
	parser          *RbacParserService
	mu              sync.RWMutex
	cachedMatrix    *MatrixResponse
	cachedEndpoints []map[string]any
	cachedUiRoutes  []map[string]any
	matrixCachedAt  time.Time
	epCachedAt      time.Time
	urCachedAt      time.Time
}

var (
	matrixServiceInstance *RbacMatrixService
	matrixOnce            sync.Once
)

func NewRbacMatrixService() *RbacMatrixService {
	matrixOnce.Do(func() {
		matrixServiceInstance = &RbacMatrixService{
			parser: GetRbacParser(),
		}
	})
	return matrixServiceInstance
}

func (s *RbacMatrixService) BuildMatrix() (*MatrixResponse, error) {
	s.mu.RLock()
	if s.cachedMatrix != nil && time.Since(s.matrixCachedAt) < 10*time.Minute {
		res := s.cachedMatrix
		s.mu.RUnlock()
		return res, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedMatrix != nil && time.Since(s.matrixCachedAt) < 10*time.Minute {
		return s.cachedMatrix, nil
	}

	// First priority: Try fetching live data from database via Goravel ORM
	if res, err := s.buildMatrixFromDb(); err == nil && res != nil && len(res.Applications) > 0 {
		s.cachedMatrix = res
		s.matrixCachedAt = time.Now()
		return res, nil
	}

	// Fallback to parsed SQL seeders
	res, err := s.buildMatrixFromParser()
	if err == nil && res != nil {
		s.cachedMatrix = res
		s.matrixCachedAt = time.Now()
	}
	return res, err
}

func (s *RbacMatrixService) buildMatrixFromDb() (*MatrixResponse, error) {
	orm := facades.Orm()
	if orm == nil {
		return nil, nil
	}

	// 1. Fetch Master Entities
	var apps []models.Application
	if err := orm.Query().Order("code asc").Find(&apps); err != nil || len(apps) == 0 {
		return nil, err
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

func (s *RbacMatrixService) buildMatrixFromParser() (*MatrixResponse, error) {
	data, err := s.parser.GetData()
	if err != nil {
		return nil, err
	}

	roleNames := []string{
		"Pemilik Usaha",
		"Administrator Perusahaan",
		"Admin Keuangan",
		"Admin Pengadaan",
		"Sales",
	}

	endpointsMap := make(map[string][]string)
	for _, ep := range data.ApiEndpoints {
		key := ep.ModuleCode + ":" + ep.FeatureCode + ":" + ep.ActionCode
		endpointsMap[key] = append(endpointsMap[key], ep.Method+" "+ep.Path+" ("+ep.ServiceName+")")
	}

	uiRoutesMap := make(map[string][]string)
	for _, ur := range data.UiRoutes {
		key := ur.ModuleCode + ":" + ur.FeatureCode + ":" + ur.ActionCode
		uiRoutesMap[key] = append(uiRoutesMap[key], ur.Permission)
	}

	appModules := make(map[string]map[string]map[string][]string)

	for _, fa := range data.FeatureActions {
		app := fa.AppCode
		if app == "" {
			app = "GROAPP_ACCOUNTING"
		}
		if _, ok := appModules[app]; !ok {
			appModules[app] = make(map[string]map[string][]string)
		}
		if _, ok := appModules[app][fa.ModuleCode]; !ok {
			appModules[app][fa.ModuleCode] = make(map[string][]string)
		}

		actions := appModules[app][fa.ModuleCode][fa.Feature]
		exists := false
		for _, a := range actions {
			if a == fa.Action {
				exists = true
				break
			}
		}
		if !exists {
			appModules[app][fa.ModuleCode][fa.Feature] = append(actions, fa.Action)
		}
	}

	totalFeatureActions := len(data.FeatureActions)
	totalFeatures := 0
	totalModules := 0

	var appNodes []AppMatrixNode
	for appCode, modules := range appModules {
		totalModules += len(modules)
		var moduleNodes []ModuleMatrixNode

		for modCode, features := range modules {
			totalFeatures += len(features)
			var featNodes []FeatureMatrixNode

			for featCode, actions := range features {
				featKey := modCode + ":" + featCode
				isAction := false
				if featEntry, ok := data.Features[featKey]; ok {
					isAction = featEntry.IsActionFeature
				}

				sort.Strings(actions)

				var actionNodes []ActionMatrixNode
				for _, act := range actions {
					faKey := modCode + ":" + featCode + ":" + act

					rolesMap := make(map[string]bool)
					for _, r := range roleNames {
						hasPerm := false
						for _, grantedKey := range data.RolePermissions[r] {
							if grantedKey == faKey {
								hasPerm = true
								break
							}
						}
						rolesMap[r] = hasPerm
					}

					actionNodes = append(actionNodes, ActionMatrixNode{
						Action:        act,
						Roles:         rolesMap,
						Endpoints:     endpointsMap[faKey],
						UiRoutes:      uiRoutesMap[faKey],
						PermissionKey: faKey,
					})
				}

				featNodes = append(featNodes, FeatureMatrixNode{
					Code:            featCode,
					IsActionFeature: isAction,
					Actions:         actionNodes,
				})
			}

			moduleNodes = append(moduleNodes, ModuleMatrixNode{
				Code:     modCode,
				Features: featNodes,
			})
		}

		appName := data.Applications[appCode]
		if appName == "" {
			appName = appCode
		}

		appNodes = append(appNodes, AppMatrixNode{
			Code:    appCode,
			Name:    appName,
			Modules: moduleNodes,
		})
	}

	stats := MatrixStats{
		TotalApplications:   len(appNodes),
		TotalModules:        totalModules,
		TotalFeatures:       totalFeatures,
		TotalFeatureActions: totalFeatureActions,
		TotalEndpoints:      len(data.ApiEndpoints),
		TotalUiRoutes:       len(data.UiRoutes),
	}

	return &MatrixResponse{
		Roles:        roleNames,
		Applications: appNodes,
		Stats:        stats,
		Source:       "seeder_sql",
	}, nil
}

func (s *RbacMatrixService) GetEndpointsWithRoles() ([]map[string]any, error) {
	s.mu.RLock()
	if s.cachedEndpoints != nil && time.Since(s.epCachedAt) < 10*time.Minute {
		res := s.cachedEndpoints
		s.mu.RUnlock()
		return res, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedEndpoints != nil && time.Since(s.epCachedAt) < 10*time.Minute {
		return s.cachedEndpoints, nil
	}

	orm := facades.Orm()
	if orm != nil {
		var endpoints []models.ApiEndpoint
		if err := orm.Query().Find(&endpoints); err == nil && len(endpoints) > 0 {
			var featureActions []models.FeatureAction
			_ = orm.Query().Find(&featureActions)
			faMap := make(map[string]models.FeatureAction)
			for _, fa := range featureActions {
				faMap[fa.ID] = fa
			}

			var features []models.Feature
			_ = orm.Query().Find(&features)
			featMap := make(map[string]models.Feature)
			for _, f := range features {
				featMap[f.ID] = f
			}

			var modules []models.Module
			_ = orm.Query().Find(&modules)
			modMap := make(map[string]models.Module)
			for _, m := range modules {
				modMap[m.ID] = m
			}

			var actions []models.Action
			_ = orm.Query().Find(&actions)
			actMap := make(map[string]string)
			for _, a := range actions {
				actMap[a.ID] = a.Code
			}

			var roles []models.Role
			_ = orm.Query().Find(&roles)

			var rolePerms []models.RolePermission
			_ = orm.Query().Find(&rolePerms)
			rolePermMap := make(map[string][]string) // fa.id -> []roleName
			for _, rp := range rolePerms {
				for _, r := range roles {
					if r.ID == rp.RoleID {
						rolePermMap[rp.FeatureActionID] = append(rolePermMap[rp.FeatureActionID], r.Name)
					}
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
			s.cachedEndpoints = results
			s.epCachedAt = time.Now()
			return results, nil
		}
	}

	// Fallback to parser
	data, err := s.parser.GetData()
	if err != nil {
		return nil, err
	}

	roleNames := []string{"Pemilik Usaha", "Administrator Perusahaan", "Admin Keuangan", "Admin Pengadaan", "Sales"}
	var results []map[string]any
	for _, ep := range data.ApiEndpoints {
		faKey := ep.ModuleCode + ":" + ep.FeatureCode + ":" + ep.ActionCode
		var authorizedRoles []string
		for _, r := range roleNames {
			for _, p := range data.RolePermissions[r] {
				if p == faKey {
					authorizedRoles = append(authorizedRoles, r)
					break
				}
			}
		}

		results = append(results, map[string]any{
			"method":           ep.Method,
			"path":             ep.Path,
			"service_name":     ep.ServiceName,
			"module":           ep.ModuleCode,
			"feature":          ep.FeatureCode,
			"action":           ep.ActionCode,
			"description":      ep.Description,
			"authorized_roles": authorizedRoles,
		})
	}
	s.cachedEndpoints = results
	s.epCachedAt = time.Now()
	return results, nil
}

func (s *RbacMatrixService) GetUiRoutesWithRoles() ([]map[string]any, error) {
	s.mu.RLock()
	if s.cachedUiRoutes != nil && time.Since(s.urCachedAt) < 10*time.Minute {
		res := s.cachedUiRoutes
		s.mu.RUnlock()
		return res, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedUiRoutes != nil && time.Since(s.urCachedAt) < 10*time.Minute {
		return s.cachedUiRoutes, nil
	}

	orm := facades.Orm()
	if orm != nil {
		var uiRoutes []models.UiRoute
		if err := orm.Query().Find(&uiRoutes); err == nil && len(uiRoutes) > 0 {
			var featureActions []models.FeatureAction
			_ = orm.Query().Find(&featureActions)
			faMap := make(map[string]models.FeatureAction)
			for _, fa := range featureActions {
				faMap[fa.ID] = fa
			}

			var features []models.Feature
			_ = orm.Query().Find(&features)
			featMap := make(map[string]models.Feature)
			for _, f := range features {
				featMap[f.ID] = f
			}

			var modules []models.Module
			_ = orm.Query().Find(&modules)
			modMap := make(map[string]models.Module)
			for _, m := range modules {
				modMap[m.ID] = m
			}

			var actions []models.Action
			_ = orm.Query().Find(&actions)
			actMap := make(map[string]string)
			for _, a := range actions {
				actMap[a.ID] = a.Code
			}

			var roles []models.Role
			_ = orm.Query().Find(&roles)

			var rolePerms []models.RolePermission
			_ = orm.Query().Find(&rolePerms)
			rolePermMap := make(map[string][]string) // fa.id -> []roleName
			for _, rp := range rolePerms {
				for _, r := range roles {
					if r.ID == rp.RoleID {
						rolePermMap[rp.FeatureActionID] = append(rolePermMap[rp.FeatureActionID], r.Name)
					}
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
			s.cachedUiRoutes = results
			s.urCachedAt = time.Now()
			return results, nil
		}
	}

	// Fallback to parser
	data, err := s.parser.GetData()
	if err != nil {
		return nil, err
	}

	roleNames := []string{"Pemilik Usaha", "Administrator Perusahaan", "Admin Keuangan", "Admin Pengadaan", "Sales"}
	var results []map[string]any
	for _, ur := range data.UiRoutes {
		faKey := ur.ModuleCode + ":" + ur.FeatureCode + ":" + ur.ActionCode
		var authorizedRoles []string
		for _, r := range roleNames {
			for _, p := range data.RolePermissions[r] {
				if p == faKey {
					authorizedRoles = append(authorizedRoles, r)
					break
				}
			}
		}

		results = append(results, map[string]any{
			"permission":       ur.Permission,
			"module":           ur.ModuleCode,
			"feature":          ur.FeatureCode,
			"action":           ur.ActionCode,
			"description":      ur.Description,
			"authorized_roles": authorizedRoles,
		})
	}
	s.cachedUiRoutes = results
	s.urCachedAt = time.Now()
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
