package controllers

import (
	"errors"
	"strconv"

	"github.com/goravel/framework/contracts/http"
	"goravel/app/services"
)

type RbacController struct {
	matrixService      *services.RbacMatrixService
	userCompanyService *services.UserCompanyService
}

func NewRbacController() *RbacController {
	return &RbacController{
		matrixService:      services.NewRbacMatrixService(),
		userCompanyService: services.NewUserCompanyService(),
	}
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
	page, _ := strconv.Atoi(ctx.Request().Query("page"))
	perPage, _ := strconv.Atoi(ctx.Request().Query("per_page"))

	filter := services.UserCompanyFilter{
		Page:    page,
		PerPage: perPage,
		Search:  ctx.Request().Query("search"),
		Role:    ctx.Request().Query("role"),
		Company: ctx.Request().Query("company"),
	}

	result, err := c.userCompanyService.GetPaginatedMemberships(filter)
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{"error": err.Error()})
	}

	return ctx.Response().Success().Json(result)
}

// InspectUserRole inspects the specific role of a user in a company (targeted single query)
func (c *RbacController) InspectUserRole(ctx http.Context) http.Response {
	params := services.InspectRoleParams{
		UserID:      ctx.Request().Query("user_id"),
		UserEmail:   ctx.Request().Query("email"),
		CompanyID:   ctx.Request().Query("company_id"),
		CompanyName: ctx.Request().Query("company_name"),
	}

	result, err := c.userCompanyService.InspectUserRole(params)
	if err != nil {
		if errors.Is(err, services.ErrUserMembershipNotFound) {
			return ctx.Response().Json(http.StatusNotFound, http.Json{"error": err.Error()})
		}
		return ctx.Response().Json(http.StatusInternalServerError, http.Json{"error": err.Error()})
	}

	return ctx.Response().Success().Json(result)
}
