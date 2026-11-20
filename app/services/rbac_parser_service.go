package services

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type ParsedData struct {
	Applications    map[string]string            // code -> description/name
	Modules         map[string]ModuleEntry       // module_code -> {AppCode, Code}
	Features        map[string]FeatureEntry      // module_code + ":" + feature_code -> FeatureEntry
	Actions         map[string]bool              // action_code -> true
	FeatureActions  map[string]FeatureActionItem // key: module:feature:action
	RolePermissions map[string][]string          // role_name -> []feature_action_keys
	ApiEndpoints    []EndpointEntry
	UiRoutes        []UiRouteEntry
}

type ModuleEntry struct {
	AppCode string `json:"app_code"`
	Code    string `json:"code"`
}

type FeatureEntry struct {
	ModuleCode      string `json:"module_code"`
	Code            string `json:"code"`
	IsActionFeature bool   `json:"is_action_feature"`
}

type FeatureActionItem struct {
	AppCode    string `json:"app_code"`
	ModuleCode string `json:"module_code"`
	Feature    string `json:"feature"`
	Action     string `json:"action"`
	Key        string `json:"key"` // module:feature:action
}

type EndpointEntry struct {
	ServiceName string `json:"service_name"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	ModuleCode  string `json:"module_code"`
	FeatureCode string `json:"feature_code"`
	ActionCode  string `json:"action_code"`
	Description string `json:"description"`
}

type UiRouteEntry struct {
	Permission  string `json:"permission"`
	ModuleCode  string `json:"module_code"`
	FeatureCode string `json:"feature_code"`
	ActionCode  string `json:"action_code"`
	Description string `json:"description"`
}

type RbacParserService struct {
	cachedData *ParsedData
	mu         sync.RWMutex
}

var parserInstance *RbacParserService
var parserOnce sync.Once

func GetRbacParser() *RbacParserService {
	parserOnce.Do(func() {
		parserInstance = &RbacParserService{}
	})
	return parserInstance
}

func findSeederDir() string {
	candidates := []string{
		filepath.Join("be-groapp-ddl", "seeder", "rbac"),
		filepath.Join("..", "be-groapp-ddl", "seeder", "rbac"),
		filepath.Join("..", "..", "be-groapp-ddl", "seeder", "rbac"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return filepath.Join("be-groapp-ddl", "seeder", "rbac")
}

func (s *RbacParserService) GetData() (*ParsedData, error) {
	s.mu.RLock()
	if s.cachedData != nil {
		defer s.mu.RUnlock()
		return s.cachedData, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedData != nil {
		return s.cachedData, nil
	}

	data, err := s.parseAll()
	if err != nil {
		return nil, err
	}
	s.cachedData = data
	return data, nil
}

func (s *RbacParserService) parseAll() (*ParsedData, error) {
	data := &ParsedData{
		Applications:    make(map[string]string),
		Modules:         make(map[string]ModuleEntry),
		Features:        make(map[string]FeatureEntry),
		Actions:         make(map[string]bool),
		FeatureActions:  make(map[string]FeatureActionItem),
		RolePermissions: make(map[string][]string),
		ApiEndpoints:    make([]EndpointEntry, 0),
		UiRoutes:        make([]UiRouteEntry, 0),
	}

	data.Applications["GROAPP_ACCESS"] = "GroApp Access Service"
	data.Applications["GROAPP_ACCOUNTING"] = "GroApp Accounting Service"

	baseActions := []string{"VIEW", "CREATE", "UPDATE", "DELETE", "ACTIVE", "ARCHIVE", "VOID", "EXPORT", "IMPORT"}
	for _, a := range baseActions {
		data.Actions[a] = true
	}

	roles := []string{
		"Pemilik Usaha",
		"Administrator Perusahaan",
		"Admin Keuangan",
		"Admin Pengadaan",
		"Sales",
	}
	for _, r := range roles {
		data.RolePermissions[r] = make([]string, 0)
	}

	seederDir := findSeederDir()
	rbacAllPath := filepath.Join(seederDir, "rbac-all.sql")

	if _, err := os.Stat(rbacAllPath); err == nil {
		s.parseSqlFile(rbacAllPath, data)
	} else {
		files, _ := filepath.Glob(filepath.Join(seederDir, "*.sql"))
		for _, f := range files {
			s.parseSqlFile(f, data)
		}
	}

	return data, nil
}

func (s *RbacParserService) parseSqlFile(filePath string, data *ParsedData) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	featureToModule := make(map[string]string)
	moduleToApp := make(map[string]string)

	reModuleInsert := regexp.MustCompile(`(?i)SELECT\s+[a-z0-9_.]*,\s*'([A-Z_]+)'\s+FROM\s+applications\s+[a-z0-9_]*\s+WHERE\s+[a-z0-9_.]*code\s*=\s*'([A-Z_]+)'`)
	reFeatureInsert := regexp.MustCompile(`(?i)SELECT\s+[a-z0-9_.]*,\s*'([A-Z0-9_]+)',\s*(true|false)\s+FROM\s+modules\s+[a-z0-9_]*\s+WHERE\s+[a-z0-9_.]*code\s*=\s*'([A-Z_]+)'`)

	// Pass 1: Discover all Modules and Features
	for _, line := range lines {
		if m := reModuleInsert.FindStringSubmatch(line); len(m) > 2 {
			modCode := m[1]
			appCode := m[2]
			moduleToApp[modCode] = appCode
			data.Modules[modCode] = ModuleEntry{AppCode: appCode, Code: modCode}
		}

		if m := reFeatureInsert.FindStringSubmatch(line); len(m) > 3 {
			featCode := m[1]
			isAction := strings.ToLower(m[2]) == "true"
			modCode := m[3]

			appCode := moduleToApp[modCode]
			if appCode == "" {
				appCode = "GROAPP_ACCOUNTING"
			}
			featureToModule[featCode] = modCode

			featKey := modCode + ":" + featCode
			data.Features[featKey] = FeatureEntry{
				ModuleCode:      modCode,
				Code:            featCode,
				IsActionFeature: isAction,
			}
			if _, exists := data.Modules[modCode]; !exists {
				data.Modules[modCode] = ModuleEntry{AppCode: appCode, Code: modCode}
			}
		}
	}

	// Pass 2: Feature Actions, Role Permissions, Endpoints, UI Routes
	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check for Feature Actions / Role Permissions
		if strings.Contains(line, "feature_actions") || strings.Contains(line, "role_permissions") || strings.Contains(line, "f.code") {
			s.scanPermissionBlock(lines, i, featureToModule, moduleToApp, data)
		}

		// Detect Endpoint
		if strings.Contains(line, "INSERT INTO api_endpoints") {
			s.extractEndpointBlock(lines, i, featureToModule, data)
		}

		// Detect UI Route
		if strings.Contains(line, "INSERT INTO ui_routes") {
			s.extractUiRouteBlock(lines, i, featureToModule, data)
		}
	}
}

func (s *RbacParserService) extractEndpointBlock(lines []string, index int, featToMod map[string]string, data *ParsedData) {
	end := index + 10
	if end > len(lines) {
		end = len(lines)
	}
	block := strings.Join(lines[index:end], " ")

	reMethodPath := regexp.MustCompile(`(?i)SELECT\s+'(GET|POST|PUT|DELETE|PATCH)',\s*'([^']+)',\s*fa\.id,\s*'([^']+)'(?:,\s*'([^']*)')?`)
	m := reMethodPath.FindStringSubmatch(block)
	if len(m) < 4 {
		return
	}

	method := m[1]
	path := m[2]
	service := m[3]
	desc := ""
	if len(m) > 4 {
		desc = m[4]
	}

	fMatch := regexp.MustCompile(`f\.code\s*=\s*'([A-Z0-9_]+)'`).FindStringSubmatch(block)
	aMatch := regexp.MustCompile(`a\.code\s*=\s*'([A-Z0-9_]+)'`).FindStringSubmatch(block)

	if len(fMatch) > 1 && len(aMatch) > 1 {
		fCode := fMatch[1]
		aCode := aMatch[1]
		mod := featToMod[fCode]
		if modMatch := regexp.MustCompile(`modules\s+WHERE\s+code\s*=\s*'([A-Z_]+)'`).FindStringSubmatch(block); len(modMatch) > 1 {
			mod = modMatch[1]
		}

		data.ApiEndpoints = append(data.ApiEndpoints, EndpointEntry{
			ServiceName: service,
			Method:      method,
			Path:        path,
			ModuleCode:  mod,
			FeatureCode: fCode,
			ActionCode:  aCode,
			Description: desc,
		})
	}
}

func (s *RbacParserService) extractUiRouteBlock(lines []string, index int, featToMod map[string]string, data *ParsedData) {
	end := index + 10
	if end > len(lines) {
		end = len(lines)
	}
	block := strings.Join(lines[index:end], " ")

	reRoute := regexp.MustCompile(`(?i)SELECT\s+fa\.id,\s*'([^']+)'(?:,\s*'([^']*)')?`)
	m := reRoute.FindStringSubmatch(block)
	if len(m) < 2 {
		return
	}

	perm := m[1]
	desc := ""
	if len(m) > 2 {
		desc = m[2]
	}

	fMatch := regexp.MustCompile(`f\.code\s*=\s*'([A-Z0-9_]+)'`).FindStringSubmatch(block)
	aMatch := regexp.MustCompile(`a\.code\s*=\s*'([A-Z0-9_]+)'`).FindStringSubmatch(block)

	if len(fMatch) > 1 && len(aMatch) > 1 {
		fCode := fMatch[1]
		aCode := aMatch[1]
		mod := featToMod[fCode]
		if modMatch := regexp.MustCompile(`modules\s+WHERE\s+code\s*=\s*'([A-Z_]+)'`).FindStringSubmatch(block); len(modMatch) > 1 {
			mod = modMatch[1]
		}

		data.UiRoutes = append(data.UiRoutes, UiRouteEntry{
			Permission:  perm,
			ModuleCode:  mod,
			FeatureCode: fCode,
			ActionCode:  aCode,
			Description: desc,
		})
	}
}

func (s *RbacParserService) scanPermissionBlock(lines []string, index int, featToMod map[string]string, modToApp map[string]string, data *ParsedData) {
	start := index - 2
	if start < 0 {
		start = 0
	}
	end := index + 8
	if end > len(lines) {
		end = len(lines)
	}

	block := strings.Join(lines[start:end], " ")

	// Extract features
	features := extractQuotedStrings(block, `f\.code\s+(?:IN\s*\(([^)]+)\)|=\s*'([^']+)')`)
	if len(features) == 0 {
		return
	}

	// Extract actions
	actions := extractQuotedStrings(block, `a\.code\s+(?:IN\s*\(([^)]+)\)|=\s*'([^']+)')`)
	if len(actions) == 0 {
		return
	}

	// Find module
	explicitMod := ""
	if m := regexp.MustCompile(`modules\s+WHERE\s+code\s*=\s*'([A-Z_]+)'`).FindStringSubmatch(block); len(m) > 1 {
		explicitMod = m[1]
	}

	// Extract roles if present
	roles := extractQuotedStrings(block, `r\.name\s+(?:IN\s*\(([^)]+)\)|=\s*'([^']+)')`)

	for _, f := range features {
		mod := explicitMod
		if mod == "" {
			mod = featToMod[f]
		}
		if mod == "" {
			continue
		}

		app := modToApp[mod]
		if app == "" {
			app = "GROAPP_ACCOUNTING"
		}

		for _, a := range actions {
			faKey := mod + ":" + f + ":" + a
			data.FeatureActions[faKey] = FeatureActionItem{
				AppCode:    app,
				ModuleCode: mod,
				Feature:    f,
				Action:     a,
				Key:        faKey,
			}

			// Add to role_permissions if roles exist in this block
			for _, r := range roles {
				list := data.RolePermissions[r]
				found := false
				for _, existing := range list {
					if existing == faKey {
						found = true
						break
					}
				}
				if !found {
					data.RolePermissions[r] = append(list, faKey)
				}
			}
		}
	}
}

func (s *RbacParserService) lookAheadWhereClause(lines []string, index int, featToMod map[string]string) (string, string, string) {
	end := index + 10
	if end > len(lines) {
		end = len(lines)
	}

	block := strings.Join(lines[index:end], " ")

	fMatch := regexp.MustCompile(`f\.code\s*=\s*'([A-Z0-9_]+)'`).FindStringSubmatch(block)
	aMatch := regexp.MustCompile(`a\.code\s*=\s*'([A-Z0-9_]+)'`).FindStringSubmatch(block)

	fCode := ""
	aCode := ""
	if len(fMatch) > 1 {
		fCode = fMatch[1]
	}
	if len(aMatch) > 1 {
		aCode = aMatch[1]
	}

	mod := featToMod[fCode]
	if m := regexp.MustCompile(`modules\s+WHERE\s+code\s*=\s*'([A-Z_]+)'`).FindStringSubmatch(block); len(m) > 1 {
		mod = m[1]
	}

	return fCode, aCode, mod
}

func extractQuotedStrings(text string, pattern string) []string {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(text)
	if len(matches) == 0 {
		return nil
	}

	var raw string
	if len(matches) > 1 && matches[1] != "" {
		raw = matches[1]
	} else if len(matches) > 2 && matches[2] != "" {
		raw = matches[2]
	}

	quoteRe := regexp.MustCompile(`'([^']+)'`)
	qMatches := quoteRe.FindAllStringSubmatch(raw, -1)
	if len(qMatches) > 0 {
		var results []string
		for _, qm := range qMatches {
			if len(qm) > 1 {
				results = append(results, qm[1])
			}
		}
		return results
	}

	trimmed := strings.Trim(raw, " '\"")
	if trimmed != "" {
		return []string{trimmed}
	}

	return nil
}
