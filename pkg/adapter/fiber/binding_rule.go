package fiber

import "github.com/nferreira/adapter/pkg/adapter"

type Method string
type ErrorMapping map[error]int

const (
	Get     Method = "GET"
	Post           = "POST"
	Put            = "PUT"
	Patch          = "PATCH"
	Delete         = "DELETE"
	Options        = "OPTIONS"
)

type BindingRule struct {
	Method Method
	Params []string
	Path   string
	ErrorMapping
}

func NewBindingRule(method Method, path string, params []string, errorMapping ErrorMapping) adapter.BindingRule {
	return &BindingRule{
		Method:       method,
		Params:       params,
		Path:         path,
		ErrorMapping: errorMapping,
	}
}
