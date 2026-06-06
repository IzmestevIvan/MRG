package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/izmestevivan/mrg/internal/model"
	"github.com/izmestevivan/mrg/internal/service"
	"github.com/izmestevivan/mrg/internal/store"
)

// failingStore реализует store.Store и всегда возвращает ошибку — для
// проверки ветвей обработки внутренних ошибок (HTTP 500).
type failingStore struct{}

func (failingStore) AddReview(r model.Review) (model.Review, error) {
	return model.Review{}, errFake
}
func (failingStore) ListByApartment(int) ([]model.Review, error) { return nil, errFake }
func (failingStore) Apartments() ([]model.ApartmentSummary, error) {
	return nil, errFake
}

var errFake = errFakeType("сбой хранилища")

type errFakeType string

func (e errFakeType) Error() string { return string(e) }

func newFailingServer(t *testing.T) http.Handler {
	t.Helper()
	svc := service.New(failingStore{}, nil)
	srv, err := NewServer(svc)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv.Routes()
}

func newTestServer(t *testing.T) (http.Handler, *service.Service) {
	t.Helper()
	svc := service.New(store.NewMemoryStore(), func() time.Time {
		return time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	})
	srv, err := NewServer(svc)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv.Routes(), svc
}

func TestIndexEmpty(t *testing.T) {
	h, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("статус = %d, ожидалось 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Пока ни одного отзыва") {
		t.Error("на пустой главной должно быть пустое состояние")
	}
}

func TestIndexListsApartments(t *testing.T) {
	h, svc := newTestServer(t)
	_, _ = svc.AddReview(service.NewReview{ApartmentNum: 42, Rating: 5})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	body := rec.Body.String()
	if !strings.Contains(body, "кв. 42") {
		t.Error("главная должна показывать квартиру 42")
	}
	if !strings.Contains(body, "/apartment/42") {
		t.Error("карточка должна ссылаться на страницу квартиры")
	}
}

func TestApartmentPageRenders(t *testing.T) {
	h, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apartment/7", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("статус = %d, ожидалось 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Квартира 7") {
		t.Error("должен отображаться заголовок квартиры")
	}
	// Форма с тегами должна присутствовать.
	if !strings.Contains(body, string(model.TagQuiet)) {
		t.Error("в форме должны быть положительные теги")
	}
}

func TestApartmentInvalidNumberIs404(t *testing.T) {
	h, _ := newTestServer(t)
	for _, path := range []string{"/apartment/0", "/apartment/abc", "/apartment/-3"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: статус = %d, ожидалось 404", path, rec.Code)
		}
	}
}

func TestApartmentNumberAboveLimitIs404(t *testing.T) {
	h, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	// Номер проходит парсинг пути, но отвергается сервисом как вне диапазона.
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apartment/999999", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("статус = %d, ожидалось 404", rec.Code)
	}
}

func TestAddReviewSuccessRedirects(t *testing.T) {
	h, svc := newTestServer(t)
	form := url.Values{
		"rating":  {"4"},
		"pros":    {string(model.TagQuiet), string(model.TagClean)},
		"comment": {"тихие соседи"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apartment/15/review", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("статус = %d, ожидалось 303 (PRG)", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/apartment/15" {
		t.Errorf("Location = %q, ожидалось /apartment/15", loc)
	}
	// Отзыв действительно сохранён.
	summary, _, _ := svc.Apartment(15)
	if summary.ReviewCount != 1 {
		t.Errorf("ожидался 1 сохранённый отзыв, получено %d", summary.ReviewCount)
	}
}

func TestAddReviewInvalidRatingShows400(t *testing.T) {
	h, _ := newTestServer(t)
	form := url.Values{"rating": {"9"}} // вне диапазона
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apartment/15/review", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("статус = %d, ожидалось 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "оценка") {
		t.Error("на странице должно быть сообщение об ошибке валидации")
	}
}

func TestAddReviewUnknownTagShows400(t *testing.T) {
	h, _ := newTestServer(t)
	form := url.Values{"rating": {"3"}, "pros": {"чужой-тег"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apartment/15/review", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("статус = %d, ожидалось 400", rec.Code)
	}
}

func TestMethodNotAllowedOnIndexPost(t *testing.T) {
	h, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST / : статус = %d, ожидалось 405", rec.Code)
	}
}

func TestStaticCSSServed(t *testing.T) {
	h, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/style.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус = %d, ожидалось 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "--brand") {
		t.Error("должен отдаваться CSS-файл")
	}
}

func TestIndexStoreErrorIs500(t *testing.T) {
	h := newFailingServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("статус = %d, ожидалось 500", rec.Code)
	}
}

func TestApartmentStoreErrorIs500(t *testing.T) {
	h := newFailingServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apartment/3", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("статус = %d, ожидалось 500", rec.Code)
	}
}

func TestAddReviewStoreErrorIs500(t *testing.T) {
	h := newFailingServer(t)
	form := url.Values{"rating": {"3"}} // данные валидны, падает само хранилище
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apartment/3/review", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("статус = %d, ожидалось 500", rec.Code)
	}
}

func TestStarsHelper(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "☆☆☆☆☆"},
		{3, "★★★☆☆"},
		{5, "★★★★★"},
		{-1, "☆☆☆☆☆"}, // защита от выхода за границы
		{99, "★★★★★"},
	}
	for _, c := range cases {
		if got := stars(c.in); got != c.want {
			t.Errorf("stars(%d) = %q, ожидалось %q", c.in, got, c.want)
		}
	}
}

func TestRoundiHelper(t *testing.T) {
	cases := map[float64]int{0: 0, 3.4: 3, 3.5: 4, 4.9: 5}
	for in, want := range cases {
		if got := roundi(in); got != want {
			t.Errorf("roundi(%v) = %d, ожидалось %d", in, got, want)
		}
	}
}
