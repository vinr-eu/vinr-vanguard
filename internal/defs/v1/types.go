package v1

type TypeMeta struct {
	Kind       string `json:"kind"`
	DefVersion string `json:"defVersion"`
}

type ObjectMeta struct {
	Name string `json:"name"`
}

type RuntimeSpec struct {
	Engine  string `json:"engine"`
	Version string `json:"version"`
}

type Service struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	Runtime     RuntimeSpec `json:"runtime"`
	GitURL      string      `json:"gitURL"`
	Branch      *string     `json:"branch,omitempty"`
	Path        *string     `json:"path,omitempty"`
	Port        *int        `json:"port,omitempty"`
	RunScript   string      `json:"runScript"`
	IngressHost *string     `json:"ingressHost,omitempty"`
	Variables   []Variable  `json:"variables,omitempty"`
}

type Variable struct {
	Name  string  `json:"name"`
	Value *string `json:"value,omitempty"`
	Ref   *string `json:"ref,omitempty"`
}

type Environment struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	Imports   []string                   `json:"imports"`
	Overrides map[string]ServiceOverride `json:"overrides,omitempty"`
}

type ServiceOverride struct {
	Branch      *string    `json:"branch,omitempty"`
	Port        *int       `json:"port,omitempty"`
	IngressHost *string    `json:"ingressHost,omitempty"`
	Variables   []Variable `json:"variables,omitempty"`
}
