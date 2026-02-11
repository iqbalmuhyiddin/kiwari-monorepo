package matcher

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// MatchStatus represents the status of a match operation
type MatchStatus int

const (
	Matched MatchStatus = iota
	Ambiguous
	Unmatched
)

func (s MatchStatus) String() string {
	switch s {
	case Matched:
		return "Matched"
	case Ambiguous:
		return "Ambiguous"
	case Unmatched:
		return "Unmatched"
	default:
		return "Unknown"
	}
}

// Item represents an inventory item with matching metadata
type Item struct {
	ID       uuid.UUID
	Code     string
	Name     string
	Keywords string // CSV like "cabe,merah,tanjung"
	Unit     string
}

// MatchResult contains the result of a matching operation
type MatchResult struct {
	Status     MatchStatus
	Item       *Item   // when Matched
	Candidates []Item  // when Ambiguous
}

// Matcher performs keyword-based item matching
type Matcher struct {
	items          []Item
	itemKeywordMap [][]string // pre-tokenized keywords per item
}

const (
	variantWeight = 5
	regularWeight = 1
)

var variantKeywords = map[string]bool{
	"merah":   true,
	"hijau":   true,
	"kuning":  true,
	"putih":   true,
	"tanjung": true,
	"kriting": true,
	"keriting": true,
	"besar":   true,
	"kecil":   true,
	"sedang":  true,
}

// New creates a new Matcher with pre-tokenized keywords
func New(items []Item) *Matcher {
	m := &Matcher{
		items:          items,
		itemKeywordMap: make([][]string, len(items)),
	}

	// Pre-tokenize keywords from CSV
	for i, item := range items {
		if item.Keywords != "" {
			// Split by comma and normalize each keyword
			parts := strings.Split(item.Keywords, ",")
			keywords := make([]string, 0, len(parts))
			for _, part := range parts {
				normalized := normalize(part)
				if normalized != "" {
					keywords = append(keywords, normalized)
				}
			}
			m.itemKeywordMap[i] = keywords
		}
	}

	return m
}

// Match performs keyword-based matching against inventory items
func (m *Matcher) Match(text string) MatchResult {
	// Normalize and tokenize input
	normalized := normalize(text)
	tokens := tokenize(normalized)

	// Extract quantity (we don't use it for matching, but remove it from description)
	_, _, descTokens := extractQuantity(tokens)

	// Convert desc tokens to map for fast lookup
	inputTokens := make(map[string]bool)
	for _, tok := range descTokens {
		inputTokens[tok] = true
	}

	// Extract variant keywords from input
	inputVariants := make(map[string]bool)
	for tok := range inputTokens {
		if variantKeywords[tok] {
			inputVariants[tok] = true
		}
	}

	// Score all items
	type scoredItem struct {
		item  Item
		score int
	}

	var scored []scoredItem

	for i, item := range m.items {
		keywords := m.itemKeywordMap[i]

		// Hard filter: if input contains variant keywords, candidate MUST have them
		if len(inputVariants) > 0 {
			hasAllVariants := true
			for variant := range inputVariants {
				found := false
				for _, kw := range keywords {
					if kw == variant {
						found = true
						break
					}
				}
				if !found {
					hasAllVariants = false
					break
				}
			}
			if !hasAllVariants {
				continue // Skip this item
			}
		}

		// Calculate score based on keyword intersections
		score := 0
		for _, kw := range keywords {
			if inputTokens[kw] {
				if variantKeywords[kw] {
					score += variantWeight
				} else {
					score += regularWeight
				}
			}
		}

		if score > 0 {
			scored = append(scored, scoredItem{item: item, score: score})
		}
	}

	// Determine result based on scores
	if len(scored) == 0 {
		return MatchResult{Status: Unmatched}
	}

	// Find max score
	maxScore := 0
	for _, s := range scored {
		if s.score > maxScore {
			maxScore = s.score
		}
	}

	// Collect all items with max score
	var topScorers []Item
	for _, s := range scored {
		if s.score == maxScore {
			topScorers = append(topScorers, s.item)
		}
	}

	if len(topScorers) == 1 {
		return MatchResult{
			Status: Matched,
			Item:   &topScorers[0],
		}
	}

	return MatchResult{
		Status:     Ambiguous,
		Candidates: topScorers,
	}
}

// normalize converts a string to lowercase and replaces non-alphanumeric chars with spaces
func normalize(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(unicode.ToLower(r))
		} else {
			sb.WriteRune(' ')
		}
	}

	// Collapse multiple spaces
	result := sb.String()
	result = strings.Join(strings.Fields(result), " ")

	return result
}

// tokenize splits a string on whitespace
func tokenize(s string) []string {
	return strings.Fields(s)
}

// extractQuantity finds quantity tokens like "5kg", "2L", "3pcs" and separates them
func extractQuantity(tokens []string) (qty float64, unit string, rest []string) {
	qty = 1
	unit = ""
	rest = make([]string, 0, len(tokens))

	for _, tok := range tokens {
		parsedQty, parsedUnit, ok := parseQtyUnit(tok)
		if ok {
			qty = parsedQty
			unit = parsedUnit
		} else {
			rest = append(rest, tok)
		}
	}

	return qty, unit, rest
}

// parseQtyUnit parses a token like "5kg" into (5, "kg", true)
func parseQtyUnit(tok string) (float64, string, bool) {
	if tok == "" {
		return 0, "", false
	}

	// Find the boundary between digits and letters
	digitEnd := 0
	for i, r := range tok {
		if unicode.IsDigit(r) || r == '.' {
			digitEnd = i + 1
		} else {
			break
		}
	}

	if digitEnd == 0 {
		return 0, "", false
	}

	numPart := tok[:digitEnd]
	unitPart := tok[digitEnd:]

	// Parse the number
	qty, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, "", false
	}

	// Unit part should be letters
	if unitPart == "" {
		return 0, "", false
	}

	for _, r := range unitPart {
		if !unicode.IsLetter(r) {
			return 0, "", false
		}
	}

	return qty, unitPart, true
}
