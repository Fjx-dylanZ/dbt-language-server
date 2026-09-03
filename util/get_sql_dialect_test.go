package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDialect_CustomProfilesDir(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "dbt_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create custom profiles directory (absolute path)
	customProfilesDir := filepath.Join(tempDir, "custom_dbt_config")
	err = os.MkdirAll(customProfilesDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create custom profiles dir: %v", err)
	}

	// Create a test profiles.yml file
	profilesContent := `test_profile:
  target: dev
  outputs:
    dev:
      type: snowflake
`
	profilesFile := filepath.Join(customProfilesDir, "profiles.yml")
	err = os.WriteFile(profilesFile, []byte(profilesContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write profiles.yml: %v", err)
	}

	// Set up environment
	originalProfilesDir := os.Getenv("DBT_PROFILES_DIR")

	defer func() {
		if originalProfilesDir == "" {
			os.Unsetenv("DBT_PROFILES_DIR")
		} else {
			os.Setenv("DBT_PROFILES_DIR", originalProfilesDir)
		}
	}()

	// Set DBT_PROFILES_DIR to absolute path
	os.Setenv("DBT_PROFILES_DIR", customProfilesDir)

	// Test GetDialect with custom profiles directory
	dialect := GetDialect("test_profile", "")

	if dialect != "snowflake" {
		t.Errorf("Expected dialect 'snowflake', got '%s'", dialect)
	}
}
func TestGetDialect_DefaultProfilesDir(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "dbt_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create default .dbt directory
	dbtDir := filepath.Join(tempDir, ".dbt")
	err = os.MkdirAll(dbtDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .dbt dir: %v", err)
	}

	// Create a test profiles.yml file
	profilesContent := `test_profile:
  target: dev
  outputs:
    dev:
      type: postgres
`
	profilesFile := filepath.Join(dbtDir, "profiles.yml")
	err = os.WriteFile(profilesFile, []byte(profilesContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write profiles.yml: %v", err)
	}

	// Set up environment
	originalHome := os.Getenv("HOME")
	originalProfilesDir := os.Getenv("DBT_PROFILES_DIR")

	defer func() {
		os.Setenv("HOME", originalHome)
		if originalProfilesDir == "" {
			os.Unsetenv("DBT_PROFILES_DIR")
		} else {
			os.Setenv("DBT_PROFILES_DIR", originalProfilesDir)
		}
	}()

	os.Setenv("HOME", tempDir)
	os.Unsetenv("DBT_PROFILES_DIR") // Ensure no custom dir is set

	// Test GetDialect with default profiles directory
	dialect := GetDialect("test_profile", "")

	if dialect != "postgres" {
		t.Errorf("Expected dialect 'postgres', got '%s'", dialect)
	}
}

func writeProfiles(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profiles.yml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

// dbt reads profiles.yml from the project directory before ~/.dbt.
func TestGetDialect_ProjectRoot(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DBT_PROFILES_DIR", "")
	writeProfiles(t, filepath.Join(home, ".dbt"), "p:\n  target: dev\n  outputs:\n    dev:\n      type: postgres\n")
	writeProfiles(t, root, "p:\n  target: \"{{ env_var('DBT_TARGET', 'prod') }}\"\n  outputs:\n    prod:\n      type: bigquery\n")

	if got := GetDialect("p", root); got != "bigquery" {
		t.Fatalf("project-root profiles.yml should win over ~/.dbt, got %q", got)
	}

	// $DBT_PROFILES_DIR beats the project root even when a root is given.
	custom := t.TempDir()
	writeProfiles(t, custom, "p:\n  target: dev\n  outputs:\n    dev:\n      type: snowflake\n")
	t.Setenv("DBT_PROFILES_DIR", custom)
	if got := GetDialect("p", root); got != "snowflake" {
		t.Fatalf("DBT_PROFILES_DIR should win over the project root, got %q", got)
	}
}
