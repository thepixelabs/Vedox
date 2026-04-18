package ai

import (
	"strings"
	"testing"
)

func TestBuildPrompt_BasicCount(t *testing.T) {
	p := BuildPrompt(GenerationParams{}, 5, nil)
	if !strings.Contains(p, "Generate exactly 5 product name suggestions") {
		t.Errorf("expected count=5 in prompt, got: %s", p[:80])
	}
}

func TestBuildPrompt_CapsAt20(t *testing.T) {
	p := BuildPrompt(GenerationParams{}, 99, nil)
	if !strings.Contains(p, "Generate exactly 20 product name suggestions") {
		t.Errorf("expected count capped at 20, got: %s", p[:80])
	}
}

func TestBuildPrompt_MinCount(t *testing.T) {
	p := BuildPrompt(GenerationParams{}, 0, nil)
	if !strings.Contains(p, "Generate exactly 10 product name suggestions") {
		t.Errorf("expected count defaulted to 10 for 0, got: %s", p[:80])
	}
}

func TestBuildPrompt_Categories(t *testing.T) {
	params := GenerationParams{Categories: []string{"SaaS", "DevTools"}}
	p := BuildPrompt(params, 5, nil)
	if !strings.Contains(p, "SaaS / DevTools") {
		t.Errorf("expected categories in prompt: %s", p)
	}
}

func TestBuildPrompt_Platform(t *testing.T) {
	params := GenerationParams{Platform: "web"}
	p := BuildPrompt(params, 5, nil)
	if !strings.Contains(p, "for web") {
		t.Errorf("expected platform in prompt: %s", p)
	}
}

func TestBuildPrompt_PlatformAnySkipped(t *testing.T) {
	params := GenerationParams{Platform: "any"}
	p := BuildPrompt(params, 5, nil)
	if strings.Contains(p, "for any") {
		t.Errorf("'any' platform should be skipped, got: %s", p)
	}
}

func TestBuildPrompt_OS(t *testing.T) {
	params := GenerationParams{OS: "macOS"}
	p := BuildPrompt(params, 5, nil)
	if !strings.Contains(p, "targeting macOS") {
		t.Errorf("expected OS in prompt: %s", p)
	}
}

func TestBuildPrompt_Interface(t *testing.T) {
	params := GenerationParams{Interface: "CLI"}
	p := BuildPrompt(params, 5, nil)
	if !strings.Contains(p, "with a CLI interface") {
		t.Errorf("expected interface in prompt: %s", p)
	}
}

func TestBuildPrompt_Audience(t *testing.T) {
	params := GenerationParams{Audience: "developers"}
	p := BuildPrompt(params, 5, nil)
	if !strings.Contains(p, "aimed at developers") {
		t.Errorf("expected audience in prompt: %s", p)
	}
}

func TestBuildPrompt_AudienceGeneralSkipped(t *testing.T) {
	params := GenerationParams{Audience: "general"}
	p := BuildPrompt(params, 5, nil)
	if strings.Contains(p, "aimed at general") {
		t.Errorf("'general' audience should be skipped, got: %s", p)
	}
}

func TestBuildPrompt_Tone(t *testing.T) {
	params := GenerationParams{Tone: "playful"}
	p := BuildPrompt(params, 5, nil)
	if !strings.Contains(p, "with a playful tone") {
		t.Errorf("expected tone in prompt: %s", p)
	}
}

func TestBuildPrompt_NameLengthShort(t *testing.T) {
	params := GenerationParams{NameLength: "short"}
	p := BuildPrompt(params, 5, nil)
	if !strings.Contains(p, "short") {
		t.Errorf("expected short length hint in prompt: %s", p)
	}
}

func TestBuildPrompt_NameLengthDescriptive(t *testing.T) {
	params := GenerationParams{NameLength: "descriptive"}
	p := BuildPrompt(params, 5, nil)
	if !strings.Contains(p, "descriptive") {
		t.Errorf("expected descriptive length hint in prompt: %s", p)
	}
}

func TestBuildPrompt_LanguageStyle(t *testing.T) {
	params := GenerationParams{LanguageStyle: "latin"}
	p := BuildPrompt(params, 5, nil)
	if !strings.Contains(p, "latin") {
		t.Errorf("expected language style in prompt: %s", p)
	}
}

func TestBuildPrompt_RefinementExact(t *testing.T) {
	ref := &RefinementInput{Mode: "exact", LikedNames: []string{"Alpha", "Beta"}}
	p := BuildPrompt(GenerationParams{}, 5, ref)
	if !strings.Contains(p, `"Alpha"`) || !strings.Contains(p, `"Beta"`) {
		t.Errorf("expected liked names in prompt: %s", p)
	}
	if !strings.Contains(p, "word-play") {
		t.Errorf("expected 'word-play' for exact mode: %s", p)
	}
}

func TestBuildPrompt_RefinementStyle(t *testing.T) {
	ref := &RefinementInput{Mode: "style", LikedNames: []string{"Nexio"}}
	p := BuildPrompt(GenerationParams{}, 5, ref)
	if !strings.Contains(p, "style/feeling") {
		t.Errorf("expected style hint in prompt: %s", p)
	}
}

func TestBuildPrompt_RefinementNilNoEffect(t *testing.T) {
	p := BuildPrompt(GenerationParams{}, 5, nil)
	if strings.Contains(p, "word-play") || strings.Contains(p, "style/feeling") {
		t.Errorf("nil refinement should not add refinement text: %s", p)
	}
}

func TestBuildPrompt_RefinementEmptyLikedNames(t *testing.T) {
	ref := &RefinementInput{Mode: "exact", LikedNames: nil}
	p := BuildPrompt(GenerationParams{}, 5, ref)
	if strings.Contains(p, "word-play") {
		t.Errorf("empty liked names should not add refinement text: %s", p)
	}
}

func TestBuildPrompt_AlwaysContainsNumberedListInstruction(t *testing.T) {
	p := BuildPrompt(GenerationParams{}, 5, nil)
	if !strings.Contains(p, "numbered list") {
		t.Errorf("prompt should always contain numbered list instruction: %s", p)
	}
}
