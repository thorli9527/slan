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

func main() {
	target := flag.String("target", "web", "validation target: web")
	root := flag.String("root", ".", "repository root")
	flag.Parse()

	specs, err := loadContractSpecs(filepath.Join(*root, "protocol", "contracts"))
	if err != nil {
		fatal(err)
	}

	switch strings.TrimSpace(*target) {
	case "web":
		err = checkWebContracts(
			specs,
			filepath.Join(*root, "server", "server-ui", "web", "src", "ui", "api-contracts.ts"),
		)
	case "flutter":
		err = checkFlutterContracts(
			specs,
			filepath.Join(*root, "client", "app", "lib", "infra", "api_contracts", "request_models.dart"),
			filepath.Join(*root, "client", "app", "lib", "infra", "control_api_responses", "response_dtos.dart"),
		)
	default:
		err = fmt.Errorf("unsupported target %q", *target)
	}
	if err != nil {
		fatal(err)
	}

	fmt.Printf("protocol contract check passed for target=%s\n", *target)
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

	responseNameMap := map[string]string{
		"AuthResponse":                "AuthResponseDto",
		"CompleteAuthCallbackRequest": "CompleteAuthCallbackRequestDto",
		"AuthCallbackStatusResponse":  "AuthCallbackStatusResponseDto",
		"Node":                        "NodeResponseDto",
		"ControlPlaneConfig":          "ControlPlaneConfigResponseDto",
		"RelayTicket":                 "RelayTicketResponseDto",
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

		responseName, ok := responseNameMap[spec.Name]
		if !ok {
			continue
		}
		fields, ok := responses[responseName]
		if !ok {
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
