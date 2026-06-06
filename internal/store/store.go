// Package store предоставляет хранилище отзывов. Текущая реализация —
// потокобезопасное in-memory хранилище; интерфейс Store позволяет заменить
// его на БД без изменения слоёв service и web.
package store

import (
	"sort"
	"sync"

	"github.com/izmestevivan/mrg/internal/model"
)

// Store — абстракция доступа к данным отзывов.
type Store interface {
	// AddReview сохраняет отзыв, проставляет ему ID и возвращает сохранённую копию.
	AddReview(r model.Review) (model.Review, error)
	// ListByApartment возвращает отзывы по квартире, новые — первыми.
	ListByApartment(apartmentNum int) ([]model.Review, error)
	// Apartments возвращает сводки по всем квартирам, у которых есть отзывы.
	Apartments() ([]model.ApartmentSummary, error)
}

// MemoryStore — реализация Store на основе среза в памяти.
type MemoryStore struct {
	mu      sync.RWMutex
	reviews []model.Review
	nextID  int64
}

// NewMemoryStore создаёт пустое in-memory хранилище.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{nextID: 1}
}

// AddReview сохраняет копию отзыва с присвоенным идентификатором.
func (s *MemoryStore) AddReview(r model.Review) (model.Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r.ID = s.nextID
	s.nextID++
	s.reviews = append(s.reviews, r)
	return r, nil
}

// ListByApartment возвращает отзывы конкретной квартиры, отсортированные по
// времени создания (свежие первыми).
func (s *MemoryStore) ListByApartment(apartmentNum int) ([]model.Review, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []model.Review
	for _, r := range s.reviews {
		if r.ApartmentNum == apartmentNum {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// Apartments агрегирует отзывы в сводки по квартирам, отсортированные по
// номеру квартиры по возрастанию.
func (s *MemoryStore) Apartments() ([]model.ApartmentSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := map[int]int{}
	sums := map[int]int{}
	for _, r := range s.reviews {
		counts[r.ApartmentNum]++
		sums[r.ApartmentNum] += r.Rating
	}

	out := make([]model.ApartmentSummary, 0, len(counts))
	for num, cnt := range counts {
		out = append(out, model.ApartmentSummary{
			Number:      num,
			ReviewCount: cnt,
			AvgRating:   float64(sums[num]) / float64(cnt),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Number < out[j].Number
	})
	return out, nil
}
