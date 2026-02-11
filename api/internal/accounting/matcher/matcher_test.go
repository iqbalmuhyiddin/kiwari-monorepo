package matcher

import (
	"testing"

	"github.com/google/uuid"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "mixed case",
			input:    "Cabe Merah Tanjung",
			expected: "cabe merah tanjung",
		},
		{
			name:     "multiple spaces",
			input:    "BERAS  Sania",
			expected: "beras sania",
		},
		{
			name:     "comma separator",
			input:    "minyak,goreng",
			expected: "minyak goreng",
		},
		{
			name:     "special characters",
			input:    "test.item!",
			expected: "test item",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalize(tt.input)
			if result != tt.expected {
				t.Errorf("normalize(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	input := "cabe merah tanjung 5kg 500k"
	tokens := tokenize(input)

	expectedCount := 5
	if len(tokens) != expectedCount {
		t.Errorf("tokenize(%q) returned %d tokens, want %d", input, len(tokens), expectedCount)
	}

	expectedTokens := []string{"cabe", "merah", "tanjung", "5kg", "500k"}
	for i, expected := range expectedTokens {
		if tokens[i] != expected {
			t.Errorf("token[%d] = %q, want %q", i, tokens[i], expected)
		}
	}
}

func TestExtractQuantity(t *testing.T) {
	tests := []struct {
		name         string
		tokens       []string
		expectedQty  float64
		expectedUnit string
		expectedRest []string
	}{
		{
			name:         "5kg",
			tokens:       []string{"cabe", "merah", "5kg"},
			expectedQty:  5,
			expectedUnit: "kg",
			expectedRest: []string{"cabe", "merah"},
		},
		{
			name:         "2L",
			tokens:       []string{"minyak", "goreng", "2L"},
			expectedQty:  2,
			expectedUnit: "L",
			expectedRest: []string{"minyak", "goreng"},
		},
		{
			name:         "20kg",
			tokens:       []string{"beras", "20kg"},
			expectedQty:  20,
			expectedUnit: "kg",
			expectedRest: []string{"beras"},
		},
		{
			name:         "3pcs",
			tokens:       []string{"item", "test", "3pcs"},
			expectedQty:  3,
			expectedUnit: "pcs",
			expectedRest: []string{"item", "test"},
		},
		{
			name:         "10bks",
			tokens:       []string{"rokok", "10bks"},
			expectedQty:  10,
			expectedUnit: "bks",
			expectedRest: []string{"rokok"},
		},
		{
			name:         "1iket",
			tokens:       []string{"kangkung", "1iket"},
			expectedQty:  1,
			expectedUnit: "iket",
			expectedRest: []string{"kangkung"},
		},
		{
			name:         "no quantity",
			tokens:       []string{"cabe", "merah"},
			expectedQty:  1,
			expectedUnit: "",
			expectedRest: []string{"cabe", "merah"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qty, unit, rest := extractQuantity(tt.tokens)

			if qty != tt.expectedQty {
				t.Errorf("extractQuantity(%v) qty = %f, want %f", tt.tokens, qty, tt.expectedQty)
			}
			if unit != tt.expectedUnit {
				t.Errorf("extractQuantity(%v) unit = %q, want %q", tt.tokens, unit, tt.expectedUnit)
			}
			if len(rest) != len(tt.expectedRest) {
				t.Errorf("extractQuantity(%v) rest length = %d, want %d", tt.tokens, len(rest), len(tt.expectedRest))
			} else {
				for i, r := range rest {
					if r != tt.expectedRest[i] {
						t.Errorf("extractQuantity(%v) rest[%d] = %q, want %q", tt.tokens, i, r, tt.expectedRest[i])
					}
				}
			}
		})
	}
}

func TestMatchItems_SingleMatch(t *testing.T) {
	items := []Item{
		{
			ID:       uuid.MustParse("00000000-0000-0000-0000-000000000012"),
			Code:     "ITEM0012",
			Name:     "Cabe Merah Tanjung",
			Keywords: "cabe,merah,tanjung",
			Unit:     "kg",
		},
		{
			ID:       uuid.MustParse("00000000-0000-0000-0000-000000000013"),
			Code:     "ITEM0013",
			Name:     "Cabe Merah Kriting",
			Keywords: "cabe,merah,kriting,keriting",
			Unit:     "kg",
		},
	}

	matcher := New(items)
	result := matcher.Match("cabe merah tanjung")

	if result.Status != Matched {
		t.Errorf("Match status = %v, want Matched", result.Status)
	}

	if result.Item == nil {
		t.Fatal("Match result.Item is nil, expected ITEM0012")
	}

	if result.Item.Code != "ITEM0012" {
		t.Errorf("Matched item code = %q, want ITEM0012", result.Item.Code)
	}
}

func TestMatchItems_Ambiguous(t *testing.T) {
	items := []Item{
		{
			ID:       uuid.MustParse("00000000-0000-0000-0000-000000000012"),
			Code:     "ITEM0012",
			Name:     "Cabe Merah Tanjung",
			Keywords: "cabe,merah,tanjung",
			Unit:     "kg",
		},
		{
			ID:       uuid.MustParse("00000000-0000-0000-0000-000000000013"),
			Code:     "ITEM0013",
			Name:     "Cabe Merah Kriting",
			Keywords: "cabe,merah,kriting,keriting",
			Unit:     "kg",
		},
	}

	matcher := New(items)
	result := matcher.Match("cabe merah")

	if result.Status != Ambiguous {
		t.Errorf("Match status = %v, want Ambiguous", result.Status)
	}

	if len(result.Candidates) != 2 {
		t.Errorf("Candidates count = %d, want 2", len(result.Candidates))
	}
}

func TestMatchItems_Unmatched(t *testing.T) {
	items := []Item{
		{
			ID:       uuid.MustParse("00000000-0000-0000-0000-000000000012"),
			Code:     "ITEM0012",
			Name:     "Cabe Merah Tanjung",
			Keywords: "cabe,merah,tanjung",
			Unit:     "kg",
		},
	}

	matcher := New(items)
	result := matcher.Match("unknown item xyz")

	if result.Status != Unmatched {
		t.Errorf("Match status = %v, want Unmatched", result.Status)
	}
}

func TestMatchItems_VariantFilter(t *testing.T) {
	items := []Item{
		{
			ID:       uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			Code:     "ITEM0001",
			Name:     "Cabe Hijau",
			Keywords: "cabe,hijau",
			Unit:     "kg",
		},
		{
			ID:       uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			Code:     "ITEM0002",
			Name:     "Cabe Merah Tanjung",
			Keywords: "cabe,merah,tanjung",
			Unit:     "kg",
		},
	}

	matcher := New(items)
	result := matcher.Match("cabe hijau")

	if result.Status != Matched {
		t.Errorf("Match status = %v, want Matched", result.Status)
	}

	if result.Item == nil {
		t.Fatal("Match result.Item is nil, expected ITEM0001")
	}

	if result.Item.Code != "ITEM0001" {
		t.Errorf("Matched item code = %q, want ITEM0001", result.Item.Code)
	}
}
