package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type contractSpec struct {
	Name   string
	Fields []string
}

type targetFlags []string

func (flags *targetFlags) String() string {
	if flags == nil || len(*flags) == 0 {
		return ""
	}
	return strings.Join(*flags, ",")
}

func (flags *targetFlags) Set(value string) error {
	for _, target := range strings.Split(value, ",") {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		*flags = append(*flags, target)
	}
	return nil
}

func main() {
	root := flag.String("root", ".", "repository root")
	var targets targetFlags
	flag.Var(&targets, "target", "validation target, repeatable or comma-separated: web, flutter, rust-controller, go-server, openapi, protobuf, http-routes")
	flag.Parse()
	if len(targets) == 0 {
		targets = targetFlags{"web"}
	}

	specs, err := loadContractSpecs(filepath.Join(*root, "protocol", "contracts"))
	if err != nil {
		fatal(err)
	}

	for _, target := range targets {
		if err := checkTarget(specs, *root, target); err != nil {
			fatal(err)
		}
		fmt.Printf("protocol contract check passed for target=%s\n", target)
	}
}

func checkTarget(specs []contractSpec, root, target string) error {
	switch strings.TrimSpace(target) {
	case "web":
		return checkWebContracts(
			specs,
			filepath.Join(root, "server", "server-ui", "web", "src", "ui", "api-contracts.ts"),
		)
	case "flutter":
		return checkFlutterContracts(
			specs,
			filepath.Join(root, "client", "app", "lib", "infra", "api_contracts", "request_models.dart"),
			filepath.Join(root, "client", "app", "lib", "infra", "control_api_responses", "response_dtos.dart"),
		)
	case "rust-controller":
		return checkRustControllerContracts(
			specs,
			filepath.Join(root, "client", "app_core", "crates", "controller-client", "src", "dto.rs"),
		)
	case "go-server":
		return checkGoServerContracts(
			specs,
			filepath.Join(root, "server", "server-biz", "api", "dto"),
		)
	case "openapi":
		return checkOpenAPIContracts(
			specs,
			filepath.Join(root, "protocol", "openapi", "phase1.yaml"),
		)
	case "protobuf":
		return checkProtobufContracts(
			specs,
			filepath.Join(root, "protocol", "protobuf", "control.proto"),
		)
	case "http-routes":
		return checkHTTPRoutes(
			filepath.Join(root, "server", "server-biz", "api", "http"),
			filepath.Join(root, "protocol", "openapi", "phase1.yaml"),
		)
	default:
		return fmt.Errorf("unsupported target %q", target)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func loadContractSpecs(dir string) ([]contractSpec, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read protocol contracts: %w", err)
	}

	var specs []contractSpec
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		loaded, err := parseContractSpecFile(path)
		if err != nil {
			return nil, err
		}
		specs = append(specs, loaded...)
	}

	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
	return specs, nil
}

func parseContractSpecFile(path string) ([]contractSpec, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	var (
		specs          []contractSpec
		current        *contractSpec
		inContracts    bool
		inFields       bool
		contractIndent = regexp.MustCompile(`^  ([A-Za-z0-9]+):\s*$`)
		fieldIndent    = regexp.MustCompile(`^      ([A-Za-z0-9]+):\s*$`)
	)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "contracts:" {
			inContracts = true
			inFields = false
			continue
		}
		if !inContracts {
			continue
		}
		if strings.HasPrefix(line, "    fields:") {
			inFields = true
			continue
		}
		if matches := contractIndent.FindStringSubmatch(line); len(matches) == 2 {
			specs = append(specs, contractSpec{Name: matches[1]})
			current = &specs[len(specs)-1]
			inFields = false
			continue
		}
		if inFields && current != nil {
			if matches := fieldIndent.FindStringSubmatch(line); len(matches) == 2 {
				current.Fields = append(current.Fields, matches[1])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return specs, nil
}

func checkWebContracts(specs []contractSpec, path string) error {
	exports, err := parseTypeScriptExports(path)
	if err != nil {
		return err
	}

	var problems []string
	for _, spec := range specs {
		fields, ok := exports[spec.Name]
		if !ok {
			continue
		}
		for _, requiredField := range spec.Fields {
			if _, ok := fields[requiredField]; !ok {
				problems = append(
					problems,
					fmt.Sprintf("%s missing field %q in %s", path, requiredField, spec.Name),
				)
			}
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("protocol drift detected:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

func checkFlutterContracts(specs []contractSpec, requestPath, responsePath string) error {
	requests, err := parseDartClassFields(requestPath)
	if err != nil {
		return err
	}
	responses, err := parseDartClassFields(responsePath)
	if err != nil {
		return err
	}

	requestNameMap := map[string]string{
		"AttachDeviceRequest":              "AttachDeviceRequest",
		"BootstrapRequest":                 "BootstrapRequest",
		"CreateNetworkRequest":             "CreateNetworkRequest",
		"CreateSubnetRequest":              "CreateSubnetRequest",
		"DeactivateNetworkRequest":         "DeactivateNetworkRequest",
		"JoinNetworkByKeyRequest":          "JoinNetworkByKeyRequest",
		"JoinNetworkByOwnerEmailRequest":   "JoinNetworkByOwnerEmailRequest",
		"JoinNetworkRequest":               "JoinNetworkRequest",
		"LoginRequest":                     "LoginRequest",
		"RefreshTokenRequest":              "RefreshTokenRequest",
		"RegisterDeviceRequest":            "RegisterDeviceRequest",
		"RegisterNodeRequest":              "RegisterNodeRequest",
		"RegisterRequest":                  "RegisterRequest",
		"RelayTicketRequest":               "RelayTicketRequest",
		"SwitchNetworkRequest":             "SwitchNetworkRequest",
		"UpdateAttachmentIPRequest":        "UpdateAttachmentIPRequest",
		"UpdateAttachmentRemarkRequest":    "UpdateAttachmentRemarkRequest",
		"UpdateNetworkDNSRequest":          "UpdateNetworkDNSRequest",
		"UpdateNetworkRequest":             "UpdateNetworkRequest",
		"UpdateNetworkMemberStatusRequest": "UpdateNetworkMemberStatusRequest",
	}

	responseNameMap := map[string]string{
		"AuthResponse":                  "AuthResponseDto",
		"CompleteAuthCallbackRequest":   "CompleteAuthCallbackRequestDto",
		"AuthCallbackStatusResponse":    "AuthCallbackStatusResponseDto",
		"BootstrapResponse":             "BootstrapResponseDto",
		"DerpCluster":                   "DerpClusterResponseDto",
		"DerpMap":                       "DerpMapResponseDto",
		"DerpNode":                      "DerpNodeResponseDto",
		"Device":                        "DeviceResponseDto",
		"DeviceBootstrap":               "DeviceBootstrapResponseDto",
		"Node":                          "NodeResponseDto",
		"ControlPlaneConfig":            "ControlPlaneConfigResponseDto",
		"DNSConfig":                     "DNSConfigResponseDto",
		"Endpoint":                      "EndpointResponseDto",
		"Network":                       "NetworkSummaryResponseDto",
		"NetworkAssignment":             "NetworkAssignmentResponseDto",
		"NetworkDetail":                 "NetworkDetailResponseDto",
		"NetworkJoinByOwnerEmailResult": "NetworkJoinByOwnerEmailResultResponseDto",
		"NetworkJoinResult":             "NetworkJoinResultResponseDto",
		"NetworkMap":                    "NetworkMapResponseDto",
		"NetworkMember":                 "NetworkMemberResponseDto",
		"Peer":                          "PeerResponseDto",
		"RelayEndpoint":                 "RelayEndpointResponseDto",
		"RelayRegion":                   "RelayRegionResponseDto",
		"RelayTicket":                   "RelayTicketResponseDto",
		"Route":                         "RouteResponseDto",
		"Subnet":                        "SubnetResponseDto",
		"SubnetAttachment":              "SubnetAttachmentResponseDto",
	}

	var problems []string
	for _, spec := range specs {
		if fields, ok := requests[spec.Name]; ok {
			for _, requiredField := range spec.Fields {
				if _, ok := fields[requiredField]; !ok {
					problems = append(
						problems,
						fmt.Sprintf("%s missing field %q in %s", requestPath, requiredField, spec.Name),
					)
				}
			}
			continue
		}
		if className, ok := requestNameMap[spec.Name]; ok {
			problems = append(
				problems,
				fmt.Sprintf("%s missing request class %s", requestPath, className),
			)
			continue
		}

		responseName, ok := responseNameMap[spec.Name]
		if !ok {
			continue
		}
		fields, ok := responses[responseName]
		if !ok {
			problems = append(
				problems,
				fmt.Sprintf("%s missing response class %s", responsePath, responseName),
			)
			continue
		}
		for _, requiredField := range spec.Fields {
			if _, ok := fields[requiredField]; !ok {
				problems = append(
					problems,
					fmt.Sprintf("%s missing field %q in %s", responsePath, requiredField, responseName),
				)
			}
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("protocol drift detected:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

func checkRustControllerContracts(specs []contractSpec, path string) error {
	structs, err := parseRustStructFields(path)
	if err != nil {
		return err
	}

	nameMap := map[string]string{
		"AuthResponse":                  "AuthResponseDto",
		"BootstrapRequest":              "BootstrapRequestDto",
		"BootstrapResponse":             "BootstrapResponseDto",
		"ControlPlaneConfig":            "ControlPlaneConfigDto",
		"CreateNetworkRequest":          "CreateNetworkRequestDto",
		"DeactivateNetworkRequest":      "DeactivateNetworkRequestDto",
		"DerpCluster":                   "DerpClusterDto",
		"DerpMap":                       "DerpMapDto",
		"DerpNode":                      "DerpNodeDto",
		"Device":                        "DeviceDto",
		"DeviceBootstrap":               "BootstrapDeviceDto",
		"DNSConfig":                     "DnsConfigDto",
		"Endpoint":                      "EndpointDto",
		"JoinNetworkRequest":            "JoinNetworkRequestDto",
		"LoginRequest":                  "LoginRequestDto",
		"Network":                       "NetworkDto",
		"NetworkJoinByOwnerEmailResult": "NetworkJoinByOwnerEmailResultDto",
		"NetworkJoinResult":             "NetworkJoinResultDto",
		"NetworkMap":                    "NetworkMapDto",
		"NetworkMember":                 "NetworkMemberDto",
		"Node":                          "NodeDto",
		"Peer":                          "PeerDto",
		"RefreshTokenRequest":           "RefreshTokenRequestDto",
		"RegisterDeviceRequest":         "RegisterDeviceRequestDto",
		"RegisterNodeRequest":           "RegisterNodeRequestDto",
		"RegisterRequest":               "RegisterRequestDto",
		"RelayCity":                     "RelayCityDto",
		"RelayCluster":                  "RelayClusterDto",
		"RelayConfig":                   "RelayConfigDto",
		"RelayCountry":                  "RelayCountryDto",
		"RelayEndpoint":                 "RelayEndpointDto",
		"RelayNode":                     "RelayNodeDto",
		"RelayRegion":                   "RelayRegionDto",
		"RelayTicket":                   "RelayTicketDto",
		"RelayTicketRequest":            "RelayTicketRequestDto",
		"Route":                         "RouteDto",
		"Subnet":                        "SubnetDto",
		"SubnetAttachment":              "SubnetAttachmentDto",
		"SwitchNetworkRequest":          "SwitchNetworkRequestDto",
		"UpdateNetworkDNSRequest":       "UpdateNetworkDNSRequestDto",
	}

	var problems []string
	for _, spec := range specs {
		structName := spec.Name + "Dto"
		if mapped, ok := nameMap[spec.Name]; ok {
			structName = mapped
		}
		fields, ok := structs[structName]
		if !ok {
			continue
		}
		for _, requiredField := range spec.Fields {
			if _, ok := fields[requiredField]; !ok {
				problems = append(
					problems,
					fmt.Sprintf("%s missing field %q in %s", path, requiredField, structName),
				)
			}
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("protocol drift detected:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

func checkGoServerContracts(specs []contractSpec, dir string) error {
	structs, err := parseGoStructFields(dir)
	if err != nil {
		return err
	}

	var problems []string
	for _, spec := range specs {
		fields, ok := structs[spec.Name]
		if !ok {
			continue
		}
		for _, requiredField := range spec.Fields {
			if _, ok := fields[requiredField]; !ok {
				problems = append(
					problems,
					fmt.Sprintf("%s missing field %q in %s", dir, requiredField, spec.Name),
				)
			}
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("protocol drift detected:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

func checkOpenAPIContracts(specs []contractSpec, path string) error {
	schemas, err := parseOpenAPISchemaFields(path)
	if err != nil {
		return err
	}

	var problems []string
	for _, spec := range specs {
		fields, ok := schemas[spec.Name]
		if !ok {
			continue
		}
		for _, requiredField := range spec.Fields {
			if _, ok := fields[requiredField]; !ok {
				problems = append(
					problems,
					fmt.Sprintf("%s missing field %q in %s", path, requiredField, spec.Name),
				)
			}
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("protocol drift detected:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

func checkProtobufContracts(specs []contractSpec, path string) error {
	messages, err := parseProtobufMessageFields(path)
	if err != nil {
		return err
	}

	nameMap := map[string]string{
		"DNSConfig": "DNSConfig",
	}

	var problems []string
	for _, spec := range specs {
		messageName := spec.Name
		if mapped, ok := nameMap[spec.Name]; ok {
			messageName = mapped
		}
		fields, ok := messages[messageName]
		if !ok {
			continue
		}
		for _, requiredField := range spec.Fields {
			if _, ok := fields[requiredField]; !ok {
				problems = append(
					problems,
					fmt.Sprintf("%s missing field %q in %s", path, requiredField, messageName),
				)
			}
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("protocol drift detected:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

type routeSpec struct {
	Method string
	Path   string
}

func (route routeSpec) String() string {
	return route.Method + " " + route.Path
}

func checkHTTPRoutes(routesDir, openAPIPath string) error {
	serverRoutes, err := parsePublicGoRoutes(routesDir)
	if err != nil {
		return err
	}
	openAPIRoutes, err := parseOpenAPIPaths(openAPIPath)
	if err != nil {
		return err
	}

	var problems []string
	for route := range serverRoutes {
		if _, ok := openAPIRoutes[route]; !ok {
			problems = append(problems, fmt.Sprintf("%s missing route %s", openAPIPath, route.String()))
		}
	}
	for route := range openAPIRoutes {
		if _, ok := serverRoutes[route]; !ok {
			problems = append(problems, fmt.Sprintf("%s documents route not found in public router: %s", openAPIPath, route.String()))
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("HTTP route drift detected:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

func parseTypeScriptExports(path string) (map[string]map[string]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	typeDecl := regexp.MustCompile(`^export type ([A-Za-z0-9]+) = \{\s*$`)
	fieldDecl := regexp.MustCompile(`^([A-Za-z0-9]+)\??:\s*.+$`)

	exports := map[string]map[string]struct{}{}
	var (
		currentType string
		braceDepth  int
	)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if currentType == "" {
			if matches := typeDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
				currentType = matches[1]
				exports[currentType] = map[string]struct{}{}
				braceDepth = 1
			}
			continue
		}

		if braceDepth == 1 {
			leading := len(line) - len(strings.TrimLeft(line, " "))
			if leading == 2 {
				if matches := fieldDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
					exports[currentType][matches[1]] = struct{}{}
				}
			}
		}

		braceDepth += strings.Count(line, "{")
		braceDepth -= strings.Count(line, "}")
		if braceDepth == 0 {
			currentType = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return exports, nil
}

func parseGoStructFields(dir string) (map[string]map[string]struct{}, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	structs := map[string]map[string]struct{}{}
	embeds := map[string][]string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		if err := scanGoStructFields(file, structs, embeds); err != nil {
			file.Close()
			return nil, fmt.Errorf("scan %s: %w", path, err)
		}
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("close %s: %w", path, err)
		}
	}
	for structName := range structs {
		mergeEmbeddedGoFields(structName, structs, embeds, map[string]struct{}{})
	}
	return structs, nil
}

func parseOpenAPISchemaFields(path string) (map[string]map[string]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	schemaDecl := regexp.MustCompile(`^    ([A-Za-z0-9]+):\s*$`)
	propertyDecl := regexp.MustCompile(`^([A-Za-z0-9]+):\s*$`)
	allOfRefDecl := regexp.MustCompile(`^- \$ref: '#/components/schemas/([A-Za-z0-9]+)'$`)

	schemas := map[string]map[string]struct{}{}
	embeds := map[string][]string{}
	var currentSchema string
	var inSchemas bool
	var inProperties bool
	var propertiesIndent int

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))

		if trimmed == "schemas:" {
			inSchemas = true
			continue
		}
		if !inSchemas {
			continue
		}

		if matches := schemaDecl.FindStringSubmatch(line); len(matches) == 2 {
			currentSchema = matches[1]
			schemas[currentSchema] = map[string]struct{}{}
			inProperties = false
			propertiesIndent = 0
			continue
		}
		if currentSchema == "" {
			continue
		}

		if matches := allOfRefDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
			embeds[currentSchema] = append(embeds[currentSchema], matches[1])
			continue
		}

		if trimmed == "properties:" {
			inProperties = true
			propertiesIndent = indent
			continue
		}
		if inProperties && indent <= propertiesIndent && trimmed != "" {
			inProperties = false
		}
		if !inProperties || indent != propertiesIndent+2 {
			continue
		}

		if matches := propertyDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
			schemas[currentSchema][matches[1]] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	for schemaName := range schemas {
		mergeEmbeddedGoFields(schemaName, schemas, embeds, map[string]struct{}{})
	}
	return schemas, nil
}

func parseOpenAPIPaths(path string) (map[routeSpec]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	pathDecl := regexp.MustCompile(`^  (/[^:]+):\s*$`)
	methodDecl := regexp.MustCompile(`^    (get|post|put|delete|patch):\s*$`)

	routes := map[routeSpec]struct{}{}
	var currentPath string
	var inPaths bool

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "paths:" {
			inPaths = true
			continue
		}
		if !inPaths {
			continue
		}
		if trimmed == "components:" {
			break
		}
		if matches := pathDecl.FindStringSubmatch(line); len(matches) == 2 {
			currentPath = matches[1]
			continue
		}
		if currentPath == "" {
			continue
		}
		if matches := methodDecl.FindStringSubmatch(line); len(matches) == 2 {
			routes[routeSpec{
				Method: strings.ToUpper(matches[1]),
				Path:   currentPath,
			}] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return routes, nil
}

func parsePublicGoRoutes(dir string) (map[routeSpec]struct{}, error) {
	files := []string{
		"routes.go",
		"routes_business_access.go",
		"routes_business_registration.go",
		"routes_business_network.go",
		"routes_business_bootstrap.go",
		"control_ws_sync.go",
	}

	routes := map[routeSpec]struct{}{}
	for _, name := range files {
		path := filepath.Join(dir, name)
		fileRoutes, err := parseGoRoutesFile(path)
		if err != nil {
			return nil, err
		}
		for route := range fileRoutes {
			routes[route] = struct{}{}
		}
	}
	return routes, nil
}

func parseGoRoutesFile(path string) (map[routeSpec]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	groupDecl := regexp.MustCompile(`^([A-Za-z0-9_]+)\s*:=\s*([A-Za-z0-9_]+)\.Group\("([^"]*)"\)`)
	routeDecl := regexp.MustCompile(`^([A-Za-z0-9_]+)\.(GET|POST|PUT|DELETE|PATCH)\("([^"]*)"`)
	controlWSDecl := regexp.MustCompile(`^router\.GET\(path,`)

	prefixes := map[string]string{
		"api":       "",
		"protected": "",
		"router":    "",
	}
	routes := map[routeSpec]struct{}{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if matches := groupDecl.FindStringSubmatch(trimmed); len(matches) == 4 {
			basePrefix := prefixes[matches[2]]
			prefixes[matches[1]] = joinRoutePath(basePrefix, matches[3])
			continue
		}
		if matches := routeDecl.FindStringSubmatch(trimmed); len(matches) == 4 {
			basePrefix, ok := prefixes[matches[1]]
			if !ok {
				continue
			}
			routes[routeSpec{
				Method: matches[2],
				Path:   normalizeGinPath(joinRoutePath(basePrefix, matches[3])),
			}] = struct{}{}
			continue
		}
		if controlWSDecl.MatchString(trimmed) {
			routes[routeSpec{Method: "GET", Path: "/control/ws"}] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return routes, nil
}

func joinRoutePath(prefix, suffix string) string {
	prefix = strings.TrimRight(prefix, "/")
	suffix = strings.TrimLeft(suffix, "/")
	switch {
	case prefix == "" && suffix == "":
		return "/"
	case prefix == "":
		return "/" + suffix
	case suffix == "":
		return prefix
	default:
		return prefix + "/" + suffix
	}
}

func normalizeGinPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func parseProtobufMessageFields(path string) (map[string]map[string]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	messageDecl := regexp.MustCompile(`^message ([A-Za-z0-9]+) \{\s*$`)
	fieldDecl := regexp.MustCompile(`^(?:repeated\s+)?[A-Za-z0-9.<>]+ ([a-zA-Z0-9_]+)\s*=\s*[0-9]+`)

	messages := map[string]map[string]struct{}{}
	var currentMessage string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if currentMessage == "" {
			if matches := messageDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
				currentMessage = matches[1]
				messages[currentMessage] = map[string]struct{}{}
			}
			continue
		}

		if trimmed == "}" {
			currentMessage = ""
			continue
		}

		if matches := fieldDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
			messages[currentMessage][snakeToCamel(matches[1])] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return messages, nil
}

func scanGoStructFields(
	file *os.File,
	structs map[string]map[string]struct{},
	embeds map[string][]string,
) error {
	structDecl := regexp.MustCompile(`^type ([A-Za-z0-9]+) struct \{\s*$`)
	jsonTagDecl := regexp.MustCompile("`json:\"([^\",]+)")
	embeddedDecl := regexp.MustCompile(`^\*?([A-Z][A-Za-z0-9]+)\s*$`)

	var currentStruct string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if currentStruct == "" {
			if matches := structDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
				currentStruct = matches[1]
				structs[currentStruct] = map[string]struct{}{}
			}
			continue
		}

		if trimmed == "}" {
			currentStruct = ""
			continue
		}

		if matches := jsonTagDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
			if matches[1] != "-" {
				structs[currentStruct][matches[1]] = struct{}{}
			}
			continue
		}

		if matches := embeddedDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
			embeds[currentStruct] = append(embeds[currentStruct], matches[1])
		}
	}
	return scanner.Err()
}

func mergeEmbeddedGoFields(
	structName string,
	structs map[string]map[string]struct{},
	embeds map[string][]string,
	visiting map[string]struct{},
) {
	if _, seen := visiting[structName]; seen {
		return
	}
	visiting[structName] = struct{}{}
	defer delete(visiting, structName)

	for _, embeddedName := range embeds[structName] {
		embeddedFields, ok := structs[embeddedName]
		if !ok {
			continue
		}
		mergeEmbeddedGoFields(embeddedName, structs, embeds, visiting)
		for field := range embeddedFields {
			structs[structName][field] = struct{}{}
		}
	}
}

func parseRustStructFields(path string) (map[string]map[string]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	structDecl := regexp.MustCompile(`^pub struct ([A-Za-z0-9]+) \{\s*$`)
	fieldDecl := regexp.MustCompile(`^pub ([A-Za-z0-9_]+):\s*.+,?\s*$`)
	renameDecl := regexp.MustCompile(`^#\[serde\(rename = "([A-Za-z0-9]+)"\)\]$`)

	structs := map[string]map[string]struct{}{}
	var currentStruct string
	var pendingRename string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())

		if currentStruct == "" {
			if matches := structDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
				currentStruct = matches[1]
				structs[currentStruct] = map[string]struct{}{}
				pendingRename = ""
			}
			continue
		}

		if trimmed == "}" {
			currentStruct = ""
			pendingRename = ""
			continue
		}

		if matches := renameDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
			pendingRename = matches[1]
			continue
		}

		if matches := fieldDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
			fieldName := pendingRename
			if fieldName == "" {
				fieldName = snakeToCamel(matches[1])
			}
			structs[currentStruct][fieldName] = struct{}{}
			pendingRename = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return structs, nil
}

func snakeToCamel(value string) string {
	parts := strings.Split(value, "_")
	if len(parts) == 1 {
		return value
	}
	var builder strings.Builder
	builder.WriteString(parts[0])
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		builder.WriteString(part[1:])
	}
	return builder.String()
}

func parseDartClassFields(path string) (map[string]map[string]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	classDecl := regexp.MustCompile(`^class ([A-Za-z0-9]+) \{\s*$`)
	fieldDecl := regexp.MustCompile(`^final [A-Za-z0-9_<>,? ]+ ([A-Za-z0-9]+);$`)

	classes := map[string]map[string]struct{}{}
	var currentClass string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if currentClass == "" {
			if matches := classDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
				currentClass = matches[1]
				classes[currentClass] = map[string]struct{}{}
			}
			continue
		}

		if trimmed == "}" {
			currentClass = ""
			continue
		}

		if matches := fieldDecl.FindStringSubmatch(trimmed); len(matches) == 2 {
			classes[currentClass][matches[1]] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return classes, nil
}
