package ai

import (
	"reflect"
	"testing"
)

func TestParseNames_NumberedDotFormat(t *testing.T) {
	input := "1. Alpha\n2. Beta\n3. Gamma\n"
	got := ParseNames(input)
	want := []string{"Alpha", "Beta", "Gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseNames_NumberedParenFormat(t *testing.T) {
	input := "1) Nexio\n2) Arkive\n3) Velox\n"
	got := ParseNames(input)
	want := []string{"Nexio", "Arkive", "Velox"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseNames_NumberedColonFormat(t *testing.T) {
	input := "1: Strato\n2: Lumio\n"
	got := ParseNames(input)
	want := []string{"Strato", "Lumio"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseNames_SkipsBlankLines(t *testing.T) {
	input := "\n1. Alpha\n\n2. Beta\n\n"
	got := ParseNames(input)
	want := []string{"Alpha", "Beta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseNames_SkipsNonNumberedLines(t *testing.T) {
	input := "Here are some names:\n1. Alpha\nThis is a description.\n2. Beta\n"
	got := ParseNames(input)
	want := []string{"Alpha", "Beta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseNames_StripsParentheticalSuffix(t *testing.T) {
	input := "1. Nexio (cloud platform)\n2. Arkive [storage]\n"
	got := ParseNames(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 names, got %v", got)
	}
	if got[0] != "Nexio" {
		t.Errorf("expected 'Nexio', got %q", got[0])
	}
	if got[1] != "Arkive" {
		t.Errorf("expected 'Arkive', got %q", got[1])
	}
}

func TestParseNames_DeduplicatesCaseInsensitive(t *testing.T) {
	input := "1. Alpha\n2. alpha\n3. ALPHA\n4. Beta\n"
	got := ParseNames(input)
	want := []string{"Alpha", "Beta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseNames_SkipsTooLongNames(t *testing.T) {
	long := "This is a very long name that exceeds the sixty character limit by a lot"
	input := "1. " + long + "\n2. Short\n"
	got := ParseNames(input)
	if len(got) != 1 || got[0] != "Short" {
		t.Errorf("expected only 'Short', got %v", got)
	}
}

func TestParseNames_EmptyInput(t *testing.T) {
	got := ParseNames("")
	if len(got) != 0 {
		t.Errorf("expected empty slice for empty input, got %v", got)
	}
}

func TestParseNames_PreservesFirstCasing(t *testing.T) {
	input := "1. NexIO\n2. nexio\n"
	got := ParseNames(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 name (deduped), got %v", got)
	}
	if got[0] != "NexIO" {
		t.Errorf("expected first-seen casing 'NexIO', got %q", got[0])
	}
}
