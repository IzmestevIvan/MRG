// Package service содержит бизнес-логику MRG: валидацию входных данных
// отзыва, нормализацию тегов и агрегацию оценок. Слой не зависит от HTTP.
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
)

// Ошибки валидации, которые слой web может отображать пользователю.
var (
	ErrInvalidApartment = errors.New("номер квартиры должен быть от 1 до 10000")
	ErrInvalidRating    = errors.New("оценка должна быть от 1 до 5 звёзд")
	ErrUnknownTag       = errors.New("неизвестный тег")
	ErrCommentTooLong   = errors.New("комментарий слишком длинный")
)

// NewReview — входные данные для создания отзыва (без ID и времени).
type NewReview struct {
	ApartmentNum int
	Rating       int
	Pros         []model.Tag
	Cons         []model.Tag
	Comment      string
}

// Clock возвращает текущее время. Вынесен в интерфейс ради детерминированных тестов.
type Clock func() time.Time

// Service инкапсулирует операции над отзывами.
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

// AddReview валидирует и сохраняет новый отзыв.
func (svc *Service) AddReview(in NewReview) (model.Review, error) {
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

	return svc.store.AddReview(model.Review{
		ApartmentNum: in.ApartmentNum,
		Rating:       in.Rating,
		Pros:         pros,
		Cons:         cons,
		Comment:      comment,
		CreatedAt:    svc.now(),
	})
}

// Apartments возвращает сводки по всем квартирам с отзывами.
func (svc *Service) Apartments() ([]model.ApartmentSummary, error) {
	return svc.store.Apartments()
}

// Apartment возвращает сводку и отзывы по конкретной квартире. Если отзывов
// нет, сводка содержит нулевые показатели (квартиру всё равно можно открыть).
func (svc *Service) Apartment(num int) (model.ApartmentSummary, []model.Review, error) {
	if num < 1 || num > MaxApartmentNum {
		return model.ApartmentSummary{}, nil, ErrInvalidApartment
	}
	reviews, err := svc.store.ListByApartment(num)
	if err != nil {
		return model.ApartmentSummary{}, nil, err
	}

	summary := model.ApartmentSummary{Number: num, ReviewCount: len(reviews)}
	if len(reviews) > 0 {
		var sum int
		for _, r := range reviews {
			sum += r.Rating
		}
		summary.AvgRating = float64(sum) / float64(len(reviews))
	}
	return summary, reviews, nil
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
