package store

import (
	"sync"
	"testing"
	"time"

	"github.com/izmestevivan/mrg/internal/model"
)

func addr(city, street, house string) model.Address {
	return model.Address{City: city, Street: street, House: house, Lat: 55.75, Lon: 37.61}
}

func TestAddReviewAssignsIncrementingIDs(t *testing.T) {
	s := NewMemoryStore()
	key := addr("Москва", "Тверская", "1").Key()

	r1, err := s.AddReview(model.Review{BuildingKey: key, ApartmentNum: 12, Rating: 5})
	if err != nil {
		t.Fatalf("AddReview: %v", err)
	}
	r2, _ := s.AddReview(model.Review{BuildingKey: key, ApartmentNum: 12, Rating: 3})

	if r1.ID != 1 || r2.ID != 2 {
		t.Errorf("ID должны быть 1 и 2, получено %d и %d", r1.ID, r2.ID)
	}
}

func TestSaveAndGetBuilding(t *testing.T) {
	s := NewMemoryStore()
	a := addr("Москва", "Тверская", "1")
	if err := s.SaveBuilding(model.Building{Key: a.Key(), Address: a}); err != nil {
		t.Fatalf("SaveBuilding: %v", err)
	}

	got, ok, err := s.Building(a.Key())
	if err != nil || !ok {
		t.Fatalf("Building: ok=%v err=%v", ok, err)
	}
	if got.Address.City != "Москва" {
		t.Errorf("неверный дом: %+v", got)
	}

	// Повторное сохранение обновляет (upsert), не плодит дубликаты.
	a.House = "1с2"
	updated := model.Building{Key: a.Key(), Address: a}
	_ = s.SaveBuilding(updated)
	if _, ok, _ := s.Building(a.Key()); !ok {
		t.Error("дом с обновлённым ключом не найден")
	}
}

func TestBuildingMissing(t *testing.T) {
	s := NewMemoryStore()
	if _, ok, _ := s.Building("нет|такого|дома"); ok {
		t.Error("ожидалось, что дом не найден")
	}
}

func TestListByApartmentFiltersByBuildingAndApartment(t *testing.T) {
	s := NewMemoryStore()
	k1 := addr("Москва", "Тверская", "1").Key()
	k2 := addr("Казань", "Баумана", "5").Key()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	_, _ = s.AddReview(model.Review{BuildingKey: k1, ApartmentNum: 5, Rating: 4, CreatedAt: base})
	_, _ = s.AddReview(model.Review{BuildingKey: k1, ApartmentNum: 9, Rating: 2, CreatedAt: base.Add(time.Hour)})
	_, _ = s.AddReview(model.Review{BuildingKey: k2, ApartmentNum: 5, Rating: 1, CreatedAt: base.Add(2 * time.Hour)})
	_, _ = s.AddReview(model.Review{BuildingKey: k1, ApartmentNum: 5, Rating: 3, CreatedAt: base.Add(3 * time.Hour)})

	got, err := s.ListByApartment(k1, 5)
	if err != nil {
		t.Fatalf("ListByApartment: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 отзыва (дом1, кв5), получено %d", len(got))
	}
	// Свежий (3 часа) первым.
	if got[0].Rating != 3 || got[1].Rating != 4 {
		t.Errorf("неверный порядок: %+v", got)
	}
}

func TestListByBuilding(t *testing.T) {
	s := NewMemoryStore()
	k := addr("Москва", "Тверская", "1").Key()
	_, _ = s.AddReview(model.Review{BuildingKey: k, ApartmentNum: 1, Rating: 4})
	_, _ = s.AddReview(model.Review{BuildingKey: k, ApartmentNum: 2, Rating: 5})
	_, _ = s.AddReview(model.Review{BuildingKey: "другой|дом|2", ApartmentNum: 1, Rating: 1})

	got, err := s.ListByBuilding(k)
	if err != nil {
		t.Fatalf("ListByBuilding: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ожидалось 2 отзыва дома, получено %d", len(got))
	}
}

func TestApartmentsByBuildingAggregatesAndSorts(t *testing.T) {
	s := NewMemoryStore()
	k := addr("Москва", "Тверская", "1").Key()
	_, _ = s.AddReview(model.Review{BuildingKey: k, ApartmentNum: 10, Rating: 4})
	_, _ = s.AddReview(model.Review{BuildingKey: k, ApartmentNum: 10, Rating: 2})
	_, _ = s.AddReview(model.Review{BuildingKey: k, ApartmentNum: 3, Rating: 5})

	got, err := s.ApartmentsByBuilding(k)
	if err != nil {
		t.Fatalf("ApartmentsByBuilding: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 квартиры, получено %d", len(got))
	}
	if got[0].Number != 3 || got[1].Number != 10 {
		t.Fatalf("неверная сортировка: %+v", got)
	}
	if got[1].AvgRating != 3 { // (4+2)/2
		t.Errorf("кв.10 средняя = %v, ожидалось 3", got[1].AvgRating)
	}
}

func TestBuildingsAggregatesAndSortsByReviewCount(t *testing.T) {
	s := NewMemoryStore()
	a1 := addr("Москва", "Тверская", "1")
	a2 := addr("Казань", "Баумана", "5")
	_ = s.SaveBuilding(model.Building{Key: a1.Key(), Address: a1})
	_ = s.SaveBuilding(model.Building{Key: a2.Key(), Address: a2})

	_, _ = s.AddReview(model.Review{BuildingKey: a1.Key(), ApartmentNum: 1, Rating: 5})
	_, _ = s.AddReview(model.Review{BuildingKey: a2.Key(), ApartmentNum: 1, Rating: 4})
	_, _ = s.AddReview(model.Review{BuildingKey: a2.Key(), ApartmentNum: 2, Rating: 2})

	got, err := s.Buildings()
	if err != nil {
		t.Fatalf("Buildings: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 дома, получено %d", len(got))
	}
	// Казань (2 отзыва) идёт раньше Москвы (1 отзыв).
	if got[0].Building.Address.City != "Казань" {
		t.Errorf("сортировка по числу отзывов нарушена: %+v", got)
	}
	if got[0].ReviewCount != 2 || got[0].AvgRating != 3 {
		t.Errorf("агрегация Казани неверна: %+v", got[0])
	}
}

// TestConcurrentAddReview проверяет потокобезопасность под -race.
func TestConcurrentAddReview(t *testing.T) {
	s := NewMemoryStore()
	k := addr("Москва", "Тверская", "1").Key()
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, _ = s.AddReview(model.Review{BuildingKey: k, ApartmentNum: 1, Rating: 3})
		}()
	}
	wg.Wait()

	got, _ := s.ListByApartment(k, 1)
	if len(got) != n {
		t.Errorf("ожидалось %d отзывов, получено %d", n, len(got))
	}
}
