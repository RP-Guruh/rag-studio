// Package service contains the Wails-bindable application services that expose
// backend functionality to the Vue frontend.
package service

// GreetService is a minimal scaffold service kept for smoke-testing bindings.
type GreetService struct{}

// NewGreetService returns a GreetService.
func NewGreetService() *GreetService {
	return &GreetService{}
}

// Greet returns a greeting for name.
func (g *GreetService) Greet(name string) string {
	return "Hello " + name + "!"
}
