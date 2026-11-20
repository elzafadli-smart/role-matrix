package controllers

import (
	"fmt"
	"math"
	"strconv"
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

// UserCompanies returns paginated list of memberships for modal picker (Server-side pagination)
func (c *RbacController) UserCompanies(ctx http.Context) http.Response {
	orm := facades.Orm()
	if orm == nil {
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{"error": "Database not initialized"})
	}

	encKey := facades.Config().GetString("APP_ENCRYPTION_KEY")

	page, _ := strconv.Atoi(ctx.Request().Query("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(ctx.Request().Query("per_page"))
	if perPage < 1 {
		perPage = 10
	}
	if perPage > 100 {
		perPage = 100
	}
	offset := (page - 1) * perPage

	search := strings.TrimSpace(ctx.Request().Query("search"))
	role := strings.TrimSpace(ctx.Request().Query("role"))
	company := strings.TrimSpace(ctx.Request().Query("company"))

	whereClauses := []string{
		"uc.revoked_at IS NULL",
		"u.deleted_at IS NULL",
		"c.deleted_at IS NULL",
	}
	var args []any

	if role != "" {
		whereClauses = append(whereClauses, "r.name = ?")
		args = append(args, role)
	}
	if company != "" {
		whereClauses = append(whereClauses, "c.name = ?")
		args = append(args, company)
	}
	if search != "" {
		searchParam := "%" + search + "%"
		if strings.Contains(search, "@") {
			whereClauses = append(whereClauses, fmt.Sprintf("(u.name ILIKE ? OR c.name ILIKE ? OR r.name ILIKE ? OR public.pgp_sym_decrypt(u.email::bytea, '%s'::text) ILIKE ?)", encKey))
			args = append(args, searchParam, searchParam, searchParam, searchParam)
		} else {
			whereClauses = append(whereClauses, "(u.name ILIKE ? OR c.name ILIKE ? OR r.name ILIKE ?)")
			args = append(args, searchParam, searchParam, searchParam)
		}
	}

	whereSql := strings.Join(whereClauses, " AND ")

	// Count total matching records
	var total int64
	countSql := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM access.user_companies uc
		JOIN access.users u        ON uc.user_id = u.id
		JOIN access.companies c    ON uc.company_id = c.id
		JOIN access.roles r        ON uc.role_id = r.id
		WHERE %s
	`, whereSql)
	if err := orm.Query().Raw(countSql, args...).Scan(&total); err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{"error": err.Error()})
	}

	// Fetch paginated memberships slice
	dataSql := fmt.Sprintf(`
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
		WHERE %s
		ORDER BY u.name ASC, c.name ASC
		LIMIT ? OFFSET ?
	`, encKey, whereSql)

	dataArgs := append([]any{}, args...)
	dataArgs = append(dataArgs, perPage, offset)

	var records []UserCompanyRecord
	if err := orm.Query().Raw(dataSql, dataArgs...).Scan(&records); err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{"error": err.Error()})
	}

	totalPages := 1
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(perPage)))
	}

	// Fetch distinct roles and companies once for filter dropdown options
	var roles []string
	_ = orm.Query().Raw(`
		SELECT DISTINCT r.name 
		FROM access.roles r 
		JOIN access.user_companies uc ON uc.role_id = r.id 
		WHERE uc.revoked_at IS NULL 
		ORDER BY r.name
	`).Scan(&roles)

	var companies []string
	_ = orm.Query().Raw(`
		SELECT DISTINCT c.name 
		FROM access.companies c 
		JOIN access.user_companies uc ON uc.company_id = c.id 
		WHERE uc.revoked_at IS NULL AND c.deleted_at IS NULL 
		ORDER BY c.name
	`).Scan(&companies)

	return ctx.Response().Success().Json(http.Json{
		"total":             total,
		"total_memberships": total,
		"page":              page,
		"per_page":          perPage,
		"total_pages":       totalPages,
		"roles":             roles,
		"companies":         companies,
		"memberships":       records,
	})
}

// InspectUserRole inspects the specific role of a user in a company (targeted single query)
func (c *RbacController) InspectUserRole(ctx http.Context) http.Response {
	userId := strings.TrimSpace(ctx.Request().Query("user_id"))
	userEmail := strings.TrimSpace(ctx.Request().Query("email"))
	companyId := strings.TrimSpace(ctx.Request().Query("company_id"))
	companyName := strings.TrimSpace(ctx.Request().Query("company_name"))

	orm := facades.Orm()
	if orm == nil {
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{"error": "Database not initialized"})
	}

	encKey := facades.Config().GetString("APP_ENCRYPTION_KEY")

	whereClauses := []string{
		"uc.revoked_at IS NULL",
		"u.deleted_at IS NULL",
		"c.deleted_at IS NULL",
	}
	var args []any

	if userId != "" {
		whereClauses = append(whereClauses, "uc.user_id = ?")
		args = append(args, userId)
	} else if userEmail != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("(u.name ILIKE ? OR public.pgp_sym_decrypt(u.email::bytea, '%s'::text) = ?)", encKey))
		args = append(args, userEmail, userEmail)
	}

	if companyId != "" {
		whereClauses = append(whereClauses, "uc.company_id = ?")
		args = append(args, companyId)
	} else if companyName != "" {
		whereClauses = append(whereClauses, "c.name ILIKE ?")
		args = append(args, companyName)
	}

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
		WHERE %s
		LIMIT 1
	`, encKey, strings.Join(whereClauses, " AND "))

	var matched UserCompanyRecord
	if err := orm.Query().Raw(query, args...).Scan(&matched); err != nil || matched.MembershipID == "" {
		return ctx.Response().Json(http.StatusNotFound, http.Json{
			"error": "User membership in company not found",
		})
	}

	// Business units for company: query live from database
	var businessUnits []map[string]string
	if matched.CompanyID != "" {
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
