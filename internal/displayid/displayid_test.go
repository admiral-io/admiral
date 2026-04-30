package displayid

import "testing"

const testPrefix = "cs"

func TestGenerateForm(t *testing.T) {
	id := Generate(testPrefix)
	if !Is(id, testPrefix) {
		t.Fatalf("generated id %q does not match display-id form", id)
	}
	if len(id) != len(testPrefix)+1+suffixLen {
		t.Fatalf("unexpected length: %d (id=%q)", len(id), id)
	}
}

func TestIsRejectsLookalikes(t *testing.T) {
	// 'i', 'l', 'o', 'u' are intentionally absent from the alphabet.
	cases := []string{
		"cs-iiiiiiiiiiii",
		"cs-llllllllllll",
		"cs-oooooooooooo",
		"cs-uuuuuuuuuuuu",
	}
	for _, c := range cases {
		if Is(c, testPrefix) {
			t.Errorf("expected %q to be rejected", c)
		}
	}
}

func TestIsRejectsBadShape(t *testing.T) {
	cases := []string{
		"",
		"cs-",
		"cs-tooshort",
		"cs-toolongabcdefgh",
		"xx-aaaaaaaaaaaa",
		"3k7m9p2q4rvw",
		"550e8400-e29b-41d4-a716-446655440000",
	}
	for _, c := range cases {
		if Is(c, testPrefix) {
			t.Errorf("expected %q to be rejected", c)
		}
	}
}

func TestGenerateUniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for range 1000 {
		id := Generate(testPrefix)
		if _, dup := seen[id]; dup {
			t.Fatalf("collision after %d ids: %q", len(seen), id)
		}
		seen[id] = struct{}{}
	}
}
