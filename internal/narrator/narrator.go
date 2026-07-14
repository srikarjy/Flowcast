// Package narrator sends FlowCast's classifier findings to an LLM API and
// returns confidence-tagged claims. Every claim must carry a
// confidence_tag of Observed, Reported, or Unknown (CLAUDE.md Cardinal
// Rule 5) — the narrator must never guess at causation it hasn't measured,
// and Unknown is a normal, expected, frequent output, not a failure.
//
// Provider: OpenAI (CLAUDE.md v3 scope amendment, 2026-07-13 — a
// credential-availability decision, not a technical one; see CLAUDE.md for
// the full record). Called via net/http + encoding/json against the Chat
// Completions API rather than an unverified third-party Go SDK.
package narrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"flowcast/internal/classify"
)

// Claim is one confidence-tagged statement from the narrator.
type Claim struct {
	Claim          string `json:"claim"`
	ConfidenceTag  string `json:"confidence_tag"` // "Observed", "Reported", or "Unknown"
	EvidenceSource string `json:"evidence_source"`
}

const systemPrompt = `You are FlowCast's failure narrator. You narrate nf-core/rnaseq pipeline QC classifier findings for a bioinformatics audience.

Rules, non-negotiable:
- Every claim you make must carry a confidence_tag: exactly one of "Observed", "Reported", or "Unknown".
- Observed: a fact computed directly from the numbers given to you below (the classifier finding, the QC values).
- Reported: a documented mechanism from the reasoning document given to you below — not something you infer, and not general training knowledge beyond what's given.
- Unknown: use this whenever a claim would require a causal explanation you have not been given evidence for. Do not guess at root cause. Unknown is a normal, expected, frequent answer — do not avoid it just to sound more confident.
- Never state or imply why a sample is an outlier unless the reasoning document explicitly establishes that cause for this data. If the reasoning document says the root cause is unresolved, output a claim with confidence_tag "Unknown" saying so explicitly — do not speculate (not biological, not technical, not batch-effect), even as a hedge.
- evidence_source must cite exactly where the claim comes from: a specific field name and value from the finding, or a specific section of the reasoning document. Never cite general knowledge.
- Output only claims supported by the material given to you. If there isn't enough to support a claim, output fewer claims rather than inventing one.`

const model = "gpt-4o"

var outputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"claims": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"claim":           map[string]any{"type": "string"},
					"confidence_tag":  map[string]any{"type": "string", "enum": []string{"Observed", "Reported", "Unknown"}},
					"evidence_source": map[string]any{"type": "string"},
				},
				"required":             []string{"claim", "confidence_tag", "evidence_source"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"claims"},
	"additionalProperties": false,
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type jsonSchemaSpec struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type responseFormat struct {
	Type       string         `json:"type"`
	JSONSchema jsonSchemaSpec `json:"json_schema"`
}

type chatCompletionRequest struct {
	Model          string         `json:"model"`
	Messages       []chatMessage  `json:"messages"`
	ResponseFormat responseFormat `json:"response_format"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Narrate sends classifier findings plus the causal reasoning document to
// the configured LLM and returns the confidence-tagged claims it produces.
// reasoningDoc is the raw contents of REASONING.md — the only source of
// "Reported" mechanisms the narrator is allowed to cite (CLAUDE.md Cardinal
// Rule 4). Reads the API key from OPENAI_API_KEY.
func Narrate(ctx context.Context, findings []classify.Finding, reasoningDoc string) ([]Claim, error) {
	if len(findings) == 0 {
		return nil, nil
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("narrator: OPENAI_API_KEY not set")
	}

	var b strings.Builder
	b.WriteString("Classifier findings from this run:\n\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "- rule=%s sample=%s detail=%s\n", f.Rule, f.Sample, f.Detail)
	}
	b.WriteString("\nCausal reasoning document (REASONING.md) — the only source of Reported mechanisms:\n\n")
	b.WriteString(reasoningDoc)

	reqBody := chatCompletionRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: b.String()},
		},
		ResponseFormat: responseFormat{
			Type: "json_schema",
			JSONSchema: jsonSchemaSpec{
				Name:   "narrator_claims",
				Strict: true,
				Schema: outputSchema,
			},
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("narrator: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("narrator: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("narrator: openai api request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("narrator: read response: %w", err)
	}

	var out chatCompletionResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("narrator: decode response: %w (body: %s)", err, string(raw))
	}
	if out.Error != nil {
		return nil, fmt.Errorf("narrator: openai api error: %s", out.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("narrator: openai api returned status %d: %s", resp.StatusCode, string(raw))
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("narrator: no choices in response")
	}

	var parsed struct {
		Claims []Claim `json:"claims"`
	}
	if err := json.Unmarshal([]byte(out.Choices[0].Message.Content), &parsed); err != nil {
		return nil, fmt.Errorf("narrator: decode structured output: %w", err)
	}
	return parsed.Claims, nil
}
