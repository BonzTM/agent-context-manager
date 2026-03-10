package main

import (
	"time"

	"github.com/bonztm/agent-context-manager/internal/contracts/v1"
)

type routeGroup string

const (
	routeGroupWorkflow    routeGroup = "workflow"
	routeGroupMaintenance routeGroup = "maintenance"
)

type routeBuilder func([]string, func() time.Time) (v1.CommandEnvelope, error)

type routeSpec struct {
	Name    string
	Usage   string
	Summary string
	Group   routeGroup
	Build   routeBuilder
	Hidden  bool
}

var routeCatalog = buildRouteCatalog()
var routeCatalogByName = buildRouteCatalogByName(routeCatalog)

func lookupRouteSpec(name string) (routeSpec, bool) {
	spec, ok := routeCatalogByName[name]
	return spec, ok
}

func matchConvenienceRoute(args []string) (routeSpec, int, bool) {
	if len(args) == 0 {
		return routeSpec{}, 0, false
	}
	spec, ok := lookupRouteSpec(args[0])
	return spec, 1, ok
}

func workflowHelpCommands() []helpCommand {
	return helpCommandsForGroup(routeGroupWorkflow)
}

func maintenanceHelpCommands() []helpCommand {
	return helpCommandsForGroup(routeGroupMaintenance)
}

func buildRouteCatalog() []routeSpec {
	specs := make([]routeSpec, 0, len(v1.CommandSpecs()))
	for _, spec := range v1.CommandSpecs() {
		builder, ok := canonicalRouteBuilder(spec.CLISubcommand)
		if !ok {
			continue
		}
		specs = append(specs, routeSpec{
			Name:    spec.CLISubcommand,
			Usage:   spec.CLIUsage,
			Summary: spec.CLISummary,
			Group:   routeGroupForCommand(spec.Group),
			Build:   builder,
		})
	}

	return specs
}

func canonicalRouteBuilder(name string) (routeBuilder, bool) {
	switch name {
	case "context":
		return buildContextEnvelope, true
	case "fetch":
		return buildFetchEnvelope, true
	case "memory":
		return buildMemoryEnvelope, true
	case "work":
		return buildWorkEnvelope, true
	case "history":
		return func(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
			return buildHistorySearchEnvelope("history", args, now)
		}, true
	case "done":
		return buildDoneEnvelope, true
	case "review":
		return buildReviewEnvelope, true
	case "sync":
		return buildSyncEnvelope, true
	case "health":
		return buildHealthEnvelope, true
	case "status":
		return buildStatusEnvelope, true
	case "verify":
		return buildVerifyEnvelope, true
	case "init":
		return buildInitEnvelope, true
	default:
		return nil, false
	}
}

func routeGroupForCommand(group v1.CommandGroup) routeGroup {
	switch group {
	case v1.CommandGroupMaintenance:
		return routeGroupMaintenance
	default:
		return routeGroupWorkflow
	}
}

func buildRouteCatalogByName(specs []routeSpec) map[string]routeSpec {
	out := make(map[string]routeSpec, len(specs))
	for _, spec := range specs {
		out[spec.Name] = spec
	}
	return out
}

func helpCommandsForGroup(group routeGroup) []helpCommand {
	out := make([]helpCommand, 0, len(routeCatalog))
	for _, spec := range routeCatalog {
		if spec.Group != group || spec.Hidden {
			continue
		}
		out = append(out, helpCommand{
			usage:   spec.Usage,
			summary: spec.Summary,
		})
	}
	return out
}
