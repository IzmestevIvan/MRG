// Package service содержит бизнес-логику MRG: валидацию входных данных
// отзыва и адреса, нормализацию тегов и агрегацию оценок по домам и
// квартирам. Слой не зависит от HTTP.
package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/izmestevivan/mrg/internal/model"
	"github.com/izmestevivan/mrg/internal/store"
)

// Ограничения ввода.
const (
	// MaxCommentLen — максимальная длина комментария в рунах.
	MaxCommentLen = 500
	// MaxApartmentNum — верхняя граница номера квартиры (защита от мусора).
	MaxApartmentNum = 10000
	// MaxAddrFieldLen — максимальная длина части адреса в рунах.
	MaxAddrFieldLen = 200
)

// Ошибки валидации, которые слой web может отображать пользователю.
var (
	ErrInvalidAddress   = errors.New("укажите город, улицу и номер дома")
	ErrInvalidCoords    = errors.New("выберите дом на карте")
	ErrInvalidApartment = errors.New("номер квартиры должен быть от 1 до 10000")
	ErrInvalidRating    = errors.New("оценка должна быть от 1 до 5 звёзд")
	ErrUnknownTag       = errors.New("неизвестный тег")
	ErrCommentTooLong   = errors.New("комментарий слишком длинный")
)

// NewReview — входные данные для создания отзыва (без ID и времени).
type NewReview struct {
	Address      model.Address
	ApartmentNum int
	Rating       int
	Pros         []model.Tag
	Cons         []model.Tag
	Comment      string
}

// Clock возвращает текущее время. Вынесен в интерфейс ради детерминированных тестов.
type Clock func() time.Time

// Service инкапсулирует операции над домами и отзывами.
type Service struct {
	store store.Store
	now   Clock
}

// New создаёт сервис поверх хранилища. Если clock == nil, используется time.Now.
func New(s store.Store, clock Clock) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{store: s, now: clock}
}

// AddReview валидирует адрес и отзыв, создаёт/обновляет дом и сохраняет отзыв.
func (svc *Service) AddReview(in NewReview) (model.Review, error) {
	addr, err := cleanAddress(in.Address)
	if err != nil {
		return model.Review{}, err
	}
	if !validCoords(addr.Lat, addr.Lon) {
		return model.Review{}, ErrInvalidCoords
	}
	if in.ApartmentNum < 1 || in.ApartmentNum > MaxApartmentNum {
		return model.Review{}, ErrInvalidApartment
	}
	if in.Rating < model.MinRating || in.Rating > model.MaxRating {
		return model.Review{}, ErrInvalidRating
	}

	pros, err := normalizeTags(in.Pros, model.IsValidPro)
	if err != nil {
		return model.Review{}, err
	}
	cons, err := normalizeTags(in.Cons, model.IsValidCon)
	if err != nil {
		return model.Review{}, err
	}

	comment := strings.TrimSpace(in.Comment)
	if len([]rune(comment)) > MaxCommentLen {
		return model.Review{}, ErrCommentTooLong
	}

	key := addr.Key()
	if err := svc.store.SaveBuilding(model.Building{Key: key, Address: addr}); err != nil {
		return model.Review{}, err
	}

	return svc.store.AddReview(model.Review{
		BuildingKey:  key,
		ApartmentNum: in.ApartmentNum,
		Rating:       in.Rating,
		Pros:         pros,
		Cons:         cons,
		Comment:      comment,
		CreatedAt:    svc.now(),
	})
}

// Buildings возвращает сводки по всем домам, у которых есть отзывы (для карты
// и списка на главной).
func (svc *Service) Buildings() ([]model.BuildingSummary, error) {
	return svc.store.Buildings()
}

// Building возвращает сводку по дому, список квартир с отзывами и ленту
// отзывов. Адрес для отображения берётся из аргумента (дом может быть ещё не
// сохранён, если по нему пока нет отзывов).
func (svc *Service) Building(addr model.Address) (model.BuildingSummary, []model.ApartmentSummary, []model.Review, error) {
	clean, err := cleanAddress(addr)
	if err != nil {
		return model.BuildingSummary{}, nil, nil, err
	}
	key := clean.Key()

	reviews, err := svc.store.ListByBuilding(key)
	if err != nil {
		return model.BuildingSummary{}, nil, nil, err
	}
	apartments, err := svc.store.ApartmentsByBuilding(key)
	if err != nil {
		return model.BuildingSummary{}, nil, nil, err
	}

	summary := model.BuildingSummary{
		Building:    model.Building{Key: key, Address: clean},
		ReviewCount: len(reviews),
	}
	if len(reviews) > 0 {
		var sum int
		for _, r := range reviews {
			sum += r.Rating
		}
		summary.AvgRating = float64(sum) / float64(len(reviews))
	}
	return summary, apartments, reviews, nil
}

// cleanAddress обрезает пробелы у частей адреса и проверяет их заполненность и
// длину. Координаты не нормализуются (проверяются отдельно при создании отзыва).
func cleanAddress(a model.Address) (model.Address, error) {
	a.City = strings.TrimSpace(a.City)
	a.Street = strings.TrimSpace(a.Street)
	a.House = strings.TrimSpace(a.House)
	if a.City == "" || a.Street == "" || a.House == "" {
		return model.Address{}, ErrInvalidAddress
	}
	for _, f := range []string{a.City, a.Street, a.House} {
		if len([]rune(f)) > MaxAddrFieldLen {
			return model.Address{}, ErrInvalidAddress
		}
	}
	return a, nil
}

// validCoords проверяет, что координаты в допустимом диапазоне и не нулевые
// (нулевые означают, что дом на карте не выбран).
func validCoords(lat, lon float64) bool {
	if lat == 0 && lon == 0 {
		return false
	}
	return lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

// normalizeTags убирает дубликаты, сохраняет порядок и проверяет допустимость.
func normalizeTags(tags []model.Tag, valid func(model.Tag) bool) ([]model.Tag, error) {
	seen := map[model.Tag]struct{}{}
	out := make([]model.Tag, 0, len(tags))
	for _, t := range tags {
		if !valid(t) {
			return nil, fmt.Errorf("%w: %q", ErrUnknownTag, t)
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out, nil
}
