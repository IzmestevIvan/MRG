package store

import (
	"sync"
	"testing"
	"time"

	"github.com/izmestevivan/mrg/internal/model"
)

func TestAddReviewAssignsIncrementingIDs(t *testing.T) {
	s := NewMemoryStore()

	r1, err := s.AddReview(model.Review{ApartmentNum: 12, Rating: 5})
	if err != nil {
		t.Fatalf("AddReview: %v", err)
	}
	r2, _ := s.AddReview(model.Review{ApartmentNum: 12, Rating: 3})

	if r1.ID != 1 || r2.ID != 2 {
		t.Errorf("ID должны быть 1 и 2, получено %d и %d", r1.ID, r2.ID)
	}
}

func TestListByApartmentFiltersAndSortsNewestFirst(t *testing.T) {
	s := NewMemoryStore()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	_, _ = s.AddReview(model.Review{ApartmentNum: 5, Rating: 4, CreatedAt: base})
	_, _ = s.AddReview(model.Review{ApartmentNum: 9, Rating: 2, CreatedAt: base.Add(time.Hour)})
	_, _ = s.AddReview(model.Review{ApartmentNum: 5, Rating: 1, CreatedAt: base.Add(2 * time.Hour)})

	got, err := s.ListByApartment(5)
	if err != nil {
		t.Fatalf("ListByApartment: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 отзыва по кв.5, получено %d", len(got))
	}
	// Свежий (2 часа) должен идти первым.
	if got[0].Rating != 1 || got[1].Rating != 4 {
		t.Errorf("неверный порядок сортировки: %+v", got)
	}
}

func TestListByApartmentEmpty(t *testing.T) {
	s := NewMemoryStore()
	got, err := s.ListByApartment(404)
	if err != nil {
		t.Fatalf("ListByApartment: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ожидался пустой результат, получено %d", len(got))
	}
}

func TestApartmentsAggregatesAndSorts(t *testing.T) {
	s := NewMemoryStore()
	_, _ = s.AddReview(model.Review{ApartmentNum: 10, Rating: 4})
	_, _ = s.AddReview(model.Review{ApartmentNum: 10, Rating: 2})
	_, _ = s.AddReview(model.Review{ApartmentNum: 3, Rating: 5})

	got, err := s.Apartments()
	if err != nil {
		t.Fatalf("Apartments: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 квартиры, получено %d", len(got))
	}
	// Отсортировано по номеру: сначала кв.3, затем кв.10.
	if got[0].Number != 3 || got[1].Number != 10 {
		t.Fatalf("неверная сортировка квартир: %+v", got)
	}
	if got[0].AvgRating != 5 {
		t.Errorf("кв.3 средняя = %v, ожидалось 5", got[0].AvgRating)
	}
	if got[1].AvgRating != 3 { // (4+2)/2
		t.Errorf("кв.10 средняя = %v, ожидалось 3", got[1].AvgRating)
	}
	if got[1].ReviewCount != 2 {
		t.Errorf("кв.10 количество = %d, ожидалось 2", got[1].ReviewCount)
	}
}

// TestConcurrentAddReview проверяет потокобезопасность под -race.
func TestConcurrentAddReview(t *testing.T) {
	s := NewMemoryStore()
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, _ = s.AddReview(model.Review{ApartmentNum: 1, Rating: 3})
		}()
	}
	wg.Wait()

	got, _ := s.ListByApartment(1)
	if len(got) != n {
		t.Errorf("ожидалось %d отзывов, получено %d", n, len(got))
	}
}
