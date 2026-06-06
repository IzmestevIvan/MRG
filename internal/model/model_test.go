package model

import "testing"

func TestIsValidPro(t *testing.T) {
	cases := []struct {
		tag  Tag
		want bool
	}{
		{TagQuiet, true},
		{TagFriendly, true},
		{TagNoisy, false}, // отрицательный тег не является положительным
		{"выдуманное", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsValidPro(c.tag); got != c.want {
			t.Errorf("IsValidPro(%q) = %v, ожидалось %v", c.tag, got, c.want)
		}
	}
}

func TestIsValidCon(t *testing.T) {
	cases := []struct {
		tag  Tag
		want bool
	}{
		{TagNoisy, true},
		{TagSmoking, true},
		{TagQuiet, false}, // положительный тег не является отрицательным
		{"мусор", false},
	}
	for _, c := range cases {
		if got := IsValidCon(c.tag); got != c.want {
			t.Errorf("IsValidCon(%q) = %v, ожидалось %v", c.tag, got, c.want)
		}
	}
}

func TestAllTagsAreValid(t *testing.T) {
	for _, p := range AllProsTags() {
		if !IsValidPro(p) {
			t.Errorf("AllProsTags вернул недопустимый тег %q", p)
		}
	}
	for _, c := range AllConsTags() {
		if !IsValidCon(c) {
			t.Errorf("AllConsTags вернул недопустимый тег %q", c)
		}
	}
}

func TestTagSetsDoNotOverlap(t *testing.T) {
	for _, p := range AllProsTags() {
		if IsValidCon(p) {
			t.Errorf("тег %q одновременно положительный и отрицательный", p)
		}
	}
}
