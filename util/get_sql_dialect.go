package util

import (
	"log"
	"os"
	"path/filepath"

	"github.com/j-clemons/dbt-language-server/docs"
	"gopkg.in/yaml.v3"
)

type Profiles struct {
	DefaultTarget string            `yaml:"target"`
	Outputs       map[string]Target `yaml:"outputs"`
}

type Target struct {
	Type string `yaml:"type"`
}

// GetDialect returns the adapter type of profileName's default target. The
// first profiles.yml found wins, in dbt's own lookup order: $DBT_PROFILES_DIR,
// the project root, then ~/.dbt (plus <root>/.dbt, kept for existing setups).
func GetDialect(profileName string, projectRoot string) docs.Dialect {
	for _, filePath := range profilesPaths(projectRoot) {
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		return dialectFromProfiles(data, profileName)
	}
	log.Println("profiles.yml not found; SQL dialect unknown")
	return ""
}

func profilesPaths(projectRoot string) []string {
	paths := make([]string, 0, 4)
	if dir := os.Getenv("DBT_PROFILES_DIR"); dir != "" {
		paths = append(paths, filepath.Join(dir, "profiles.yml"))
	}
	if projectRoot != "" {
		paths = append(paths,
			filepath.Join(projectRoot, "profiles.yml"),
			filepath.Join(projectRoot, ".dbt", "profiles.yml"),
		)
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(homeDir, ".dbt", "profiles.yml"))
	}
	return paths
}

func dialectFromProfiles(data []byte, profileName string) docs.Dialect {
	var profilesYaml map[string]Profiles
	if err := yaml.Unmarshal([]byte(ResolveEnvVars(string(data))), &profilesYaml); err != nil {
		log.Println("Error parsing profiles.yml:", err)
		return ""
	}

	entry, exists := profilesYaml[profileName]
	if !exists {
		log.Println("Profile not found in profiles.yml:", profileName)
		return ""
	}

	if output, ok := entry.Outputs[entry.DefaultTarget]; ok {
		return docs.Dialect(output.Type)
	}
	log.Println("Default target not found in outputs:", entry.DefaultTarget)
	return ""
}
