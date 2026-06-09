package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/izmestevivan/mrg/internal/model"
	"github.com/izmestevivan/mrg/internal/store"
)

func fixedClock(t time.Time) Clock {
	return func() time.Time { return t }
}

func newService() *Service {
	return New(store.NewMemoryStore(), fixedClock(time.Unix(0, 0).UTC()))
}

// validAddr — корректный адрес с координатами для использования в тестах.
func validAddr() model.Address {
	return model.Address{City: "Москва", Street: "Тверская", House: "7", Lat: 55.76, Lon: 37.6}
}

func validReview() NewReview {
	return NewReview{Address: validAddr(), ApartmentNum: 7, Rating: 5}
}

func TestAddReviewValid(t *testing.T) {
	svc := newService()
	in := validReview()
	in.Pros = []model.Tag{model.TagQuiet, model.TagFriendly}
	in.Comment = "  хорошие соседи  "

	got, err := svc.AddReview(in)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if got.ID == 0 {
		t.Error("ожидался присвоенный ID")
	}
	if got.BuildingKey != validAddr().Key() {
		t.Errorf("BuildingKey = %q", got.BuildingKey)
	}
	if got.Comment != "хорошие соседи" {
		t.Errorf("комментарий должен быть обрезан, получено %q", got.Comment)
	}
	if !got.CreatedAt.Equal(time.Unix(0, 0).UTC()) {
		t.Errorf("должно использоваться время из Clock, получено %v", got.CreatedAt)
	}
}

func TestAddReviewCreatesBuilding(t *testing.T) {
	st := store.NewMemoryStore()
	svc := New(st, fixedClock(time.Unix(0, 0).UTC()))

	if _, err := svc.AddReview(validReview()); err != nil {
		t.Fatalf("AddReview: %v", err)
	}
	if _, ok, _ := st.Building(validAddr().Key()); !ok {
		t.Error("дом должен быть создан при первом отзыве")
	}
}

func TestAddReviewValidation(t *testing.T) {
	noHouse := validAddr()
	noHouse.House = "   "
	zeroCoords := validAddr()
	zeroCoords.Lat, zeroCoords.Lon = 0, 0
	badCoords := validAddr()
	badCoords.Lat = 200

	cases := []struct {
		name    string
		mutate  func(*NewReview)
		wantErr error
	}{
		{"пустой дом", func(r *NewReview) { r.Address = noHouse }, ErrInvalidAddress},
		{"нет координат", func(r *NewReview) { r.Address = zeroCoords }, ErrInvalidCoords},
		{"координаты вне диапазона", func(r *NewReview) { r.Address = badCoords }, ErrInvalidCoords},
		{"квартира 0", func(r *NewReview) { r.ApartmentNum = 0 }, ErrInvalidApartment},
		{"квартира слишком большая", func(r *NewReview) { r.ApartmentNum = MaxApartmentNum + 1 }, ErrInvalidApartment},
		{"оценка 0", func(r *NewReview) { r.Rating = 0 }, ErrInvalidRating},
		{"оценка 6", func(r *NewReview) { r.Rating = 6 }, ErrInvalidRating},
		{"неизвестный pro-тег", func(r *NewReview) { r.Pros = []model.Tag{"чужой"} }, ErrUnknownTag},
		{"con в pros", func(r *NewReview) { r.Pros = []model.Tag{model.TagNoisy} }, ErrUnknownTag},
		{"неизвестный con-тег", func(r *NewReview) { r.Cons = []model.Tag{model.TagQuiet} }, ErrUnknownTag},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := newService()
			in := validReview()
			c.mutate(&in)
			_, err := svc.AddReview(in)
			if !errors.Is(err, c.wantErr) {
				t.Errorf("ожидалась ошибка %v, получено %v", c.wantErr, err)
			}
		})
	}
}

func TestAddReviewCommentTooLong(t *testing.T) {
	svc := newService()
	in := validReview()
	in.Comment = strings.Repeat("я", MaxCommentLen+1) // считаем руны, а не байты
	if _, err := svc.AddReview(in); !errors.Is(err, ErrCommentTooLong) {
		t.Errorf("ожидалась ErrCommentTooLong, получено %v", err)
	}
}

func TestAddReviewCommentAtLimitOK(t *testing.T) {
	svc := newService()
	in := validReview()
	in.Comment = strings.Repeat("я", MaxCommentLen)
	if _, err := svc.AddReview(in); err != nil {
		t.Errorf("комментарий ровно на пределе должен проходить, ошибка: %v", err)
	}
}

func TestAddReviewDeduplicatesTags(t *testing.T) {
	svc := newService()
	in := validReview()
	in.Pros = []model.Tag{model.TagQuiet, model.TagQuiet, model.TagClean}

	got, err := svc.AddReview(in)
	if err != nil {
		t.Fatalf("ошибка: %v", err)
	}
	if len(got.Pros) != 2 {
		t.Errorf("дубликаты тегов не удалены: %v", got.Pros)
	}
	if got.Pros[0] != model.TagQuiet || got.Pros[1] != model.TagClean {
		t.Errorf("порядок тегов нарушен: %v", got.Pros)
	}
}

func TestBuildingAggregation(t *testing.T) {
	svc := newService()
	a := validAddr()
	in := NewReview{Address: a, ApartmentNum: 2, Rating: 5}
	if _, err := svc.AddReview(in); err != nil {
		t.Fatalf("ошибка: %v", err)
	}
	in.Rating = 2
	_, _ = svc.AddReview(in)

	summary, apartments, reviews, err := svc.Building(a)
	if err != nil {
		t.Fatalf("Building: %v", err)
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
	if len(apartments) != 1 || apartments[0].Number != 2 {
		t.Errorf("неверные сводки по квартирам: %+v", apartments)
	}
}

func TestBuildingEmptyHasZeroAvg(t *testing.T) {
	svc := newService()
	summary, _, reviews, err := svc.Building(validAddr())
	if err != nil {
		t.Fatalf("Building: %v", err)
	}
	if summary.ReviewCount != 0 || summary.AvgRating != 0 {
		t.Errorf("дом без отзывов должен иметь нулевые показатели: %+v", summary)
	}
	if len(reviews) != 0 {
		t.Errorf("ожидался пустой список отзывов")
	}
	// Адрес должен сохраниться для отображения, даже если дом ещё не в хранилище.
	if summary.Building.Address.City != "Москва" {
		t.Errorf("адрес для отображения потерян: %+v", summary.Building.Address)
	}
}

func TestBuildingInvalidAddress(t *testing.T) {
	svc := newService()
	bad := model.Address{City: "", Street: "Тверская", House: "1"}
	if _, _, _, err := svc.Building(bad); !errors.Is(err, ErrInvalidAddress) {
		t.Errorf("ожидалась ErrInvalidAddress, получено %v", err)
	}
}

func TestBuildingsListsAllWithReviews(t *testing.T) {
	svc := newService()
	a1 := model.Address{City: "Москва", Street: "Тверская", House: "1", Lat: 55.7, Lon: 37.6}
	a2 := model.Address{City: "Казань", Street: "Баумана", House: "5", Lat: 55.8, Lon: 49.1}
	_, _ = svc.AddReview(NewReview{Address: a1, ApartmentNum: 1, Rating: 4})
	_, _ = svc.AddReview(NewReview{Address: a2, ApartmentNum: 1, Rating: 2})

	got, err := svc.Buildings()
	if err != nil {
		t.Fatalf("Buildings: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 дома, получено %d", len(got))
	}
}

func TestNewDefaultsToTimeNow(t *testing.T) {
	svc := New(store.NewMemoryStore(), nil)
	before := time.Now()
	got, err := svc.AddReview(validReview())
	if err != nil {
		t.Fatalf("ошибка: %v", err)
	}
	if got.CreatedAt.Before(before) {
		t.Errorf("по умолчанию должно использоваться time.Now, получено %v", got.CreatedAt)
	}
}
