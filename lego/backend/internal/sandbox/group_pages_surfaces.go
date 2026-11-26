/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sandbox

import (
	"context"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// GraphQL for a sandbox group's pages in the dashboard (w1/m658), named as
// Render's dashboard names them: sandboxesForGroup, sandboxEvents,
// runningSandboxCount and sandboxBuildSources, plus the group's run summary
// and a snapshot page. They are the dashboard's reads, so they have no REST
// or MCP twin; every sandbox, snapshot and image they answer is the one
// REST and MCP answer for the same id.

// nanoTime is an instant as an event's timestamp and cursor carry it, to the
// nanosecond, so a page boundary never falls inside a rounded second.
func nanoTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// timeArg reads an optional RFC 3339 time argument (core.ParseTime); a
// malformed one is refused by name.
func timeArg(args map[string]any, key string) (*time.Time, error) {
	t, err := core.ParseTime(key, gqlutil.Str(args, key))
	if err != nil || t.IsZero() {
		return nil, err
	}
	return &t, nil
}

// inputArg is a resolver's `input` object and the context scoped to its
// ownerId, as gqlutil.OwnerScoped scopes a top-level one.
func inputArg(p graphql.ResolveParams) (map[string]any, context.Context) {
	input, _ := p.Args["input"].(map[string]any)
	return input, core.WithWorkspace(p.Context, gqlutil.Str(input, "ownerId"))
}

// sandboxPlanSpecGQLType is Render's plan object on a sandbox and a snapshot:
// the plan's name and size.
var sandboxPlanSpecGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxPlanSpec",
	Fields: graphql.Fields{
		"name":     gqlutil.ReqStrField(func(p Plan) any { return string(p) }),
		"cpus":     gqlutil.FloatField(func(p Plan) any { return planCPUs(p) }),
		"memoryGb": gqlutil.FloatField(func(p Plan) any { return planMemoryGB(p) }),
	},
})

// planSpec is a plan for planSpec fields: null for no plan.
func planSpec(p Plan) any {
	if p == "" {
		return nil
	}
	return p
}

func planCPUs(p Plan) any {
	size, ok := planResources[p]
	if !ok {
		return nil
	}
	cpus := quantity(size.CPU)
	return cpus.AsApproximateFloat64()
}

func planMemoryGB(p Plan) any {
	size, ok := planResources[p]
	if !ok {
		return nil
	}
	memory := quantity(size.Memory)
	return memory.AsApproximateFloat64() / (1 << 30)
}

// sandboxSourceBuildGQLType is Render's sourceBuild: the imported image a
// sandbox started from. Its name is null once the image is deleted.
var sandboxSourceBuildGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxSourceBuild",
	Fields: graphql.Fields{
		"id":   gqlutil.ReqStrField(func(b Sandbox) any { return b.sourceBuildID() }),
		"name": gqlutil.OptionalStrField(func(b Sandbox) any { return b.details.sourceBuildName }),
	},
})

// sandboxEmailSourceGQLType is Render's SandboxEmailSource: the person who
// started the sandbox. bex has no template source, Render's other kind.
var sandboxEmailSourceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxEmailSource",
	Fields: graphql.Fields{
		"email": gqlutil.ReqStrField(func(b Sandbox) any { return b.details.createdBy }),
	},
})

var sandboxSourceGQLType = graphql.NewUnion(graphql.UnionConfig{
	Name:        "SandboxSource",
	Types:       []*graphql.Object{sandboxEmailSourceGQLType},
	ResolveType: func(graphql.ResolveTypeParams) *graphql.Object { return sandboxEmailSourceGQLType },
})

// sandboxDetailFields are the Sandbox fields Render's drawer reads beyond
// the REST shape (sandboxListFields).
func sandboxDetailFields() graphql.Fields {
	return graphql.Fields{
		"startedAt": gqlutil.TimeField(func(b Sandbox) any { return b.details.startedAt }),
		"endedAt":   gqlutil.TimeField(func(b Sandbox) any { return b.TerminatedAt }),
		// Render's errorReason; bex does not record why a sandbox errored.
		"errorReason": gqlutil.StrField(func(b Sandbox) any {
			if b.Status != StatusErrored {
				return nil
			}
			return "UNKNOWN"
		}),
		"timeoutAt":        gqlutil.TimeField(func(b Sandbox) any { return b.details.timeoutAt }),
		"sourceSnapshotId": gqlutil.OptionalStrField(func(b Sandbox) any { return b.sourceSnapshotID() }),
		"sourceBuildId":    gqlutil.OptionalStrField(func(b Sandbox) any { return b.sourceBuildID() }),
		"sourceBuild": gqlutil.Typed(sandboxSourceBuildGQLType, func(b Sandbox) any {
			if b.sourceBuildID() == "" {
				return nil
			}
			return b
		}),
		"source": gqlutil.Typed(sandboxSourceGQLType, func(b Sandbox) any {
			if b.details.createdBy == "" {
				return nil
			}
			return b
		}),
		// Which surface started the sandbox (Render's created event trigger:
		// DASHBOARD, CLI, API, SDK); null when bex did not record it.
		"trigger":  gqlutil.OptionalStrField(func(b Sandbox) any { return b.details.trigger }),
		"planSpec": gqlutil.Typed(sandboxPlanSpecGQLType, func(b Sandbox) any { return planSpec(b.Plan) }),
	}
}

var sandboxesForGroupInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "SandboxesForGroupFilterInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"sandboxGroupId":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"ownerId":          &graphql.InputObjectFieldConfig{Type: graphql.String},
		"statuses":         &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
		"search":           &graphql.InputObjectFieldConfig{Type: graphql.String},
		"createdAfter":     &graphql.InputObjectFieldConfig{Type: graphql.String},
		"createdBefore":    &graphql.InputObjectFieldConfig{Type: graphql.String},
		"sourceSnapshotId": &graphql.InputObjectFieldConfig{Type: graphql.String},
		"sourceBuildId":    &graphql.InputObjectFieldConfig{Type: graphql.String},
		// Every version of an image, for its drawer's Sandboxes tab.
		"sourceBuildIds": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
		"cursor":         &graphql.InputObjectFieldConfig{Type: graphql.String},
		"limit":          &graphql.InputObjectFieldConfig{Type: graphql.Int},
	},
})

var sandboxesForGroupPageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxesForGroupPage",
	Fields: graphql.Fields{
		"cursor":        gqlutil.OptionalStrField(func(p GroupSandboxPage) any { return p.Cursor }),
		"hasMore":       gqlutil.ReqBoolField(func(p GroupSandboxPage) any { return p.HasMore }),
		"sandboxes":     gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(sandboxGQLType))), func(p GroupSandboxPage) any { return p.Sandboxes }),
		"sortField":     gqlutil.ReqStrField(func(GroupSandboxPage) any { return groupSandboxSortField }),
		"sortDirection": gqlutil.ReqStrField(func(GroupSandboxPage) any { return groupSandboxSortDirection }),
	},
})

var sandboxRunBucketGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxRunBucket",
	Fields: graphql.Fields{
		"time":      gqlutil.TimeField(func(b RunBucket) any { return b.Time }),
		"succeeded": gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(b RunBucket) any { return b.Succeeded }),
		"errored":   gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(b RunBucket) any { return b.Errored }),
	},
})

var sandboxGroupRunsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxGroupRuns",
	Fields: graphql.Fields{
		"total":     gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(r GroupRunSummary) any { return r.Total }),
		"errored":   gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(r GroupRunSummary) any { return r.Errored }),
		"succeeded": gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(r GroupRunSummary) any { return r.Succeeded }),
		"buckets":   gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(sandboxRunBucketGQLType))), func(r GroupRunSummary) any { return r.Buckets }),
	},
})

// sandboxEventFields are every SandboxEvent's fields.
func sandboxEventFields() graphql.Fields {
	return graphql.Fields{
		"id":        gqlutil.ReqStrField(func(e Event) any { return e.ID }),
		"timestamp": gqlutil.Typed(graphql.NewNonNull(graphql.String), func(e Event) any { return nanoTime(e.Timestamp) }),
		"outcome":   gqlutil.ReqStrField(func(e Event) any { return e.Outcome }),
		"label":     gqlutil.ReqStrField(func(e Event) any { return e.Label }),
	}
}

var sandboxEventInterface = graphql.NewInterface(graphql.InterfaceConfig{
	Name:   "SandboxEvent",
	Fields: sandboxEventFields(),
	ResolveType: func(p graphql.ResolveTypeParams) *graphql.Object {
		e, _ := p.Value.(Event)
		return sandboxEventTypes[e.Kind]
	},
})

// sandboxEventTypes are SandboxEvent's implementations by event kind, filled
// in init: the interface's resolver and its implementations refer to each
// other.
var sandboxEventTypes = map[string]*graphql.Object{}

func init() {
	sandboxEventTypes[EventKindLifecycle] = sandboxLifecycleEventGQLType
	sandboxEventTypes[EventKindExecution] = sandboxExecutionEventGQLType
	sandboxEventTypes[EventKindSnapshot] = sandboxSnapshotEventGQLType
}

// GraphQLTypes names SandboxEvent's implementations for the schema, which no
// field returns by their own type.
func (s *Service) GraphQLTypes() []graphql.Type {
	return []graphql.Type{sandboxLifecycleEventGQLType, sandboxExecutionEventGQLType, sandboxSnapshotEventGQLType}
}

func eventObject(name string, extra graphql.Fields) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{Name: name, Interfaces: []*graphql.Interface{sandboxEventInterface}, Fields: withFields(extra, sandboxEventFields())})
}

var sandboxLifecycleEventGQLType = eventObject("SandboxLifecycleEvent", graphql.Fields{
	"lifecycleType": gqlutil.ReqStrField(func(e Event) any { return e.LifecycleType }),
	"trigger":       gqlutil.OptionalStrField(func(e Event) any { return e.Trigger }),
	// The person who started the sandbox, on its created event.
	"userEmail": gqlutil.OptionalStrField(func(e Event) any { return e.UserEmail }),
})

var sandboxExecutionEventGQLType = eventObject("SandboxExecutionEvent", graphql.Fields{
	"summary":       gqlutil.ReqStrField(func(e Event) any { return e.Label }),
	"executionId":   gqlutil.ReqStrField(func(e Event) any { return e.Execution.ID }),
	"executionType": gqlutil.ReqStrField(func(e Event) any { return e.Execution.Type }),
	"operation":     gqlutil.ReqStrField(func(e Event) any { return e.Execution.Operation }),
	"fullCommand":   gqlutil.OptionalStrField(func(e Event) any { return e.Execution.Command }),
	"exitCode":      gqlutil.IntField(func(e Event) any { return e.Execution.ExitCode }),
	"startedAt":     gqlutil.TimeField(func(e Event) any { return e.Execution.StartedAt }),
	"completedAt":   gqlutil.TimeField(func(e Event) any { return e.Execution.StoppedAt }),
})

var sandboxSnapshotEventGQLType = eventObject("SandboxSnapshotEvent", graphql.Fields{
	"snapshot": gqlutil.Typed(graphql.NewNonNull(sandboxSnapshotGQLType), func(e Event) any { return *e.Snapshot }),
})

var sandboxEventPageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxEventPage",
	Fields: graphql.Fields{
		"hasMore": gqlutil.ReqBoolField(func(p EventPage) any { return p.HasMore }),
		"events":  gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(sandboxEventInterface))), func(p EventPage) any { return p.Events }),
	},
})

var sandboxSnapshotsForGroupInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "SandboxSnapshotsFilterInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"sandboxGroupId":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"ownerId":         &graphql.InputObjectFieldConfig{Type: graphql.String},
		"statuses":        &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
		"search":          &graphql.InputObjectFieldConfig{Type: graphql.String},
		"sourceSandboxId": &graphql.InputObjectFieldConfig{Type: graphql.String},
		"createdAfter":    &graphql.InputObjectFieldConfig{Type: graphql.String},
		"createdBefore":   &graphql.InputObjectFieldConfig{Type: graphql.String},
		"cursor":          &graphql.InputObjectFieldConfig{Type: graphql.String},
		"limit":           &graphql.InputObjectFieldConfig{Type: graphql.Int},
	},
})

var sandboxSnapshotPageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxSnapshotPage",
	Fields: graphql.Fields{
		"cursor":    gqlutil.OptionalStrField(func(p SnapshotPage) any { return p.Cursor }),
		"hasMore":   gqlutil.ReqBoolField(func(p SnapshotPage) any { return p.HasMore }),
		"snapshots": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(sandboxSnapshotGQLType))), func(p SnapshotPage) any { return p.Snapshots }),
	},
})

var sandboxBuildSourcesInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "SandboxBuildSourcesFilterInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"sandboxGroupId": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"ownerId":        &graphql.InputObjectFieldConfig{Type: graphql.String},
		"search":         &graphql.InputObjectFieldConfig{Type: graphql.String},
		"cursor":         &graphql.InputObjectFieldConfig{Type: graphql.String},
		"limit":          &graphql.InputObjectFieldConfig{Type: graphql.Int},
	},
})

var sandboxBuildSourceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxBuildSource",
	Fields: graphql.Fields{
		"name":        gqlutil.ReqStrField(func(s ImageSource) any { return s.Name }),
		"source":      gqlutil.Typed(graphql.NewNonNull(sandboxImageSourceGQLType), func(s ImageSource) any { return s.Source }),
		"latestBuild": gqlutil.Typed(graphql.NewNonNull(sandboxImageGQLType), func(s ImageSource) any { return s.LatestBuild }),
		"latestSucceededBuild": gqlutil.Typed(sandboxImageGQLType, func(s ImageSource) any {
			if s.LatestSucceededBuild == nil {
				return nil
			}
			return *s.LatestSucceededBuild
		}),
	},
})

var sandboxBuildSourcePageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxBuildSourcePage",
	Fields: graphql.Fields{
		"cursor":  gqlutil.OptionalStrField(func(p ImageSourcePage) any { return p.Cursor }),
		"hasMore": gqlutil.ReqBoolField(func(p ImageSourcePage) any { return p.HasMore }),
		"sources": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(sandboxBuildSourceGQLType))), func(p ImageSourcePage) any { return p.Sources }),
	},
})

// groupPageQueryFields are the queries of a sandbox group's pages.
func (s *Service) groupPageQueryFields() graphql.Fields {
	return graphql.Fields{
		"sandboxesForGroup": &graphql.Field{
			Type: graphql.NewNonNull(sandboxesForGroupPageGQLType),
			Args: graphql.FieldConfigArgument{"input": gqlutil.ReqArg(sandboxesForGroupInput)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				input, ctx := inputArg(p)
				after, err := timeArg(input, "createdAfter")
				if err != nil {
					return nil, err
				}
				before, err := timeArg(input, "createdBefore")
				if err != nil {
					return nil, err
				}
				return s.ListGroupSandboxes(ctx, GroupSandboxQuery{
					GroupID: gqlutil.Str(input, "sandboxGroupId"), Statuses: gqlutil.StringList(input["statuses"]),
					Search: gqlutil.Str(input, "search"), CreatedAfter: after, CreatedBefore: before,
					SourceSnapshotID: gqlutil.Str(input, "sourceSnapshotId"),
					SourceBuilds:     sourceBuilds(input),
					Cursor:           gqlutil.Str(input, "cursor"), Limit: gqlutil.Int(input, "limit"),
				})
			},
		},
		"sandboxGroupRuns": &graphql.Field{
			Type: graphql.NewNonNull(sandboxGroupRunsGQLType),
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"ownerId":        gqlutil.Arg(graphql.String),
				"endedAfter":     gqlutil.Arg(graphql.String),
				"endedBefore":    gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				after, err := timeArg(p.Args, "endedAfter")
				if err != nil {
					return nil, err
				}
				before, err := timeArg(p.Args, "endedBefore")
				if err != nil {
					return nil, err
				}
				return s.GroupRuns(gqlutil.OwnerScoped(p), p.Args["sandboxGroupId"].(string), after, before)
			},
		},
		"sandboxEvents": &graphql.Field{
			Type: graphql.NewNonNull(sandboxEventPageGQLType),
			Args: graphql.FieldConfigArgument{
				"sandboxId": gqlutil.ReqArg(graphql.String),
				"ownerId":   gqlutil.Arg(graphql.String),
				"before":    gqlutil.Arg(graphql.String),
				"beforeId":  gqlutil.Arg(graphql.String),
				"limit":     gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				before, err := timeArg(p.Args, "before")
				if err != nil {
					return nil, err
				}
				return s.ListEvents(gqlutil.OwnerScoped(p), p.Args["sandboxId"].(string), before, gqlutil.Str(p.Args, "beforeId"), gqlutil.Int(p.Args, "limit"))
			},
		},
		"sandboxSnapshotsForGroup": &graphql.Field{
			Type: graphql.NewNonNull(sandboxSnapshotPageGQLType),
			Args: graphql.FieldConfigArgument{"input": gqlutil.ReqArg(sandboxSnapshotsForGroupInput)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				input, ctx := inputArg(p)
				after, err := timeArg(input, "createdAfter")
				if err != nil {
					return nil, err
				}
				before, err := timeArg(input, "createdBefore")
				if err != nil {
					return nil, err
				}
				return s.SearchSnapshots(ctx, gqlutil.Str(input, "sandboxGroupId"), SnapshotQuery{
					Statuses: gqlutil.StringList(input["statuses"]), Search: gqlutil.Str(input, "search"),
					SourceSandboxID: gqlutil.Str(input, "sourceSandboxId"), CreatedAfter: after, CreatedBefore: before,
					Cursor: gqlutil.Str(input, "cursor"), Limit: gqlutil.Int(input, "limit"),
				})
			},
		},
		"sandboxBuildSources": &graphql.Field{
			Type: graphql.NewNonNull(sandboxBuildSourcePageGQLType),
			Args: graphql.FieldConfigArgument{"input": gqlutil.ReqArg(sandboxBuildSourcesInput)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				input, ctx := inputArg(p)
				return s.ListImageSources(ctx, gqlutil.Str(input, "sandboxGroupId"), gqlutil.Str(input, "search"),
					gqlutil.Str(input, "cursor"), gqlutil.Int(input, "limit"))
			},
		},
	}
}

// sourceBuilds is the input's sourceBuildId and sourceBuildIds together.
func sourceBuilds(input map[string]any) []string {
	builds := gqlutil.StringList(input["sourceBuildIds"])
	if id := gqlutil.Str(input, "sourceBuildId"); id != "" {
		builds = append(builds, id)
	}
	return builds
}

// runningSandboxCountField is Render's SandboxGroup.runningSandboxCount, read
// only when asked for: it lists the group's live sandboxes.
var runningSandboxCountField = &graphql.Field{
	Type: graphql.Int,
	Resolve: func(p graphql.ResolveParams) (any, error) {
		g, ok := p.Source.(Group)
		if !ok || g.running == nil {
			return nil, nil
		}
		return g.running(p.Context)
	},
}
