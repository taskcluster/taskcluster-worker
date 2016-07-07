package hello

import "github.com/jonasfj/go-import-subtree/example/extpoints"

func init() {
	extpoints.RegisterExtension(&helloCommandProvider{}, "hello")
}

type helloCommandProvider struct{}

func (p *helloCommandProvider) Execute() string {
	return "Hello world!"
}
