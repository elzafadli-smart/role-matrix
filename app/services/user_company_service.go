package services

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"goravel/app/facades"
)

var (
	ErrDatabaseNotInitialized   = errors.New("database connection not initialized")
	ErrUserMembershipNotFound   = errors.New("user membership in company not found")
)

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

type UserCompanyFilter struct {
	Page    int
	PerPage int
	Search  string
	Role    string
	Company string
}

type UserCompanyPaginatedResult struct {
	Total            int64               `json:"total"`
	TotalMemberships int64               `json:"total_memberships"`
	Page             int                 `json:"page"`
	PerPage          int                 `json:"per_page"`
	TotalPages       int                 `json:"total_pages"`
	Roles            []string            `json:"roles"`
	Companies        []string            `json:"companies"`
	Memberships      []UserCompanyRecord `json:"memberships"`
}

type InspectRoleParams struct {
	UserID      string
	UserEmail   string
	CompanyID   string
	CompanyName string
}

type UserRoleInspectionResult struct {
	MembershipID  string              `json:"membership_id"`
	User          map[string]any      `json:"user"`
	Company       map[string]any      `json:"company"`
	Role          map[string]any      `json:"role"`
	AccessScope   map[string]any      `json:"access_scope"`
	BusinessUnits []map[string]string `json:"business_units"`
}

type UserCompanyService struct{}

func NewUserCompanyService() *UserCompanyService {
	return &UserCompanyService{}
}

// GetPaginatedMemberships retrieves paginated memberships with optional filters and distinct options for modals
func (s *UserCompanyService) GetPaginatedMemberships(filter UserCompanyFilter) (*UserCompanyPaginatedResult, error) {
	orm := facades.Orm()
	if orm == nil {
		return nil, ErrDatabaseNotInitialized
	}

	encKey := facades.Config().GetString("APP_ENCRYPTION_KEY")

	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 10
	}
	if perPage > 100 {
		perPage = 100
	}
	offset := (page - 1) * perPage

	search := strings.TrimSpace(filter.Search)
	role := strings.TrimSpace(filter.Role)
	company := strings.TrimSpace(filter.Company)

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

	// 1. Count total matching records
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
		return nil, err
	}

	// 2. Fetch paginated memberships
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
		return nil, err
	}

	totalPages := 1
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(perPage)))
	}

	// 3. Fetch distinct filter options
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

	return &UserCompanyPaginatedResult{
		Total:            total,
		TotalMemberships: total,
		Page:             page,
		PerPage:          perPage,
		TotalPages:       totalPages,
		Roles:            roles,
		Companies:        companies,
		Memberships:      records,
	}, nil
}

// InspectUserRole inspects the specific role of a user in a company
func (s *UserCompanyService) InspectUserRole(params InspectRoleParams) (*UserRoleInspectionResult, error) {
	orm := facades.Orm()
	if orm == nil {
		return nil, ErrDatabaseNotInitialized
	}

	encKey := facades.Config().GetString("APP_ENCRYPTION_KEY")

	userId := strings.TrimSpace(params.UserID)
	userEmail := strings.TrimSpace(params.UserEmail)
	companyId := strings.TrimSpace(params.CompanyID)
	companyName := strings.TrimSpace(params.CompanyName)

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
		return nil, ErrUserMembershipNotFound
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

	return &UserRoleInspectionResult{
		MembershipID: matched.MembershipID,
		User: map[string]any{
			"id":    matched.UserID,
			"name":  matched.UserName,
			"email": matched.Email,
		},
		Company: map[string]any{
			"id":            matched.CompanyID,
			"name":          matched.CompanyName,
			"business_type": matched.BusinessType,
		},
		Role: map[string]any{
			"id":   matched.RoleID,
			"name": matched.RoleName,
		},
		AccessScope: map[string]any{
			"company_access":           matched.CompanyAccess,
			"unit_access_mode":        matched.UnitAccessMode,
			"auto_assign_future_units": matched.AutoAssignFutureUnits,
			"joined_at":                matched.JoinedAt,
			"revoked_at":               matched.RevokedAt,
			"is_active":                matched.RevokedAt == nil,
		},
		BusinessUnits: businessUnits,
	}, nil
}
