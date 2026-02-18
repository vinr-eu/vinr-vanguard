package defs

import (
	"vinr.eu/vanguard/internal/defs/v1"
)

func mapServiceV1(svc *v1.Service) *Service {
	branch := "main"
	if svc.Branch != nil {
		branch = *svc.Branch
	}

	path := ""
	if svc.Path != nil {
		path = *svc.Path
	}

	port := 0
	if svc.Port != nil {
		port = *svc.Port
	}

	return &Service{
		Name:        svc.Name,
		Runtime:     RuntimeSpec(svc.Runtime),
		GitURL:      svc.GitURL,
		Branch:      branch,
		Path:        path,
		Port:        port,
		RunScript:   svc.RunScript,
		IngressHost: svc.IngressHost,
		Variables:   mapVariablesV1(svc.Variables),
	}
}

func mapEnvironmentV1(env *v1.Environment) *Environment {
	overrides := make(map[string]ServiceOverride)
	for name, o := range env.Overrides {
		overrides[name] = ServiceOverride{
			Branch:      o.Branch,
			Port:        o.Port,
			IngressHost: o.IngressHost,
			Variables:   mapVariablesV1(o.Variables),
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
			Ref:   v.Ref,
		}
	}
	return out
}
