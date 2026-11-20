package controllers

import (
	"fmt"
	"strings"

	"github.com/goravel/framework/contracts/http"
	"goravel/app/facades"
	"goravel/app/services"
)

type RbacController struct {
	matrixService *services.RbacMatrixService
}

func NewRbacController() *RbacController {
	return &RbacController{
		matrixService: services.NewRbacMatrixService(),
	}
}

type UserCompanyRecord struct {
	MembershipID          string  `json:"membership_id" gorm:"column:membership_id"`
	UserID                string  `json:"user_id" gorm:"column:user_id"`
	UserName              string  `json:"user_name" gorm:"column:user_name"`
	Email                 string  `json:"email" gorm:"column:email"`
	CompanyID             string  `json:"company_id" gorm:"column:company_id"`
	CompanyName           string  `json:"company_name" gorm:"column:company_name"`
	BusinessType          string  `json:"business_type" gorm:"column:business_type"`
	RoleID                string  `json:"role_id" gorm:"column:role_id"`
	RoleName              string  `json:"role_name" gorm:"column:role_name"`
	CompanyAccess         bool    `json:"company_access" gorm:"column:company_access"`
	UnitAccessMode        string  `json:"unit_access_mode" gorm:"column:unit_access_mode"`
	AutoAssignFutureUnits bool    `json:"auto_assign_future_units" gorm:"column:auto_assign_future_units"`
	JoinedAt              string  `json:"joined_at" gorm:"column:joined_at"`
	RevokedAt             *string `json:"revoked_at" gorm:"column:revoked_at"`
}

// Matrix returns the full hierarchical RBAC grid matrix
func (c *RbacController) Matrix(ctx http.Context) http.Response {
	matrix, err := c.matrixService.BuildMatrix()
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{
			"error": err.Error(),
		})
	}
	return ctx.Response().Success().Json(matrix)
}

// Endpoints returns all backend API endpoints with mapped permissions & authorized roles
func (c *RbacController) Endpoints(ctx http.Context) http.Response {
	endpoints, err := c.matrixService.GetEndpointsWithRoles()
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{
			"error": err.Error(),
		})
	}
	return ctx.Response().Success().Json(http.Json{
		"total": len(endpoints),
		"data":  endpoints,
	})
}

// UiRoutes returns all frontend UI routes with mapped permissions & authorized roles
func (c *RbacController) UiRoutes(ctx http.Context) http.Response {
	routes, err := c.matrixService.GetUiRoutesWithRoles()
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{
			"error": err.Error(),
		})
	}
	return ctx.Response().Success().Json(http.Json{
		"total": len(routes),
		"data":  routes,
	})
}

// Stats returns high-level summary statistics of the RBAC system
func (c *RbacController) Stats(ctx http.Context) http.Response {
	matrix, err := c.matrixService.BuildMatrix()
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{
			"error": err.Error(),
		})
	}
	return ctx.Response().Success().Json(http.Json{
		"roles": matrix.Roles,
		"stats": matrix.Stats,
	})
}

// Simulate evaluates whether a specified role has access to a feature + action
func (c *RbacController) Simulate(ctx http.Context) http.Response {
	role := ctx.Request().Query("role")
	module := ctx.Request().Query("module")
	feature := ctx.Request().Query("feature")
	action := ctx.Request().Query("action")

	if role == "" || module == "" || feature == "" || action == "" {
		return ctx.Response().Json(http.StatusBadRequest, http.Json{
			"error": "Query parameters required: role, module, feature, action",
		})
	}

	matrix, err := c.matrixService.BuildMatrix()
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{
			"error": err.Error(),
		})
	}

	allowed := false
	var matchingEndpoints []string
	var matchingUiRoutes []string

	for _, app := range matrix.Applications {
		for _, mod := range app.Modules {
			if mod.Code == module {
				for _, feat := range mod.Features {
					if feat.Code == feature {
						for _, act := range feat.Actions {
							if act.Action == action {
								allowed = act.Roles[role]
								matchingEndpoints = act.Endpoints
								matchingUiRoutes = act.UiRoutes
								break
							}
						}
					}
				}
			}
		}
	}

	return ctx.Response().Success().Json(http.Json{
		"role":               role,
		"module":             module,
		"feature":            feature,
		"action":             action,
		"allowed":            allowed,
		"matching_endpoints": matchingEndpoints,
		"matching_ui_routes": matchingUiRoutes,
	})
}

// loadUserCompaniesData helper: queries live database directly
func loadUserCompaniesData() ([]UserCompanyRecord, error) {
	orm := facades.Orm()
	if orm == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	encKey := facades.Config().GetString("APP_ENCRYPTION_KEY")

	var records []UserCompanyRecord
	query := fmt.Sprintf(`
		SELECT 
			uc.id::text                      AS membership_id,
			uc.user_id::text                 AS user_id,
			u.name                           AS user_name,
			COALESCE(public.pgp_sym_decrypt(u.email::bytea, '%s'::text), '') AS email,
			uc.company_id::text              AS company_id,
			c.name                           AS company_name,
			c.business_type                  AS business_type,
			uc.role_id::text                 AS role_id,
			r.name                           AS role_name,
			uc.company_access                AS company_access,
			uc.unit_access_mode              AS unit_access_mode,
			uc.auto_assign_future_units      AS auto_assign_future_units,
			to_char(uc.joined_at, 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS joined_at,
			to_char(uc.revoked_at, 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"') AS revoked_at
		FROM access.user_companies uc
		JOIN access.users u        ON uc.user_id = u.id
		JOIN access.companies c    ON uc.company_id = c.id
		JOIN access.roles r        ON uc.role_id = r.id
		WHERE uc.revoked_at IS NULL
		  AND u.deleted_at IS NULL
		  AND c.deleted_at IS NULL
		ORDER BY u.name ASC, c.name ASC
	`, encKey)

	if err := orm.Query().Raw(query).Scan(&records); err != nil {
		return nil, err
	}

	return records, nil
}

// UserCompanies returns list of available users, companies, and their membership options for the modal picker
func (c *RbacController) UserCompanies(ctx http.Context) http.Response {
	records, err := loadUserCompaniesData()
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{
			"error": err.Error(),
		})
	}

	type UserItem struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Email string `json:"email"`
	}
	type CompanyItem struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		BusinessType string `json:"business_type"`
	}

	userMap := make(map[string]UserItem)
	compMap := make(map[string]CompanyItem)

	for _, r := range records {
		userMap[r.UserID] = UserItem{
			ID:    r.UserID,
			Name:  r.UserName,
			Email: r.Email,
		}
		compMap[r.CompanyID] = CompanyItem{
			ID:           r.CompanyID,
			Name:         r.CompanyName,
			BusinessType: r.BusinessType,
		}
	}

	var users []UserItem
	for _, u := range userMap {
		users = append(users, u)
	}
	var companies []CompanyItem
	for _, co := range compMap {
		companies = append(companies, co)
	}

	return ctx.Response().Success().Json(http.Json{
		"total_users":       len(users),
		"total_companies":   len(companies),
		"total_memberships": len(records),
		"users":             users,
		"companies":         companies,
		"memberships":       records,
	})
}

// InspectUserRole inspects the specific role of a user in a company
func (c *RbacController) InspectUserRole(ctx http.Context) http.Response {
	userId := ctx.Request().Query("user_id")
	userEmail := ctx.Request().Query("email")
	companyId := ctx.Request().Query("company_id")
	companyName := ctx.Request().Query("company_name")

	records, err := loadUserCompaniesData()
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{
			"error": err.Error(),
		})
	}

	var matched *UserCompanyRecord
	for _, r := range records {
		userMatch := false
		if userId != "" && r.UserID == userId {
			userMatch = true
		} else if userEmail != "" && (strings.EqualFold(r.Email, userEmail) || strings.EqualFold(r.UserName, userEmail)) {
			userMatch = true
		}

		compMatch := false
		if companyId != "" && r.CompanyID == companyId {
			compMatch = true
		} else if companyName != "" && strings.EqualFold(r.CompanyName, companyName) {
			compMatch = true
		}

		if userMatch && compMatch {
			matched = &r
			break
		}
	}

	if matched == nil {
		return ctx.Response().Json(http.StatusNotFound, http.Json{
			"error": "User membership in company not found",
		})
	}

	// Business units for company: query live from database
	var businessUnits []map[string]string
	if orm := facades.Orm(); orm != nil && matched.CompanyID != "" {
		type BuRow struct {
			ID       string `gorm:"column:id"`
			Name     string `gorm:"column:name"`
			UnitType string `gorm:"column:unit_type"`
			Status   string `gorm:"column:status"`
		}
		var buRows []BuRow
		if err := orm.Query().Raw(`
			SELECT id::text, name, unit_type, status 
			FROM access.business_units 
			WHERE company_id = ? AND deleted_at IS NULL 
			ORDER BY name ASC
		`, matched.CompanyID).Scan(&buRows); err == nil && len(buRows) > 0 {
			for _, b := range buRows {
				businessUnits = append(businessUnits, map[string]string{
					"id":        b.ID,
					"name":      b.Name,
					"unit_type": b.UnitType,
					"status":    b.Status,
				})
			}
		}
	}

	return ctx.Response().Success().Json(http.Json{
		"membership_id": matched.MembershipID,
		"user": http.Json{
			"id":    matched.UserID,
			"name":  matched.UserName,
			"email": matched.Email,
		},
		"company": http.Json{
			"id":            matched.CompanyID,
			"name":          matched.CompanyName,
			"business_type": matched.BusinessType,
		},
		"role": http.Json{
			"id":   matched.RoleID,
			"name": matched.RoleName,
		},
		"access_scope": http.Json{
			"company_access":           matched.CompanyAccess,
			"unit_access_mode":        matched.UnitAccessMode,
			"auto_assign_future_units": matched.AutoAssignFutureUnits,
			"joined_at":                matched.JoinedAt,
			"revoked_at":               matched.RevokedAt,
			"is_active":                matched.RevokedAt == nil,
		},
		"business_units": businessUnits,
	})
}
