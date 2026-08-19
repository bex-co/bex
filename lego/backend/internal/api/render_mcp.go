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

package api

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
)

// render_mcp.go pins Render's official MCP tool surface, closing the asymmetry
// that let bex's MCP adapter grow to many times upstream's size without anyone
// deciding to. REST parity has been machine-checked since w7/m49 —
// renderOpenAPISHA256 pins the spec, TestRenderConformance checks the surface
// against it, and render-schema-drift.yml watches upstream. MCP parity, until
// this file, was ten hand-written comments carrying manual check dates
// ("checked live 2026-07-13", "@ 2a00be1", "v0.3.0") that nothing verified and
// that had already gone stale in both directions.
//
// The pin records what upstream ACTUALLY registers: scripts/render-mcp-capture.py
// builds render-oss/render-mcp-server and drives it through the MCP stdio
// handshake to read its own tools/list. Tool names and argument names are the
// contractual surface; descriptions and annotations are excluded because they
// drift editorially.

//go:embed openapi/render-mcp-tools.json
var renderMCPToolsSource []byte

// renderMCPToolsSHA256 pins the embedded capture the way renderOpenAPISHA256
// pins the REST spec: a hand-edit or a truncated file fails loudly at load
// rather than silently weakening every parity assertion built on it.
const renderMCPToolsSHA256 = "28ac990ade694df68502b9d7e5a79473691b0f2ba2d09cca181d6bcad75fdca1"

// renderMCPTool is one upstream tool reduced to its contractual surface.
type renderMCPTool struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
	// Required is upstream's required-argument set. bex may relax a requirement
	// (its own ids differ), but a Parity1to1 tool may never ADD one — that would
	// reject a call the official server accepts.
	Required []string `json:"required"`
}

// renderMCPContract is the parsed pin.
type renderMCPContract struct {
	Source struct {
		Repo       string `json:"repo"`
		Ref        string `json:"ref"`
		Commit     string `json:"commit"`
		CommitDate string `json:"commitDate"`
	} `json:"source"`
	Tools []renderMCPTool `json:"tools"`

	byName map[string]renderMCPTool
}

// Tool returns the upstream tool of that name, if upstream registers one.
func (c *renderMCPContract) Tool(name string) (renderMCPTool, bool) {
	t, ok := c.byName[name]
	return t, ok
}

// Names returns every upstream tool name, sorted.
func (c *renderMCPContract) Names() []string {
	names := make([]string, 0, len(c.Tools))
	for _, t := range c.Tools {
		names = append(names, t.Name)
	}
	slices.Sort(names)
	return names
}

func loadRenderMCPContract() (*renderMCPContract, error) {
	return loadRenderMCPContractData(renderMCPToolsSource, renderMCPToolsSHA256)
}

func loadRenderMCPContractData(data []byte, expectedSHA256 string) (*renderMCPContract, error) {
	if err := verifyPinnedArtifact("Render MCP tool pin", data, expectedSHA256,
		"refresh with scripts/render-mcp-capture.py and update renderMCPToolsSHA256"); err != nil {
		return nil, err
	}

	var c renderMCPContract
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse embedded Render MCP tool pin: %w", err)
	}
	if len(c.Tools) == 0 {
		return nil, fmt.Errorf("embedded Render MCP tool pin contains no tools")
	}
	if c.Source.Commit == "" {
		return nil, fmt.Errorf("embedded Render MCP tool pin records no upstream commit")
	}

	c.byName = make(map[string]renderMCPTool, len(c.Tools))
	for _, t := range c.Tools {
		if t.Name == "" {
			return nil, fmt.Errorf("embedded Render MCP tool pin contains an unnamed tool")
		}
		if _, dup := c.byName[t.Name]; dup {
			return nil, fmt.Errorf("embedded Render MCP tool pin lists %q twice", t.Name)
		}
		c.byName[t.Name] = t
	}
	return &c, nil
}
