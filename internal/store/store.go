// Package store предоставляет хранилище домов и отзывов. Текущая реализация —
// потокобезопасное in-memory хранилище; интерфейс Store позволяет заменить
// его на БД без изменения слоёв service и web.
package store

import (
	"sort"
	"sync"

	"github.com/izmestevivan/mrg/internal/model"
)

// Store — абстракция доступа к данным домов и отзывов.
type Store interface {
	// SaveBuilding создаёт или обновляет дом (upsert по ключу адреса).
	SaveBuilding(b model.Building) error
	// Building возвращает дом по ключу. Второй результат — найден ли дом.
	Building(key string) (model.Building, bool, error)
	// AddReview сохраняет отзыв, проставляет ему ID и возвращает копию.
	AddReview(r model.Review) (model.Review, error)
	// ListByApartment возвращает отзывы по квартире в доме, свежие — первыми.
	ListByApartment(buildingKey string, apartmentNum int) ([]model.Review, error)
	// ListByBuilding возвращает все отзывы дома, свежие — первыми.
	ListByBuilding(buildingKey string) ([]model.Review, error)
	// ApartmentsByBuilding возвращает сводки по квартирам дома, у которых есть отзывы.
	ApartmentsByBuilding(buildingKey string) ([]model.ApartmentSummary, error)
	// Buildings возвращает сводки по всем домам, у которых есть отзывы.
	Buildings() ([]model.BuildingSummary, error)
}

// MemoryStore — реализация Store на основе срезов и карт в памяти.
type MemoryStore struct {
	mu        sync.RWMutex
	buildings map[string]model.Building
	reviews   []model.Review
	nextID    int64
}

// NewMemoryStore создаёт пустое in-memory хранилище.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		buildings: make(map[string]model.Building),
		nextID:    1,
	}
}

// SaveBuilding сохраняет дом, перезаписывая запись с тем же ключом.
func (s *MemoryStore) SaveBuilding(b model.Building) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buildings[b.Key] = b
	return nil
}

// Building возвращает дом по ключу.
func (s *MemoryStore) Building(key string) (model.Building, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.buildings[key]
	return b, ok, nil
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

// ListByApartment возвращает отзывы конкретной квартиры в доме,
// отсортированные по времени создания (свежие первыми).
func (s *MemoryStore) ListByApartment(buildingKey string, apartmentNum int) ([]model.Review, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []model.Review
	for _, r := range s.reviews {
		if r.BuildingKey == buildingKey && r.ApartmentNum == apartmentNum {
			out = append(out, r)
		}
	}
	sortNewestFirst(out)
	return out, nil
}

// ListByBuilding возвращает все отзывы дома, свежие — первыми.
func (s *MemoryStore) ListByBuilding(buildingKey string) ([]model.Review, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []model.Review
	for _, r := range s.reviews {
		if r.BuildingKey == buildingKey {
			out = append(out, r)
		}
	}
	sortNewestFirst(out)
	return out, nil
}

// ApartmentsByBuilding агрегирует отзывы дома в сводки по квартирам,
// отсортированные по номеру квартиры по возрастанию.
func (s *MemoryStore) ApartmentsByBuilding(buildingKey string) ([]model.ApartmentSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := map[int]int{}
	sums := map[int]int{}
	for _, r := range s.reviews {
		if r.BuildingKey != buildingKey {
			continue
		}
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

// Buildings агрегирует отзывы по домам, возвращая сводки только для домов с
// отзывами. Сортировка — по числу отзывов по убыванию, затем по адресу.
func (s *MemoryStore) Buildings() ([]model.BuildingSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := map[string]int{}
	sums := map[string]int{}
	for _, r := range s.reviews {
		counts[r.BuildingKey]++
		sums[r.BuildingKey] += r.Rating
	}

	out := make([]model.BuildingSummary, 0, len(counts))
	for key, cnt := range counts {
		out = append(out, model.BuildingSummary{
			Building:    s.buildings[key],
			ReviewCount: cnt,
			AvgRating:   float64(sums[key]) / float64(cnt),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ReviewCount != out[j].ReviewCount {
			return out[i].ReviewCount > out[j].ReviewCount
		}
		return out[i].Building.Key < out[j].Building.Key
	})
	return out, nil
}

func sortNewestFirst(reviews []model.Review) {
	sort.Slice(reviews, func(i, j int) bool {
		return reviews[i].CreatedAt.After(reviews[j].CreatedAt)
	})
}
