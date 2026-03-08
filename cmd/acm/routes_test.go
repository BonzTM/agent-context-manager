package main

import (
	"testing"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
)

func TestRouteCatalogCoversCanonicalCommands(t *testing.T) {
	specs := v1.CommandSpecs()
	for _, spec := range specs {
		route, ok := lookupRouteSpec(spec.CLISubcommand)
		if !ok {
			t.Fatalf("missing route for %q", spec.CLISubcommand)
		}
		if route.Usage != spec.CLIUsage {
			t.Fatalf("usage drift for %q: got %q want %q", spec.CLISubcommand, route.Usage, spec.CLIUsage)
		}
		if route.Summary != spec.CLISummary {
			t.Fatalf("summary drift for %q: got %q want %q", spec.CLISubcommand, route.Summary, spec.CLISummary)
		}
	}

	healthRoute, ok := lookupRouteSpec("health")
	if !ok {
		t.Fatal("missing route for health")
	}
	if healthRoute.Summary == "" {
		t.Fatal("expected health route summary")
	}
	for _, name := range []string{"health-check", "health-fix"} {
		route, ok := lookupRouteSpec(name)
		if !ok {
			t.Fatalf("missing hidden route for %q", name)
		}
		if !route.Hidden {
			t.Fatalf("expected %q to stay hidden from main help", name)
		}
	}
}
