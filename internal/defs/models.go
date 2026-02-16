package defs

type RuntimeSpec struct {
	Engine  string
	Version string
}

type Service struct {
	Name        string
	Runtime     RuntimeSpec
	GitURL      string
	Branch      string
	Path        string
	RunScript   string
	IngressHost *string
	Variables   []Variable
}

type Variable struct {
	Name  string
	Value string
}

type Environment struct {
	Name      string
	Imports   []string
	Overrides map[string]ServiceOverride
}

type ServiceOverride struct {
	Branch      *string
	IngressHost *string
	Variables   []Variable
}
