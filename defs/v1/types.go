package v1

type TypeMeta struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
}

type ObjectMeta struct {
	Name string `json:"name"`
}

type Service struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:",inline"`

	GitHubURL   string     `json:"gitHubUrl"`
	Framework   string     `json:"framework"`
	Port        int        `json:"port"`
	Path        string     `json:"path"`
	IngressType string     `json:"ingressType"`
	IngressHost string     `json:"ingressHost"`
	Variables   []Variable `json:"variables,omitempty"`
}

type Variable struct {
	Name          string `json:"name"`
	Value         string `json:"value,omitempty"`
	Ref           string `json:"ref,omitempty"`
	RefExpression string `json:"refExpression,omitempty"`
}
