package defs

type Service struct {
	Name      string
	GitURL    string
	Branch    string
	Path      string
	RunScript string
	Variables []Variable
}

type Variable struct {
	Name  string
	Value *string
}

type Environment struct {
	Name      string
	Imports   []string
	Overrides map[string]ServiceOverride
}

type ServiceOverride struct {
	Branch    *string
	Variables []Variable
}
