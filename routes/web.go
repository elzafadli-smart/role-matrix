package routes

import (
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/support"

	"goravel/app/facades"
	"goravel/app/http/controllers"
)

func Web() {
	// 1. User & Company Role Inspector (Landing Page)
	facades.Route().Get("/", func(ctx http.Context) http.Response {
		return ctx.Response().View().Make("inspector.tmpl", map[string]any{
			"active":  "inspector",
			"version": support.Version,
		})
	})
	facades.Route().Get("/inspector", func(ctx http.Context) http.Response {
		return ctx.Response().View().Make("inspector.tmpl", map[string]any{
			"active":  "inspector",
			"version": support.Version,
		})
	})

	// 2. RBAC Hierarchical Matrix Grid
	facades.Route().Get("/matrix", func(ctx http.Context) http.Response {
		return ctx.Response().View().Make("matrix.tmpl", map[string]any{
			"active":  "matrix",
			"version": support.Version,
		})
	})

	// 3. API Endpoints Matrix Grid
	facades.Route().Get("/endpoints", func(ctx http.Context) http.Response {
		return ctx.Response().View().Make("endpoints.tmpl", map[string]any{
			"active":  "endpoints",
			"version": support.Version,
		})
	})

	// 4. Frontend UI Routes Matrix Grid
	facades.Route().Get("/ui-routes", func(ctx http.Context) http.Response {
		return ctx.Response().View().Make("ui_routes.tmpl", map[string]any{
			"active":  "ui-routes",
			"version": support.Version,
		})
	})

	facades.Route().Static("public", "./public")

	userController := controllers.NewUserController()
	facades.Route().Get("/users", userController.Index)
}
