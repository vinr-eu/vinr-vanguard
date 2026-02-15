package defs

import "vinr.eu/vanguard/internal/defs/v1"

func mapServiceV1(svc *v1.Service) *Service {
	branch := "main"
	if svc.Branch != nil {
		branch = *svc.Branch
	}

	path := ""
	if svc.Path != nil {
		path = *svc.Path
	}

	return &Service{
		Name:      svc.Name,
		GitURL:    svc.GitURL,
		Branch:    branch,
		Variables: mapVariablesV1(svc.Variables),
		Path:      path,
		RunScript: svc.RunScript,
	}
}

func mapEnvironmentV1(env *v1.Environment) *Environment {
	overrides := make(map[string]ServiceOverride)
	for name, o := range env.Overrides {
		overrides[name] = ServiceOverride{
			Branch:    o.Branch,
			Variables: mapVariablesV1(o.Variables),
		}
	}

	return &Environment{
		Name:      env.Name,
		Imports:   env.Imports,
		Overrides: overrides,
	}
}

func mapVariablesV1(vars []v1.Variable) []Variable {
	out := make([]Variable, len(vars))
	for i, v := range vars {
		out[i] = Variable{
			Name:  v.Name,
			Value: v.Value,
		}
	}
	return out
}
