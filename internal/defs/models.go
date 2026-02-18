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
	Port        int
	RunScript   string
	IngressHost *string
	Variables   []Variable
}

type Variable struct {
	Name  string
	Value *string
	Ref   *string
}

type Environment struct {
	Name      string
	Imports   []string
	Overrides map[string]ServiceOverride
}

type ServiceOverride struct {
	Branch      *string
	Port        *int
	IngressHost *string
	Variables   []Variable
}
