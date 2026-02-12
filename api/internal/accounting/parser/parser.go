package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ParsedMessage is the result of parsing a WhatsApp reimbursement message.
type ParsedMessage struct {
	ExpenseDate time.Time
	Items       []ParsedItem
	Warnings    []string // Lines that failed to parse
}

// ParsedItem is a single item parsed from a WhatsApp message line.
type ParsedItem struct {
	RawText     string
	Description string
	Qty         float64
	Unit        string
	TotalPrice  float64
}

var indonesianMonths = map[string]time.Month{
	"jan": time.January, "januari": time.January,
	"feb": time.February, "februari": time.February,
	"mar": time.March, "maret": time.March,
	"apr": time.April, "april": time.April,
	"mei": time.May,
	"jun": time.June, "juni": time.June,
	"jul": time.July, "juli": time.July,
	"agu": time.August, "ags": time.August, "agustus": time.August,
	"sep": time.September, "september": time.September,
	"okt": time.October, "oktober": time.October,
	"nov": time.November, "november": time.November,
	"des": time.December, "desember": time.December,
}

// Known quantity units (NOT price suffixes).
var qtyUnits = map[string]bool{
	"kg": true, "g": true, "l": true, "ml": true,
	"pcs": true, "bks": true, "pack": true, "box": true,
	"ikat": true, "iket": true, "lbr": true, "btl": true,
	"ltr": true, "buah": true, "bh": true, "lembar": true,
	"sdm": true, "sdt": true, "ekor": true, "btr": true,
}

// ParseMessage parses a WhatsApp reimbursement message into structured data.
// First non-empty line must be a date (e.g. "20 jan"). Subsequent lines are items.
func ParseMessage(text string) (*ParsedMessage, error) {
	lines := strings.Split(text, "\n")

	var expenseDate time.Time
	var dateFound bool
	var items []ParsedItem
	var warnings []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !dateFound {
			date, ok := parseDateLine(line)
			if ok {
				expenseDate = date
				dateFound = true
				continue
			}
			return nil, fmt.Errorf("first line must be a date, got: %q", line)
		}

		item, err := parseItemLine(line)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipped: %s", line))
			continue
		}
		items = append(items, *item)
	}

	if !dateFound {
		return nil, fmt.Errorf("no date found in message")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no items found in message")
	}

	return &ParsedMessage{
		ExpenseDate: expenseDate,
		Items:       items,
		Warnings:    warnings,
	}, nil
}

// parseDateLine tries to parse a line as an Indonesian date (e.g. "20 jan").
func parseDateLine(line string) (time.Time, bool) {
	line = strings.TrimSpace(strings.ToLower(line))
	parts := strings.Fields(line)
	if len(parts) != 2 {
		return time.Time{}, false
	}

	day, err := strconv.Atoi(parts[0])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}

	month, ok := indonesianMonths[parts[1]]
	if !ok {
		return time.Time{}, false
	}

	now := time.Now()
	year := now.Year()
	parsed := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

	// If parsed date is more than 30 days in the future, assume previous year
	// (handles Dec messages submitted in January)
	if parsed.After(now.AddDate(0, 0, 30)) {
		parsed = time.Date(year-1, month, day, 0, 0, 0, 0, time.UTC)
	}

	return parsed, true
}

// parseItemLine parses a single item line (e.g. "cabe merah 5kg 500k").
func parseItemLine(line string) (*ParsedItem, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	tokens := strings.Fields(strings.ToLower(line))

	var totalPrice float64
	var qty float64 = 1
	var unit string
	var descTokens []string
	var priceFound, qtyFound bool

	for _, tok := range tokens {
		if p, ok := parsePrice(tok); ok && !priceFound {
			totalPrice = p
			priceFound = true
		} else if q, u, ok := parseQtyUnitToken(tok); ok && !qtyFound {
			qty = q
			unit = u
			qtyFound = true
		} else {
			descTokens = append(descTokens, tok)
		}
	}

	if !priceFound {
		return nil, fmt.Errorf("no price found in line: %q", line)
	}

	return &ParsedItem{
		RawText:     line,
		Description: strings.Join(descTokens, " "),
		Qty:         qty,
		Unit:        unit,
		TotalPrice:  totalPrice,
	}, nil
}

// parsePrice parses price shortcuts: "500k" → 500000, "1.5jt" → 1500000, "300rb" → 300000.
func parsePrice(tok string) (float64, bool) {
	tok = strings.ToLower(tok)

	type suffix struct {
		s string
		m float64
	}
	suffixes := []suffix{
		{"jt", 1_000_000},
		{"rb", 1_000},
		{"k", 1_000},
	}

	for _, sf := range suffixes {
		if strings.HasSuffix(tok, sf.s) {
			numStr := tok[:len(tok)-len(sf.s)]
			if numStr == "" {
				continue
			}
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				continue
			}
			return num * sf.m, true
		}
	}

	return 0, false
}

// parseQtyUnitToken parses "5kg" → (5, "kg", true). Only matches known units.
func parseQtyUnitToken(tok string) (float64, string, bool) {
	if tok == "" {
		return 0, "", false
	}

	// Find boundary between digits and letters
	digitEnd := 0
	for i, r := range tok {
		if unicode.IsDigit(r) || r == '.' {
			digitEnd = i + 1
		} else {
			break
		}
	}

	if digitEnd == 0 || digitEnd == len(tok) {
		return 0, "", false
	}

	numPart := tok[:digitEnd]
	unitPart := tok[digitEnd:]

	// Must be a known quantity unit
	if !qtyUnits[unitPart] {
		return 0, "", false
	}

	qty, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, "", false
	}

	return qty, unitPart, true
}
