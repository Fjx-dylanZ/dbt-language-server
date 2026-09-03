package analysis

import "github.com/j-clemons/dbt-language-server/lsp"

type Column struct {
	Name        string
	DataType    string
	Description string
}

type ModelDetails struct {
	URI         string
	ProjectName string
	Description string
	SchemaURI   string
	SchemaRange lsp.Range
	Columns     []Column
}

type ProjectDetails struct {
	RootPath       string
	DbtProjectYaml DbtProjectYaml
}

func (s *State) getModelDetails() (map[string]ModelDetails, map[string]Source) {
	modelMap := make(map[string]ModelDetails)
	sourceMap := make(map[string]Source)

	packageDetails := getPackageModelDetails(s.DbtContext.ProjectRoot, s.DbtContext.ProjectYaml)

	processList := []ProjectDetails{
		{
			RootPath:       s.DbtContext.ProjectRoot,
			DbtProjectYaml: s.DbtContext.ProjectYaml,
		},
	}
	processList = append(processList, packageDetails...)

	for _, p := range processList {
		modelPathMap := createModelPathMap(p.RootPath, p.DbtProjectYaml)
		modelSchemaDetails, projectSourceMap := parseYamlModels(p.RootPath, p.DbtProjectYaml)

		for k, v := range projectSourceMap {
			sourceMap[k] = v
		}

		for k, v := range modelPathMap {
			modelMapKey := k
			alias, ok := modelSchemaDetails[k].ModelConfig["alias"].Value.(string)
			if ok && alias != "" {
				modelMapKey = alias
			}

			schemaDetails, hasSchema := modelSchemaDetails[k]
			description := ""
			schemaURI := ""
			schemaRange := lsp.Range{}
			var columns []Column

			if hasSchema {
				description = schemaDetails.Description.Value
				schemaURI = schemaDetails.SchemaURI
				schemaRange = lsp.Range{
					Start: schemaDetails.Name.Position,
					End:   schemaDetails.Name.Position,
				}
				if len(schemaDetails.Columns) > 0 {
					columns = make([]Column, len(schemaDetails.Columns))
					for i, c := range schemaDetails.Columns {
						columns[i] = Column{
							Name:        c.Name.Value,
							DataType:    c.DataType.Value,
							Description: c.Description.Value,
						}
					}
				}
			}

			modelMap[modelMapKey] = ModelDetails{
				URI:         v,
				ProjectName: p.DbtProjectYaml.ProjectName.Value,
				Description: description,
				SchemaURI:   schemaURI,
				SchemaRange: schemaRange,
				Columns:     columns,
			}
		}
	}

	seedPathMap := createSeedPathMap(s.DbtContext.ProjectRoot, s.DbtContext.ProjectYaml)
	for k, v := range seedPathMap {
		modelMap[k] = ModelDetails{
			URI:         v,
			ProjectName: s.DbtContext.ProjectYaml.ProjectName.Value,
			Description: "Seed File",
			SchemaURI:   "",
			SchemaRange: lsp.Range{},
		}
	}
	return modelMap, sourceMap
}
