package ai

import (
	"fmt"
	"strings"
)

// GenerationParams holds all user-facing configuration for name generation.
type GenerationParams struct {
	Categories    []string `json:"categories"`
	Platform      string   `json:"platform"`
	OS            string   `json:"os"`
	Interface     string   `json:"interface"`
	Audience      string   `json:"audience"`
	Tone          string   `json:"tone"`
	NameLength    string   `json:"nameLength"`
	LanguageStyle string   `json:"languageStyle"`
}

// RefinementInput carries the user's selection and refinement mode.
type RefinementInput struct {
	// Mode is "exact" (word-play on selected names) or "style" (same vibe, new roots).
	Mode       string   `json:"mode"`
	LikedNames []string `json:"likedNames"`
}

// BuildPrompt constructs the full prompt string sent to the AI CLI.
// count is capped at 20 per request to keep output format reliable.
// If refinement is non-nil and has liked names, the prompt includes
// instructions to build on or match the style of those names.
func BuildPrompt(params GenerationParams, count int, refinement *RefinementInput) string {
	if count > 20 {
		count = 20
	}
	if count < 1 {
		count = 10
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Generate exactly %d product name suggestions", count))

	// Build the context clause from non-default param values.
	var ctx []string
	if len(params.Categories) > 0 {
		ctx = append(ctx, "in the "+strings.Join(params.Categories, " / ")+" space")
	}
	if params.Platform != "" && params.Platform != "any" {
		ctx = append(ctx, "for "+params.Platform)
	}
	if params.OS != "" && params.OS != "any" {
		ctx = append(ctx, "targeting "+params.OS)
	}
	if params.Interface != "" && params.Interface != "any" {
		ctx = append(ctx, "with a "+params.Interface+" interface")
	}
	if params.Audience != "" && params.Audience != "general" {
		ctx = append(ctx, "aimed at "+params.Audience)
	}
	if params.Tone != "" && params.Tone != "any" {
		ctx = append(ctx, "with a "+params.Tone+" tone")
	}
	if params.NameLength != "" && params.NameLength != "any" {
		switch params.NameLength {
		case "short":
			ctx = append(ctx, "keep names short (1-2 syllables preferred)")
		case "descriptive":
			ctx = append(ctx, "names can be descriptive or multi-word")
		}
	}
	if params.LanguageStyle != "" && params.LanguageStyle != "any" {
		ctx = append(ctx, "in a "+params.LanguageStyle+" naming style")
	}

	if len(ctx) > 0 {
		sb.WriteString(" " + strings.Join(ctx, ", "))
	}
	sb.WriteString(".")

	// Refinement instructions — only written when the user has liked names.
	if refinement != nil && len(refinement.LikedNames) > 0 {
		quoted := make([]string, len(refinement.LikedNames))
		for i, n := range refinement.LikedNames {
			quoted[i] = `"` + n + `"`
		}
		nameList := strings.Join(quoted, ", ")
		switch refinement.Mode {
		case "exact":
			sb.WriteString(fmt.Sprintf(
				" Base your suggestions on word-play and variations of these names the user liked: %s. Explore different spellings, compounds, and suffixes.",
				nameList,
			))
		case "style":
			sb.WriteString(fmt.Sprintf(
				" The user liked the style/feeling of these names: %s. Generate entirely new names in the same style but with completely different roots.",
				nameList,
			))
		}
	}

	sb.WriteString(
		"\n\nIMPORTANT: Output ONLY a numbered list, one name per line. No explanations, descriptions, or extra text." +
			"\nFormat: 1. Name\n2. Name\n...\n" +
			fmt.Sprintf("Output exactly %d names, no more, no less.", count),
	)

	return sb.String()
}
