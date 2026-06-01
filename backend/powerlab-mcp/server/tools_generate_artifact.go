package server

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/neochaotic/powerlab/backend/powerlab-mcp/composevalidator"
)

// P1.6 from the 2026-05-31 MCP-only chat-mode retro: agents had no
// PROPOSE-then-review path for compose YAMLs (and other artifacts).
// install_app executes immediately; printing YAML in chat skips the
// deny-list validator. generate_artifact is the review-first
// alternative: agent submits a draft, Tool structures it + runs the
// per-kind validator (when one exists), returns the artifact for
// operator approval. Nothing persisted, nothing executed.
//
// Side-effect class: READ. The artifact is the response; no disk
// state, no upstream call, no install attempt.

// ArtifactValidation reports what (if anything) the per-kind
// validator said about the content. OK distinguishes "validator ran
// and accepted" from "no validator for this kind" — the latter
// surfaces a Note instead of silently claiming OK so the agent
// never represents an unvalidated artifact as safe.
type ArtifactValidation struct {
	OK         bool                       `json:"ok"`
	Note       string                     `json:"note,omitempty"`
	Violations []composevalidator.Violation `json:"violations,omitempty"`
}

// GenerateArtifactOutput is the structured envelope returned to the
// agent. The agent's UI can render this specially (a fenced block,
// a side-panel review pane) and surface the validation result so the
// operator decides with full context.
type GenerateArtifactOutput struct {
	Kind       string             `json:"kind"`
	Title      string             `json:"title"`
	Content    string             `json:"content"`
	Rationale  string             `json:"rationale,omitempty"`
	Validation ArtifactValidation `json:"validation"`
}

type generateArtifactInput struct {
	Kind      string `json:"kind" jsonschema:"required artifact kind; one of compose-yaml | shell-script | config-snippet | markdown"`
	Title     string `json:"title" jsonschema:"required short human-readable title shown to the operator"`
	Content   string `json:"content" jsonschema:"required draft body the agent proposes; must not be empty"`
	Rationale string `json:"rationale,omitempty" jsonschema:"optional explanation of why the agent generated this artifact; surfaced to operator"`
}

func registerGenerateArtifact(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "generate_artifact",
		Description: "READ ONLY — propose a draft artifact for operator review without executing or persisting it. Supported kinds: compose-yaml (validated against PowerLab's composevalidator deny-list), shell-script | config-snippet | markdown (no validator yet — validation.note explains). Use this BEFORE install_app when you want the operator to approve a YAML you authored; the artifact is the response, no install runs.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in generateArtifactInput) (*mcp.CallToolResult, GenerateArtifactOutput, error) {
		kind := strings.TrimSpace(in.Kind)
		title := strings.TrimSpace(in.Title)
		content := in.Content
		if title == "" {
			return nil, GenerateArtifactOutput{}, errors.New("title is required")
		}
		if strings.TrimSpace(content) == "" {
			return nil, GenerateArtifactOutput{}, errors.New("content is required and must not be empty")
		}
		out := GenerateArtifactOutput{
			Kind:      kind,
			Title:     title,
			Content:   content,
			Rationale: strings.TrimSpace(in.Rationale),
		}
		out.Validation = validateArtifact(kind, content)
		return nil, out, nil
	})
}

// validateArtifact dispatches to the per-kind validator. Today only
// compose-yaml has one (composevalidator deny-list). Other kinds
// roundtrip with OK=false + a Note so the agent reports the absence
// of validation explicitly. As validators for other kinds appear
// (shell-script linter, markdown frontmatter check, etc.) they wire
// in here.
func validateArtifact(kind, content string) ArtifactValidation {
	switch kind {
	case "compose-yaml":
		res := composevalidator.Validate([]byte(content))
		return ArtifactValidation{
			OK:         res.OK,
			Violations: res.Violations,
		}
	case "shell-script":
		return ArtifactValidation{
			OK:   false,
			Note: "no validator wired for shell-script yet; operator should review manually",
		}
	case "config-snippet":
		return ArtifactValidation{
			OK:   false,
			Note: "no validator wired for config-snippet yet; operator should review manually",
		}
	case "markdown":
		return ArtifactValidation{
			OK:   false,
			Note: "no validator wired for markdown yet; operator should review manually",
		}
	default:
		return ArtifactValidation{
			OK:   false,
			Note: "unknown kind " + kind + "; supported: compose-yaml, shell-script, config-snippet, markdown",
		}
	}
}
