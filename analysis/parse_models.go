package analysis

import (
	"log"
	"path/filepath"
	"regexp"

	"github.com/j-clemons/dbt-language-server/util"
)

func createModelPathMap(projectRoot string, projYaml DbtProjectYaml) map[string]string {
	files, err := util.CreateFileNameMap([]string{".sql", ".py"}, projectRoot, projYaml.ModelPaths.Value)
	if err != nil {
		log.Print(err)
		return nil
	}

	return files
}

func createSeedPathMap(projectRoot string, projYaml DbtProjectYaml) map[string]string {
	files, err := util.CreateFileNameMap([]string{".csv"}, projectRoot, projYaml.SeedPaths.Value)
	if err != nil {
		log.Print(err)
		return nil
	}

	return files
}

var snapshotBlockRegex = regexp.MustCompile(`\{%-?\s*snapshot\s+(\w+)\s*-?%\}`)

// snapshotNames returns the snapshots a project defines under its snapshot
// paths: `{% snapshot name %}` blocks in .sql files and `snapshots:` entries
// in YAML files.
func snapshotNames(projectRoot string, projYaml DbtProjectYaml) []string {
	var names []string
	for _, path := range projYaml.SnapshotPaths.Value {
		dir := filepath.Join(projectRoot, path)

		sqlFiles, _ := util.WalkFilepath(dir, ".sql")
		for _, file := range sqlFiles {
			text, err := util.ReadFileContents(file)
			if err != nil {
				log.Print(err)
				continue
			}
			for _, match := range snapshotBlockRegex.FindAllStringSubmatch(text, -1) {
				names = append(names, match[1])
			}
		}

		for _, file := range propertiesFiles(dir) {
			for _, snapshot := range parsePropertiesYamlFile(file).Snapshots {
				names = append(names, snapshot.Name.Value)
			}
		}
	}
	return names
}
