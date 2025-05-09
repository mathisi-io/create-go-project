package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var goVer = getGoVersion()

func main() {
	// Handle project name (from arguments, not flags)
	projectName := ""
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		projectName = os.Args[1]
		newArgs := []string{os.Args[0]}
		newArgs = append(newArgs, os.Args[2:]...)
		os.Args = newArgs
	}

	// Define flags for service and skipPrompt options
	serviceName := flag.String("service", "", "Service to scaffold")
	skipPrompt := flag.Bool("yes", false, "Skip prompts and use defaults")

	// Parse flags
	flag.Parse()

	// If skipPrompt is true, use default values for project and service
	if *skipPrompt {
		if projectName == "" {
			projectName = "microservice"
		}
		if *serviceName == "" {
			*serviceName = "example"
		}
		fmt.Println("⚙️  Using defaults: project =", projectName, ", service =", *serviceName)
	} else {
		// Otherwise, interactively ask for names
		reader := bufio.NewReader(os.Stdin)

		// Prompt for project name if not supplied
		if projectName == "" {
			fmt.Print("📝 Enter project name: ")
			input, _ := reader.ReadString('\n')
			projectName = strings.TrimSpace(input)
		}

		// Prompt for service name if not supplied
		if *serviceName == "" {
			fmt.Print("🛠️  Enter service name (e.g. user, billing): ")
			input, _ := reader.ReadString('\n')
			*serviceName = strings.TrimSpace(input)
		}
	}

	// Check for empty project or service names
	if projectName == "" || *serviceName == "" {
		log.Fatal("❌ Project and service names are required.")
	}

	if _, err := os.Stat(projectName); err == nil {
		log.Printf("Project %s already exists, skipping project creation.", projectName)
		createService(projectName, *serviceName)
	} else {
		// Proceed with the project creation
		createProject(projectName, *serviceName)
	}

	formatCode(projectName)
}

func createProject(project, service string) {
	// List of directories to create
	baseDirs := []string{
		"shared/config",
		"deploy",
	}

	// Create directories
	for _, dir := range baseDirs {
		fullPath := filepath.Join(project, dir)

		if err := os.MkdirAll(fullPath, 0755); err != nil {
			log.Fatalf("Error creating directory %s: %v", fullPath, err)
		}
	}

	// Add initial files in the project
	writeFile(project, "go.work", fmt.Sprintf(`go %s
	`, goVer))

	writeFile(project, "Makefile", fmt.Sprintf(`build:
	go build -o bin/%s-cli ./services/%s/cmd/cli/main.go
	go build -o bin/%s-api ./services/%s/cmd/api/main.go

`, service, service, service, service))

	writeFile(project, "README.md", fmt.Sprintf(`# %s

Generated with create-go-app.

Includes:
- shared/config
- services/%s (API, CLI)
`, project, service))

	writeFile(filepath.Join(project, "shared"), "go.mod", fmt.Sprintf(`module %s/shared

go %s`, project, goVer))

	const configTpl = `package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Database struct {
		Driver          string        §yaml:"driver"§
		Host            string        §yaml:"host"§
		Port            int           §yaml:"port"§
		User            string        §yaml:"user"§
		Password        string        §yaml:"password"§
		Dbname          string        §yaml:"dbname"§
		Sslmode         string        §yaml:"sslmode"§
		MaxOpenConns    int           §yaml:"maxOpenConns"§
		MaxIdleConns    int           §yaml:"maxIdleConns"§
		ConnMaxLifetime time.Duration §yaml:"connMaxLifetime"§
	} §yaml:"database"§
	Context struct {
		Timeout time.Duration §yaml:"timeout"§
	} §yaml:"context"§
	Server struct {
		Port int §yaml:"port"§
	} §yaml:"server"§
}

func LoadConfig(service string) (*Config, error) {
	data, err := os.ReadFile("./services/" + service + "/config/config.yaml")
	if err != nil {
		return nil, err
	}
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
`
	writeFile(filepath.Join(project, "shared/config"), "config.go", renderTemplate(configTpl, '§'))

	writeFile(project, ".gitignore", `.DS_Store
bin/
*.log
*.test
*.out
*.swp
vendor/
*.exe
*.exe~
*.dll
*.so
*.dylib
coverage.out
.idea/
.env
.env.*
`)
	// Initialize Git repo
	if err := runCmd(project, "git", "init"); err != nil {
		log.Printf("⚠️ Failed to initialize Git repo: %v", err)
	} else {
		fmt.Println("📦 Git repository initialized.")
	}

	// Run go mod tidy in shared folder
	sharedPath := filepath.Join(project, "shared")
	if err := runCmd(sharedPath, "go", "mod", "tidy"); err != nil {
		log.Printf("⚠️ Failed to run in shared 'go mod tidy': %v", err)
	} else {
		fmt.Println("🧹 go mod tidy run inside shared")
	}

	// Create initial service files
	createService(project, service)

	// Final message
	fmt.Printf("\n✅ Project '%s' created with service '%s'\n", project, service)
	fmt.Printf("📁 cd %s\n", project)
	fmt.Println("🚀 You're ready to start building!")
}

// Replace placeholder with backtick
func renderTemplate(template string, placeholder rune) string {
	return strings.ReplaceAll(template, string(placeholder), "`")
}

func writeFile(base, name, content string) {
	path := filepath.Join(base, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		log.Fatalf("Error writing file %s: %v", path, err)
	}
}

func runCmd(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getGoVersion() string {
	// Get and print the Go version
	goVersionCmd := exec.Command("go", "version")
	output, err := goVersionCmd.Output()
	goVer := ""

	if err != nil {
		log.Fatalf("❌ Failed to get Go version: %v", err)
	} else {
		goVer = strings.TrimSpace(string(output))
		parts := strings.Fields(goVer)
		if len(parts) >= 3 {
			versionParts := strings.Split(parts[2][2:], ".")
			if len(versionParts) > 1 {
				goVer = versionParts[0] + "." + versionParts[1]
			}
		}
		fmt.Printf("✅ Go version: %s\n", goVer)
	}

	return goVer
}

func createService(project, service string) {
	// List of directories to create
	baseDirs := []string{
		fmt.Sprintf("services/%s/api", service),
		fmt.Sprintf("services/%s/cli", service),
		fmt.Sprintf("services/%s/cmd/api", service),
		fmt.Sprintf("services/%s/cmd/cli", service),
		fmt.Sprintf("services/%s/config", service),
		fmt.Sprintf("services/%s/db", service),
		fmt.Sprintf("services/%s/internal", service),
	}
	// Create directories
	for _, dir := range baseDirs {
		fullPath := filepath.Join(project, dir)

		if err := os.MkdirAll(fullPath, 0755); err != nil {
			log.Fatalf("Error creating directory %s: %v", fullPath, err)
		}
	}

	writeFile(filepath.Join(project, "services", service), "go.mod", fmt.Sprintf(`module %s/%s

go %s
`, project, service, goVer))

	// Create service files
	writeFile(filepath.Join(project, "services", service, "cmd/api"), "main.go", fmt.Sprintf(`package main

import (
	"fmt"
	"log"
	"net/http"
	"%s/shared/config"
	"%s/%s/api"
)

func main() {
	port := 8081
	config, err := config.LoadConfig("%s")
	if err == nil {
		port = config.Server.Port
	}

	http.HandleFunc("/hello", api.HelloHandler)
	log.Printf("🔌 API server running at :%s\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
`, project, project, service, service, "%d", "%d"))

	writeFile(filepath.Join(project, "services", service, "cmd/cli"), "main.go", fmt.Sprintf(`package main

import (
	"%s/%s/cli"
)

func main() {
	cli.Execute()
}
`, project, service))

	writeFile(filepath.Join(project, "services", service, "api"), "handlers.go", fmt.Sprintf(`package api

import (
	"fmt"
	"net/http"
)

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "👋 Hello from the %s API!")
}
`, service))

	writeFile(filepath.Join(project, "services", service, "cli"), "root.go", fmt.Sprintf(`package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cli",
	Short: "CLI entry point",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("👋 Hello from the %s CLI!")
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
`, service))

	writeFile(filepath.Join(project, "services", service, "config"), "config.yaml", fmt.Sprintf(`server:
  port: %d
`, 8080+rand.Intn(10)))

	writeFile(filepath.Join(project, "services", service, "db"), "schema.sql", `-- SQL schema placeholder
CREATE TABLE example (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);
`)

	writeFile(filepath.Join(project, "services", service, "internal"), "service.go", `package internal

func Greet(name string) string {
	return "Hello, " + name + "!"
}
`)

	// Run go mod tidy in service folder
	servicePath := filepath.Join(project, "services", service)
	if err := runCmd(servicePath, "go", "mod", "edit", "-replace", project+"/shared=../../shared"); err != nil {
		log.Println("⚠️ Failed to run 'go mod edit'")
	}

	if err := runCmd(servicePath, "go", "mod", "tidy"); err != nil {
		log.Printf("⚠️ Failed to run 'go mod tidy': %v", err)
	} else {
		fmt.Println("🧹 go mod tidy run inside", service)
	}

	// aupdate go.work with the service name
	if err := runCmd(project, "go", "work", "use", fmt.Sprintf("./services/%s", service)); err != nil {
		log.Printf("⚠️ Failed to run go work use ./services/%s", service)
	}

	makefilePath := filepath.Join(project, "Makefile")
	if !fileContainsText(makefilePath, fmt.Sprintf("run-%s-api", service)) {
		makefileContent := fmt.Sprintf(`run-%s-api:
	go run services/%s/cmd/api/main.go

run-%s-cli:
	go run services/%s/cmd/cli/main.go

`, service, service, service, service)
		appendContent(makefilePath, makefileContent)
	}

	// Update README.md
	readmePath := filepath.Join(project, "README.md")
	readmeContent := fmt.Sprintf(`- services/%s (API, CLI)`, service)
	if !fileContainsText(readmePath, readmeContent) {
		appendContent(readmePath, readmeContent)
	}
}

func appendContent(filePath, content string) {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("Error opening file %s: %v", filePath, err)
	}
	defer file.Close()

	// Read the existing content
	existingContent, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Error reading file %s: %v", filePath, err)
	}

	// Append the new content
	newContent := string(existingContent) + content
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		log.Fatalf("Error writing to file %s: %v", filePath, err)
	}
}

func fileContainsText(filepath, text string) bool {
	content, err := os.ReadFile(filepath)
	if err != nil {
		log.Fatalf("Error reading %s file: %v", filepath, err)
	}
	if strings.Contains(string(content), text) {
		return true
	}
	return false
}

func formatCode(path string) error {
	cmd := exec.Command("go", "fmt", "./...")
	cmd.Dir = path
	return cmd.Run()
}
