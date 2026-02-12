package parser

import (
	"testing"
	"time"
)

func TestParseDateLine(t *testing.T) {
	tests := []struct {
		input string
		month time.Month
		day   int
		ok    bool
	}{
		{"20 jan", time.January, 20, true},
		{"5 feb", time.February, 5, true},
		{"15 mei", time.May, 15, true},
		{"1 agu", time.August, 1, true},
		{"31 des", time.December, 31, true},
		{"10 oktober", time.October, 10, true},
		{"not a date", 0, 0, false},
		{"", 0, 0, false},
		{"cabe merah 5kg", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			date, ok := parseDateLine(tt.input)
			if ok != tt.ok {
				t.Fatalf("parseDateLine(%q): got ok=%v, want %v", tt.input, ok, tt.ok)
			}
			if !ok {
				return
			}
			if date.Month() != tt.month || date.Day() != tt.day {
				t.Errorf("parseDateLine(%q): got %v-%v, want %v-%v",
					tt.input, date.Month(), date.Day(), tt.month, tt.day)
			}
		})
	}
}

func TestParsePrice(t *testing.T) {
	tests := []struct {
		input string
		price float64
		ok    bool
	}{
		{"500k", 500000, true},
		{"72k", 72000, true},
		{"1.5jt", 1500000, true},
		{"2jt", 2000000, true},
		{"300rb", 300000, true},
		{"25.5k", 25500, true},
		{"cabe", 0, false},
		{"5kg", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			price, ok := parsePrice(tt.input)
			if ok != tt.ok {
				t.Fatalf("parsePrice(%q): got ok=%v, want %v", tt.input, ok, tt.ok)
			}
			if ok && price != tt.price {
				t.Errorf("parsePrice(%q): got %v, want %v", tt.input, price, tt.price)
			}
		})
	}
}

func TestParseItemLine(t *testing.T) {
	tests := []struct {
		input       string
		description string
		qty         float64
		unit        string
		totalPrice  float64
	}{
		{"cabe merah tanjung 5kg 500k", "cabe merah tanjung", 5, "kg", 500000},
		{"beras sania 20kg 300k", "beras sania", 20, "kg", 300000},
		{"minyak 2L 72k", "minyak", 2, "l", 72000},
		{"gas 12kg 200k", "gas", 12, "kg", 200000},
		{"bensin 100k", "bensin", 1, "", 100000},
		{"tisu 3pack 45k", "tisu", 3, "pack", 45000},
		{"garam 10bks 50k", "garam", 10, "bks", 50000},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			item, err := parseItemLine(tt.input)
			if err != nil {
				t.Fatalf("parseItemLine(%q): %v", tt.input, err)
			}
			if item.Description != tt.description {
				t.Errorf("description: got %q, want %q", item.Description, tt.description)
			}
			if item.Qty != tt.qty {
				t.Errorf("qty: got %v, want %v", item.Qty, tt.qty)
			}
			if item.Unit != tt.unit {
				t.Errorf("unit: got %q, want %q", item.Unit, tt.unit)
			}
			if item.TotalPrice != tt.totalPrice {
				t.Errorf("totalPrice: got %v, want %v", item.TotalPrice, tt.totalPrice)
			}
		})
	}
}

func TestParseMessage(t *testing.T) {
	msg := "20 jan\ncabe merah tanjung 5kg 500k\nberas sania 20kg 300k\nminyak 2L 72k"

	result, err := ParseMessage(msg)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	if result.ExpenseDate.Month() != time.January || result.ExpenseDate.Day() != 20 {
		t.Errorf("date: got %v, want Jan 20", result.ExpenseDate)
	}
	if len(result.Items) != 3 {
		t.Fatalf("items: got %d, want 3", len(result.Items))
	}
	if result.Items[0].Description != "cabe merah tanjung" {
		t.Errorf("item 0 desc: got %q", result.Items[0].Description)
	}
	if result.Items[0].TotalPrice != 500000 {
		t.Errorf("item 0 price: got %v", result.Items[0].TotalPrice)
	}
	if result.Items[2].Description != "minyak" {
		t.Errorf("item 2 desc: got %q", result.Items[2].Description)
	}
}

func TestParseMessage_NoDate(t *testing.T) {
	msg := "cabe merah 5kg 500k\nberas 20kg 300k"

	_, err := ParseMessage(msg)
	if err == nil {
		t.Fatal("expected error for missing date")
	}
}

func TestParseMessage_EmptyLines(t *testing.T) {
	msg := "20 jan\n\ncabe merah 5kg 500k\n\nberas 20kg 300k\n"

	result, err := ParseMessage(msg)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items: got %d, want 2", len(result.Items))
	}
}
