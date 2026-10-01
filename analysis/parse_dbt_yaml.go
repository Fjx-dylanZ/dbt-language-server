package analysis

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/j-clemons/dbt-language-server/lsp"
	"github.com/j-clemons/dbt-language-server/util"
	"gopkg.in/yaml.v3"
)

type AnnotatedField[T any] struct {
	Value    T
	Position lsp.Position
}

func (a *AnnotatedField[T]) UnmarshalYAML(value *yaml.Node) error {
	a.Position = lsp.Position{
		Line:      value.Line - 1,
		Character: value.Column - 1,
	}
	return value.Decode(&a.Value)
}

type AnnotatedMap map[string]AnnotatedField[any]

func (a *AnnotatedMap) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node but got %v", value.Kind)
	}

	*a = make(AnnotatedMap)
	for i := 0; i < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		valueNode := value.Content[i+1]

		var key string
		if err := keyNode.Decode(&key); err != nil {
			return fmt.Errorf("failed to decode key: %w", err)
		}

		if valueNode.Kind == yaml.MappingNode {
			var nested AnnotatedMap
			if err := valueNode.Decode(&nested); err != nil {
				return fmt.Errorf("failed to decode nested map for key '%s': %w", key, err)
			}

			(*a)[key] = AnnotatedField[any]{
				Value: nested,
				Position: lsp.Position{
					Line:      valueNode.Line - 1,
					Character: valueNode.Column - 1,
				},
			}
		} else {
			var value any
			if err := valueNode.Decode(&value); err != nil {
				return fmt.Errorf("failed to decode value for key '%s': %w", key, err)
			}

			(*a)[key] = AnnotatedField[any]{
				Value: value,
				Position: lsp.Position{
					Line:      valueNode.Line - 1,
					Character: valueNode.Column - 1,
				},
			}
		}
	}
	return nil
}

type DbtProjectYaml struct {
	ProjectName         AnnotatedField[string]   `yaml:"name"`
	Profile             AnnotatedField[string]   `yaml:"profile"`
	ModelPaths          AnnotatedField[[]string] `yaml:"model-paths"`
	SeedPaths           AnnotatedField[[]string] `yaml:"seed-paths"`
	SnapshotPaths       AnnotatedField[[]string] `yaml:"snapshot-paths"`
	MacroPaths          AnnotatedField[[]string] `yaml:"macro-paths"`
	PackagesInstallPath AnnotatedField[string]   `yaml:"packages-install-path"`
	DocsPaths           AnnotatedField[[]string] `yaml:"docs-paths"`
	Vars                AnnotatedMap             `yaml:"vars"`
}

// parseDbtProjectYaml logs to stderr: stdout carries the LSP stream.
func parseDbtProjectYaml(projectRoot string) DbtProjectYaml {
	fileStr, err := util.ReadFileContents(filepath.Join(projectRoot, "dbt_project.yml"))
	if err != nil {
		log.Printf("Error opening file: %v", err)
		return DbtProjectYaml{}

	}

	fileStr = util.ResolveEnvVars(fileStr)

	var projYaml DbtProjectYaml
	if err := yaml.Unmarshal([]byte(fileStr), &projYaml); err != nil {
		log.Printf("Failed to unmarshal YAML: %v", err)
		return DbtProjectYaml{}
	}

	availableDirs := map[string]int{}
	entries, err := os.ReadDir(projectRoot)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				availableDirs[entry.Name()] = 1
			}
		}
	}

	if projYaml.ModelPaths.Value == nil || len(projYaml.ModelPaths.Value) == 0 {
		if availableDirs["models"] == 1 {
			projYaml.ModelPaths.Value = []string{"models"}
		}
	}
	if projYaml.SeedPaths.Value == nil || len(projYaml.SeedPaths.Value) == 0 {
		if availableDirs["seeds"] == 1 {
			projYaml.SeedPaths.Value = []string{"seeds"}
		}
	}
	if projYaml.SnapshotPaths.Value == nil || len(projYaml.SnapshotPaths.Value) == 0 {
		if availableDirs["snapshots"] == 1 {
			projYaml.SnapshotPaths.Value = []string{"snapshots"}
		}
	}
	if projYaml.MacroPaths.Value == nil || len(projYaml.MacroPaths.Value) == 0 {
		if availableDirs["macros"] == 1 {
			projYaml.MacroPaths.Value = []string{"macros"}
		}
	}
	if projYaml.PackagesInstallPath.Value == "" {
		if availableDirs["dbt_packages"] == 1 {
			projYaml.PackagesInstallPath.Value = "dbt_packages"
		}
	}
	if projYaml.DocsPaths.Value == nil || len(projYaml.DocsPaths.Value) == 0 {
		if availableDirs["docs"] == 1 {
			projYaml.DocsPaths.Value = []string{"docs"}
		}
		projYaml.DocsPaths.Value = append(projYaml.DocsPaths.Value, projYaml.ModelPaths.Value...)
		projYaml.DocsPaths.Value = append(projYaml.DocsPaths.Value, projYaml.MacroPaths.Value...)
	}
	return projYaml
}

type PropertiesYaml struct {
	Models    []ModelProperties    `yaml:"models"`
	Sources   []SourceProperties   `yaml:"sources"`
	Snapshots []SnapshotProperties `yaml:"snapshots"`
}

type ModelProperties struct {
	Name        AnnotatedField[string] `yaml:"name"`
	Description AnnotatedField[string] `yaml:"description"`
	ModelConfig AnnotatedMap           `yaml:"config"`
	Columns     []ColumnProperties     `yaml:"columns"`
	// Versions lists a versioned model's versions; ref() resolves its name
	// although no file carries it.
	Versions  []any `yaml:"versions"`
	SchemaURI string
}

type SnapshotProperties struct {
	Name AnnotatedField[string] `yaml:"name"`
}

type SourceProperties struct {
	Name        AnnotatedField[string]  `yaml:"name"`
	Database    AnnotatedField[string]  `yaml:"database"`
	Schema      AnnotatedField[string]  `yaml:"schema"`
	Description AnnotatedField[string]  `yaml:"description"`
	Tables      []SourceTableProperties `yaml:"tables"`
}

type SourceTableProperties struct {
	Name        AnnotatedField[string] `yaml:"name"`
	Description AnnotatedField[string] `yaml:"description"`
}

type ColumnProperties struct {
	Name        AnnotatedField[string] `yaml:"name"`
	DataType    AnnotatedField[string] `yaml:"data_type"`
	Description AnnotatedField[string] `yaml:"description"`
}

// parsePropertiesYamlFile keeps what decodes when some values have the wrong
// type, so one odd field does not hide a file's models and sources.
func parsePropertiesYamlFile(path string) PropertiesYaml {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("Error opening file: %v", err)
		return PropertiesYaml{}

	}
	defer file.Close()

	var config PropertiesYaml
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		var typeErr *yaml.TypeError
		if errors.As(err, &typeErr) {
			log.Printf("Partially decoded %s: %v", path, err)
			return config
		}
		log.Printf("Error decoding YAML %s: %v", path, err)
		return PropertiesYaml{}
	}
	return config
}

// propertiesFiles lists the YAML files under dir: dbt reads both .yml and .yaml.
func propertiesFiles(dir string) []string {
	var files []string
	for _, ext := range []string{".yml", ".yaml"} {
		found, _ := util.WalkFilepath(dir, ext)
		files = append(files, found...)
	}
	return files
}

type Source struct {
	Name        string
	Description string
	URI         string
	Range       lsp.Range
	Tables      map[string]SourceTable
}

type SourceTable struct {
	Name        string
	Description string
	Table       string
	URI         string
	Range       lsp.Range
}

func parseYamlModels(projectRoot string, projYaml DbtProjectYaml) (map[string]ModelProperties, map[string]Source) {
	modelMap := make(map[string]ModelProperties)
	sourceMap := make(map[string]Source)

	docsFiles := getDocsFiles(projectRoot, projYaml)
	docsMap := processDocsFiles(docsFiles)

	for _, path := range projYaml.ModelPaths.Value {
		_, err := os.ReadDir(projectRoot + "/" + path)
		if err != nil {
			continue
		}
		for _, file := range propertiesFiles(projectRoot + "/" + path + "/") {
			dbtYml := parsePropertiesYamlFile(file)
			for _, model := range dbtYml.Models {
				columns := make([]ColumnProperties, len(model.Columns))
				for i, column := range model.Columns {
					column.Description.Value = replaceDescriptionDocsBlocks(column.Description.Value, docsMap)
					columns[i] = column
				}
				modelMap[model.Name.Value] = ModelProperties{
					Name:        model.Name,
					Description: AnnotatedField[string]{Value: replaceDescriptionDocsBlocks(model.Description.Value, docsMap)},
					ModelConfig: AnnotatedMap{
						"alias": AnnotatedField[any]{
							Value: model.ModelConfig["alias"].Value,
							Position: lsp.Position{
								Line:      model.ModelConfig["alias"].Position.Line,
								Character: model.ModelConfig["alias"].Position.Character,
							},
						},
					},
					Columns:   columns,
					Versions:  model.Versions,
					SchemaURI: file,
				}
			}
			for _, source := range dbtYml.Sources {
				tables := make(map[string]SourceTable, len(source.Tables))
				for _, table := range source.Tables {
					tables[table.Name.Value] = SourceTable{
						Name:        table.Name.Value,
						Description: replaceDescriptionDocsBlocks(table.Description.Value, docsMap),
						Table:       source.Name.Value,
						URI:         file,
						Range: lsp.Range{
							Start: table.Name.Position,
							End:   table.Name.Position,
						},
					}
				}
				addSource(sourceMap, Source{
					Name:        source.Name.Value,
					Description: replaceDescriptionDocsBlocks(source.Description.Value, docsMap),
					URI:         file,
					Range: lsp.Range{
						Start: source.Name.Position,
						End:   source.Name.Position,
					},
					Tables: tables,
				})
			}
		}
	}

	return modelMap, sourceMap
}

// addSource records a source definition. dbt lets several files and packages
// declare tables under one source name, so their tables accumulate; the first
// definition keeps the source's own description and location.
func addSource(sourceMap map[string]Source, source Source) {
	existing, ok := sourceMap[source.Name]
	if !ok {
		sourceMap[source.Name] = source
		return
	}
	for name, table := range source.Tables {
		existing.Tables[name] = table
	}
}

func replaceDescriptionDocsBlocks(description string, docsMap map[string]Docs) string {
	docBlocksRegex := regexp.MustCompile(`{{\s*doc\(('|")([-zA-z]+)('|")\)\s*}}`)

	matches := docBlocksRegex.FindAllStringSubmatchIndex(description, -1)
	if len(matches) == 0 {
		return description
	}

	newDescription := description
	for i := 0; i < len(matches); i++ {
		docName := description[matches[i][4]:matches[i][5]]

		if _, ok := docsMap[docName]; ok {
			newDescription = newDescription[:matches[i][0]] + docsMap[docName].Content + newDescription[matches[i][1]:]
		}
	}

	return newDescription
}
