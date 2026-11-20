package routes

import (
	"github.com/goravel/framework/contracts/route"
	"goravel/app/facades"
	"goravel/app/http/controllers"
)

func Api() {
	rbacController := controllers.NewRbacController()

	facades.Route().Prefix("api/v1/rbac").Group(func(router route.Router) {
		router.Get("matrix", rbacController.Matrix)
		router.Get("endpoints", rbacController.Endpoints)
		router.Get("ui-routes", rbacController.UiRoutes)
		router.Get("stats", rbacController.Stats)
		router.Get("simulate", rbacController.Simulate)
		router.Get("user-companies", rbacController.UserCompanies)
		router.Get("user-role", rbacController.InspectUserRole)
	})
}
