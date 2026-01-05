package caddy

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/fastcp/fastcp/internal/config"
	"github.com/fastcp/fastcp/internal/models"
)

// Generator generates Caddyfile configurations
type Generator struct {
	templatesDir string
	outputDir    string
}

// NewGenerator creates a new Caddyfile generator
func NewGenerator(templatesDir, outputDir string) *Generator {
	return &Generator{
		templatesDir: templatesDir,
		outputDir:    outputDir,
	}
}

// GenerateMainProxy generates the main reverse proxy Caddyfile
func (g *Generator) GenerateMainProxy(sites []models.Site, phpVersions []models.PHPVersionConfig, httpPort, httpsPort int) (string, error) {
	var buf bytes.Buffer
	cfg := config.Get()

	logPath := filepath.Join(cfg.LogDir, "caddy-proxy.log")

	// Global options
	buf.WriteString(fmt.Sprintf(`# FastCP Main Proxy Configuration
# Auto-generated - Do not edit manually

{
	admin localhost:2019
	
	# Disable automatic HTTPS for local development
	auto_https off
	
	# HTTP port configuration
	http_port %d
	https_port %d
	
	log {
		output file %s {
			roll_size 100mb
			roll_keep 5
		}
		format json
	}
}

`, httpPort, httpsPort, logPath))

	// Find port for each PHP version
	versionPorts := make(map[string]int)
	for _, pv := range phpVersions {
		if pv.Enabled {
			versionPorts[pv.Version] = pv.Port
		}
	}

	// Generate site blocks for each active site
	for _, site := range sites {
		if site.Status != "active" {
			continue
		}

		port, ok := versionPorts[site.PHPVersion]
		if !ok {
			continue
		}

		// Domain(s) for this site
		domains := []string{site.Domain}
		domains = append(domains, site.Aliases...)

		buf.WriteString(fmt.Sprintf("# Site: %s (PHP %s)\n", site.Name, site.PHPVersion))
		
		// Use http:// prefix to disable automatic HTTPS
		for i, d := range domains {
			domains[i] = "http://" + d
		}
		buf.WriteString(strings.Join(domains, ", "))
		buf.WriteString(" {\n")

		// Reverse proxy to PHP instance
		buf.WriteString(fmt.Sprintf("\treverse_proxy localhost:%d\n", port))

		buf.WriteString("}\n\n")
	}

	// Default fallback for unmatched domains (catch-all on the HTTP port)
	buf.WriteString(fmt.Sprintf(`# Default fallback for unmatched domains
:%d {
	respond "Site not found. Configure your domain in FastCP." 404
}
`, httpPort))

	return buf.String(), nil
}

// GeneratePHPInstance generates a Caddyfile for a specific PHP version instance
func (g *Generator) GeneratePHPInstance(version string, port, adminPort int, sites []models.Site) (string, error) {
	var buf bytes.Buffer
	cfg := config.Get()

	// Filter sites for this PHP version
	var versionSites []models.Site
	for _, site := range sites {
		if site.PHPVersion == version && site.Status == "active" {
			versionSites = append(versionSites, site)
		}
	}

	logPath := filepath.Join(cfg.LogDir, fmt.Sprintf("php-%s.log", version))

	// Global options
	buf.WriteString(fmt.Sprintf(`# FastCP PHP %s Instance Configuration
# Auto-generated - Do not edit manually

{
	admin localhost:%d
	
	frankenphp
	
	log {
		output file %s {
			roll_size 100mb
			roll_keep 5
		}
		format json
	}
}

`, version, adminPort, logPath))

	// If no sites, create a minimal placeholder config
	if len(versionSites) == 0 {
		buf.WriteString(fmt.Sprintf("# No sites configured for PHP %s\n", version))
		buf.WriteString(fmt.Sprintf(":%d {\n", port))
		buf.WriteString("\trespond \"No sites configured\" 503\n")
		buf.WriteString("}\n")
		return buf.String(), nil
	}

	// Generate a single server block for all sites
	buf.WriteString(fmt.Sprintf(":%d {\n", port))

	for _, site := range versionSites {
		domains := []string{site.Domain}
		domains = append(domains, site.Aliases...)
		matcherName := sanitizeName(site.ID)

		buf.WriteString(fmt.Sprintf("\n\t# Site: %s (%s)\n", site.Name, site.Domain))

		// Match specific domains
		buf.WriteString(fmt.Sprintf("\t@%s host %s\n", matcherName, strings.Join(domains, " ")))
		buf.WriteString(fmt.Sprintf("\thandle @%s {\n", matcherName))

		// Root directory
		rootPath := filepath.Join(site.RootPath, site.PublicPath)
		buf.WriteString(fmt.Sprintf("\t\troot * %s\n", rootPath))

		// Encoding
		buf.WriteString("\t\tencode zstd br gzip\n")

		// PHP server directive with optional worker mode
		if site.WorkerMode && site.WorkerFile != "" {
			workerNum := site.WorkerNum
			if workerNum <= 0 {
				workerNum = 2
			}
			// Worker file path must be absolute
			workerPath := site.WorkerFile
			if !filepath.IsAbs(workerPath) {
				workerPath = filepath.Join(rootPath, workerPath)
			}
			
			// Safety check: verify worker file exists to prevent breaking all sites
			if _, err := os.Stat(workerPath); err != nil {
				// Worker file doesn't exist - fall back to regular php_server
				buf.WriteString("\t\t# WARNING: Worker file not found, falling back to regular mode\n")
				buf.WriteString("\t\t# Expected: " + workerPath + "\n")
				buf.WriteString("\t\tphp_server\n")
			} else {
				buf.WriteString("\t\tphp_server {\n")
				buf.WriteString(fmt.Sprintf("\t\t\tworker %s %d\n", workerPath, workerNum))

				// Add environment variables
				for key, value := range site.Environment {
					buf.WriteString(fmt.Sprintf("\t\t\tenv %s %s\n", key, value))
				}

				buf.WriteString("\t\t}\n")
			}
		} else {
			buf.WriteString("\t\tphp_server")
			if len(site.Environment) > 0 {
				buf.WriteString(" {\n")
				for key, value := range site.Environment {
					buf.WriteString(fmt.Sprintf("\t\t\tenv %s %s\n", key, value))
				}
				buf.WriteString("\t\t}")
			}
			buf.WriteString("\n")
		}

		buf.WriteString("\t}\n")
	}

	// Default fallback for unmatched hosts
	buf.WriteString("\n\t# Default fallback\n")
	buf.WriteString("\thandle {\n")
	buf.WriteString("\t\trespond \"Site not found\" 404\n")
	buf.WriteString("\t}\n")

	buf.WriteString("}\n")

	return buf.String(), nil
}

// GenerateSiteConfig generates an individual site configuration
func (g *Generator) GenerateSiteConfig(site *models.Site) (string, error) {
	tmplContent := `# Site: {{.Name}}
# Domain: {{.Domain}}
# PHP Version: {{.PHPVersion}}
# Generated by FastCP

{{.Domain}}{{range .Aliases}}, {{.}}{{end}} {
	root * {{.RootPath}}/{{.PublicPath}}
	
	encode zstd br gzip
	
	{{if .WorkerMode}}
	php_server {
		worker {{.WorkerFile}} {{if .WorkerNum}}{{.WorkerNum}}{{else}}2{{end}}
		{{range $key, $value := .Environment}}
		env {{$key}} {{$value}}
		{{end}}
	}
	{{else}}
	php_server{{if .Environment}} {
		{{range $key, $value := .Environment}}
		env {{$key}} {{$value}}
		{{end}}
	}{{end}}
	{{end}}
	
	log {
		output file /var/log/fastcp/sites/{{.ID}}/access.log {
			roll_size 50mb
			roll_keep 3
		}
	}
}
`

	tmpl, err := template.New("site").Parse(tmplContent)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, site); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// WriteMainProxy writes the main proxy Caddyfile
func (g *Generator) WriteMainProxy(content string) error {
	path := filepath.Join(g.outputDir, "Caddyfile.proxy")
	return g.writeFile(path, content)
}

// WritePHPInstance writes a PHP instance Caddyfile
func (g *Generator) WritePHPInstance(version, content string) error {
	path := filepath.Join(g.outputDir, fmt.Sprintf("Caddyfile.php-%s", version))
	return g.writeFile(path, content)
}

// writeFile writes content to a file
func (g *Generator) writeFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// sanitizeName converts a string to a valid Caddy matcher name
func sanitizeName(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "-", "_"), ".", "_")
}

