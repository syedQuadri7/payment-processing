package generator

import "fmt"

// Generator creates provider-specific webhook payloads.
type Generator interface {
	// Generate creates a webhook payload for the given event type and data.
	Generate(eventType string, data map[string]any) ([]byte, error)

	// Provider returns the provider name (stripe, adyen, paypal).
	Provider() string

	// Endpoint returns the webhook endpoint path for this provider.
	Endpoint() string

	// SupportedEvents returns a list of supported event types.
	SupportedEvents() []string
}

// Registry holds all available generators.
type Registry struct {
	generators map[string]Generator
}

// NewRegistry creates a new generator registry with all providers.
func NewRegistry() *Registry {
	r := &Registry{
		generators: make(map[string]Generator),
	}

	// Register all providers
	r.Register(NewStripeGenerator())
	r.Register(NewAdyenGenerator())
	r.Register(NewPayPalGenerator())

	return r
}

// Register adds a generator to the registry.
func (r *Registry) Register(g Generator) {
	r.generators[g.Provider()] = g
}

// Get returns a generator for the given provider.
func (r *Registry) Get(provider string) (Generator, error) {
	g, ok := r.generators[provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
	return g, nil
}

// Providers returns a list of all registered provider names.
func (r *Registry) Providers() []string {
	providers := make([]string, 0, len(r.generators))
	for name := range r.generators {
		providers = append(providers, name)
	}
	return providers
}
