package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/izmestevivan/mrg/internal/model"
	"github.com/izmestevivan/mrg/internal/store"
)

// fixedClock возвращает постоянное время для детерминированных тестов.
func fixedClock(t time.Time) Clock {
	return func() time.Time { return t }
}

func newService() *Service {
	return New(store.NewMemoryStore(), fixedClock(time.Unix(0, 0).UTC()))
}

func TestAddReviewValid(t *testing.T) {
	svc := newService()
	got, err := svc.AddReview(NewReview{
		ApartmentNum: 7,
		Rating:       5,
		Pros:         []model.Tag{model.TagQuiet, model.TagFriendly},
		Cons:         nil,
		Comment:      "  хорошие соседи  ",
	})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if got.ID == 0 {
		t.Error("ожидался присвоенный ID")
	}
	if got.Comment != "хорошие соседи" {
		t.Errorf("комментарий должен быть обрезан, получено %q", got.Comment)
	}
	if !got.CreatedAt.Equal(time.Unix(0, 0).UTC()) {
		t.Errorf("должно использоваться время из Clock, получено %v", got.CreatedAt)
	}
}

func TestAddReviewValidation(t *testing.T) {
	cases := []struct {
		name    string
		in      NewReview
		wantErr error
	}{
		{"квартира 0", NewReview{ApartmentNum: 0, Rating: 3}, ErrInvalidApartment},
		{"квартира слишком большая", NewReview{ApartmentNum: MaxApartmentNum + 1, Rating: 3}, ErrInvalidApartment},
		{"оценка 0", NewReview{ApartmentNum: 1, Rating: 0}, ErrInvalidRating},
		{"оценка 6", NewReview{ApartmentNum: 1, Rating: 6}, ErrInvalidRating},
		{"неизвестный pro-тег", NewReview{ApartmentNum: 1, Rating: 3, Pros: []model.Tag{"чужой"}}, ErrUnknownTag},
		{"con в pros", NewReview{ApartmentNum: 1, Rating: 3, Pros: []model.Tag{model.TagNoisy}}, ErrUnknownTag},
		{"неизвестный con-тег", NewReview{ApartmentNum: 1, Rating: 3, Cons: []model.Tag{model.TagQuiet}}, ErrUnknownTag},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := newService()
			_, err := svc.AddReview(c.in)
			if !errors.Is(err, c.wantErr) {
				t.Errorf("ожидалась ошибка %v, получено %v", c.wantErr, err)
			}
		})
	}
}

func TestAddReviewCommentTooLong(t *testing.T) {
	svc := newService()
	long := strings.Repeat("я", MaxCommentLen+1) // считаем руны, а не байты
	_, err := svc.AddReview(NewReview{ApartmentNum: 1, Rating: 3, Comment: long})
	if !errors.Is(err, ErrCommentTooLong) {
		t.Errorf("ожидалась ErrCommentTooLong, получено %v", err)
	}
}

func TestAddReviewCommentAtLimitOK(t *testing.T) {
	svc := newService()
	exact := strings.Repeat("я", MaxCommentLen)
	if _, err := svc.AddReview(NewReview{ApartmentNum: 1, Rating: 3, Comment: exact}); err != nil {
		t.Errorf("комментарий ровно на пределе должен проходить, ошибка: %v", err)
	}
}

func TestAddReviewDeduplicatesTags(t *testing.T) {
	svc := newService()
	got, err := svc.AddReview(NewReview{
		ApartmentNum: 1,
		Rating:       4,
		Pros:         []model.Tag{model.TagQuiet, model.TagQuiet, model.TagClean},
	})
	if err != nil {
		t.Fatalf("ошибка: %v", err)
	}
	if len(got.Pros) != 2 {
		t.Errorf("дубликаты тегов не удалены: %v", got.Pros)
	}
	// Порядок сохраняется: первым идёт TagQuiet.
	if got.Pros[0] != model.TagQuiet || got.Pros[1] != model.TagClean {
		t.Errorf("порядок тегов нарушен: %v", got.Pros)
	}
}

func TestApartmentAggregation(t *testing.T) {
	svc := newService()
	_, _ = svc.AddReview(NewReview{ApartmentNum: 2, Rating: 5})
	_, _ = svc.AddReview(NewReview{ApartmentNum: 2, Rating: 2})

	summary, reviews, err := svc.Apartment(2)
	if err != nil {
		t.Fatalf("Apartment: %v", err)
	}
	if summary.ReviewCount != 2 {
		t.Errorf("ReviewCount = %d, ожидалось 2", summary.ReviewCount)
	}
	if summary.AvgRating != 3.5 {
		t.Errorf("AvgRating = %v, ожидалось 3.5", summary.AvgRating)
	}
	if len(reviews) != 2 {
		t.Errorf("ожидалось 2 отзыва, получено %d", len(reviews))
	}
}

func TestApartmentEmptyHasZeroAvg(t *testing.T) {
	svc := newService()
	summary, reviews, err := svc.Apartment(99)
	if err != nil {
		t.Fatalf("Apartment: %v", err)
	}
	if summary.ReviewCount != 0 || summary.AvgRating != 0 {
		t.Errorf("пустая квартира должна иметь нулевые показатели: %+v", summary)
	}
	if len(reviews) != 0 {
		t.Errorf("ожидался пустой список отзывов")
	}
}

func TestApartmentInvalidNumber(t *testing.T) {
	svc := newService()
	if _, _, err := svc.Apartment(0); !errors.Is(err, ErrInvalidApartment) {
		t.Errorf("ожидалась ErrInvalidApartment, получено %v", err)
	}
}

func TestApartmentsListsAllWithReviews(t *testing.T) {
	svc := newService()
	_, _ = svc.AddReview(NewReview{ApartmentNum: 5, Rating: 4})
	_, _ = svc.AddReview(NewReview{ApartmentNum: 8, Rating: 2})

	got, err := svc.Apartments()
	if err != nil {
		t.Fatalf("Apartments: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 квартиры, получено %d", len(got))
	}
	if got[0].Number != 5 || got[1].Number != 8 {
		t.Errorf("неверный список квартир: %+v", got)
	}
}

func TestNewDefaultsToTimeNow(t *testing.T) {
	svc := New(store.NewMemoryStore(), nil)
	before := time.Now()
	got, err := svc.AddReview(NewReview{ApartmentNum: 1, Rating: 3})
	if err != nil {
		t.Fatalf("ошибка: %v", err)
	}
	if got.CreatedAt.Before(before) {
		t.Errorf("по умолчанию должно использоваться time.Now, получено %v", got.CreatedAt)
	}
}
