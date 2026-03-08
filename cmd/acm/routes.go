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
	switch args[0] {
	case "work":
		if len(args) > 1 {
			switch args[1] {
			case "list":
				spec, ok := lookupRouteSpec("work-list")
				return spec, 2, ok
			case "search":
				spec, ok := lookupRouteSpec("work-search")
				return spec, 2, ok
			}
		}
	case "history":
		if len(args) > 1 && args[1] == "search" {
			spec, ok := lookupRouteSpec("history-search")
			return spec, 2, ok
		}
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
	specs := make([]routeSpec, 0, len(v1.CommandSpecs())+3)
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

	specs = append(specs,
		routeSpec{
			Name:    "work-list",
			Usage:   "acm work list [--project <id>] [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded[=true|false]]",
			Summary: "List compact plan summaries for current, deferred, completed, or all work.",
			Group:   routeGroupWorkflow,
			Build: func(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
				return buildHistorySearchEnvelope("work-list", args, now)
			},
		},
		routeSpec{
			Name:    "work-search",
			Usage:   "acm work search [--project <id>] (--query <text>|--query-file <path>) [--scope <current|deferred|completed|all>] [--kind <kind>] [--limit <n>] [--unbounded[=true|false]]",
			Summary: "Search plan and task history without direct database access.",
			Group:   routeGroupWorkflow,
			Build: func(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
				return buildHistorySearchEnvelope("work-search", args, now)
			},
		},
		routeSpec{
			Name:    "health",
			Usage:   "acm health [flags]",
			Summary: "Alias for `acm health-check`.",
			Group:   routeGroupMaintenance,
			Build: func(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
				return buildHealthCheckEnvelope(args, now)
			},
		},
	)

	return specs
}

func canonicalRouteBuilder(name string) (routeBuilder, bool) {
	switch name {
	case "get-context":
		return buildGetContextEnvelope, true
	case "fetch":
		return buildFetchEnvelope, true
	case "propose-memory":
		return buildProposeMemoryEnvelope, true
	case "work":
		return buildWorkEnvelope, true
	case "history-search":
		return func(args []string, now func() time.Time) (v1.CommandEnvelope, error) {
			return buildHistorySearchEnvelope("history-search", args, now)
		}, true
	case "report-completion":
		return buildReportCompletionEnvelope, true
	case "review":
		return buildReviewEnvelope, true
	case "sync":
		return buildSyncEnvelope, true
	case "health-check":
		return buildHealthCheckEnvelope, true
	case "health-fix":
		return buildHealthFixEnvelope, true
	case "coverage":
		return buildCoverageEnvelope, true
	case "eval":
		return buildEvalEnvelope, true
	case "verify":
		return buildVerifyEnvelope, true
	case "bootstrap":
		return buildBootstrapEnvelope, true
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
		if spec.Group != group {
			continue
		}
		out = append(out, helpCommand{
			usage:   spec.Usage,
			summary: spec.Summary,
		})
	}
	return out
}
