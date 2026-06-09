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

func TestAddressKeyNormalizes(t *testing.T) {
	a := Address{City: "  Москва ", Street: "Тверская  улица", House: "1", Lat: 1, Lon: 2}
	b := Address{City: "москва", Street: "тверская улица", House: "1", Lat: 9, Lon: 9}
	// Регистр, лишние пробелы и координаты не влияют на ключ.
	if a.Key() != b.Key() {
		t.Errorf("ключи должны совпадать: %q vs %q", a.Key(), b.Key())
	}
	if a.Key() != "москва|тверская улица|1" {
		t.Errorf("неожиданный ключ: %q", a.Key())
	}
}

func TestAddressKeyDistinguishesHouses(t *testing.T) {
	a := Address{City: "Москва", Street: "Тверская", House: "1"}
	b := Address{City: "Москва", Street: "Тверская", House: "2"}
	if a.Key() == b.Key() {
		t.Error("дома с разными номерами должны иметь разные ключи")
	}
}

func TestAddressDisplay(t *testing.T) {
	a := Address{City: "Москва", Street: "Тверская", House: "7"}
	if got := a.Display(); got != "Москва, Тверская, д. 7" {
		t.Errorf("Display = %q", got)
	}
}
