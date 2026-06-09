package web

import (
	"encoding/json"
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

// --- Вспомогательное: фейковое падающее хранилище для проверки ошибок 500 ---

type errFakeType string

func (e errFakeType) Error() string { return string(e) }

var errFake = errFakeType("сбой хранилища")

type failingStore struct{}

func (failingStore) SaveBuilding(model.Building) error { return errFake }
func (failingStore) Building(string) (model.Building, bool, error) {
	return model.Building{}, false, errFake
}
func (failingStore) AddReview(model.Review) (model.Review, error) { return model.Review{}, errFake }
func (failingStore) ListByApartment(string, int) ([]model.Review, error) {
	return nil, errFake
}
func (failingStore) ListByBuilding(string) ([]model.Review, error) { return nil, errFake }
func (failingStore) ApartmentsByBuilding(string) ([]model.ApartmentSummary, error) {
	return nil, errFake
}
func (failingStore) Buildings() ([]model.BuildingSummary, error) { return nil, errFake }

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

func newFailingServer(t *testing.T) http.Handler {
	t.Helper()
	srv, err := NewServer(service.New(failingStore{}, nil))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv.Routes()
}

// validAddrValues — корректные параметры адреса для query/form.
func validAddrValues() url.Values {
	return url.Values{
		"city":   {"Москва"},
		"street": {"Тверская"},
		"house":  {"7"},
		"lat":    {"55.760000"},
		"lon":    {"37.600000"},
	}
}

func seedReview(t *testing.T, svc *service.Service, apartment, rating int) {
	t.Helper()
	_, err := svc.AddReview(service.NewReview{
		Address:      model.Address{City: "Москва", Street: "Тверская", House: "7", Lat: 55.76, Lon: 37.6},
		ApartmentNum: apartment,
		Rating:       rating,
	})
	if err != nil {
		t.Fatalf("seed AddReview: %v", err)
	}
}

// --- Главная ---

func TestIndexEmpty(t *testing.T) {
	h, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("статус = %d, ожидалось 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Пока ни одного отзыва") {
		t.Error("на пустой главной должно быть пустое состояние")
	}
	if !strings.Contains(body, `id="map"`) {
		t.Error("на главной должна быть карта")
	}
}

func TestIndexListsBuildings(t *testing.T) {
	h, svc := newTestServer(t)
	seedReview(t, svc, 42, 5)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	body := rec.Body.String()
	if !strings.Contains(body, "Москва, Тверская, д. 7") {
		t.Error("главная должна показывать адрес дома")
	}
	if !strings.Contains(body, "/building?") {
		t.Error("карточка должна ссылаться на страницу дома")
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

// --- JSON для карты ---

func TestBuildingsAPIReturnsMarkers(t *testing.T) {
	h, svc := newTestServer(t)
	seedReview(t, svc, 1, 4)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/buildings", nil))

	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, ожидался JSON", ct)
	}
	var markers []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &markers); err != nil {
		t.Fatalf("невалидный JSON: %v", err)
	}
	if len(markers) != 1 {
		t.Fatalf("ожидался 1 маркер, получено %d", len(markers))
	}
	if markers[0]["lat"].(float64) == 0 {
		t.Error("у маркера должна быть широта")
	}
	if !strings.HasPrefix(markers[0]["url"].(string), "/building?") {
		t.Error("у маркера должна быть ссылка на дом")
	}
}

func TestBuildingsAPIStoreErrorIs500(t *testing.T) {
	h := newFailingServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/buildings", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("статус = %d, ожидалось 500", rec.Code)
	}
}

// --- Страница дома ---

func TestBuildingPageRenders(t *testing.T) {
	h, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/building?"+validAddrValues().Encode(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("статус = %d, ожидалось 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Москва, Тверская, д. 7") {
		t.Error("должен отображаться адрес дома")
	}
	if !strings.Contains(body, string(model.TagQuiet)) {
		t.Error("в форме должны быть положительные теги")
	}
	if !strings.Contains(body, `id="bmap"`) {
		t.Error("на странице дома должна быть мини-карта")
	}
}

func TestBuildingPageShowsReviews(t *testing.T) {
	h, svc := newTestServer(t)
	seedReview(t, svc, 42, 5)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/building?"+validAddrValues().Encode(), nil))

	body := rec.Body.String()
	if !strings.Contains(body, "кв. 42") {
		t.Error("должен отображаться номер квартиры из отзыва")
	}
}

func TestBuildingPageNoAddressRedirectsHome(t *testing.T) {
	h, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	// Без обязательных частей адреса — редирект на главную.
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/building?city=Москва", nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("статус = %d, ожидалось 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, ожидалось /", loc)
	}
}

func TestBuildingPageStoreErrorIs500(t *testing.T) {
	h := newFailingServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/building?"+validAddrValues().Encode(), nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("статус = %d, ожидалось 500", rec.Code)
	}
}

// --- Отправка отзыва ---

func postReview(t *testing.T, h http.Handler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/review", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	return rec
}

func TestAddReviewSuccessRedirects(t *testing.T) {
	h, svc := newTestServer(t)
	form := validAddrValues()
	form.Set("apartment", "15")
	form.Set("rating", "4")
	form["pros"] = []string{string(model.TagQuiet), string(model.TagClean)}
	form.Set("comment", "тихие соседи")

	rec := postReview(t, h, form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("статус = %d, ожидалось 303 (PRG)", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/building?") {
		t.Errorf("Location = %q, ожидался переход на дом", loc)
	}

	summary, _, _, _ := svc.Building(model.Address{City: "Москва", Street: "Тверская", House: "7", Lat: 55.76, Lon: 37.6})
	if summary.ReviewCount != 1 {
		t.Errorf("ожидался 1 сохранённый отзыв, получено %d", summary.ReviewCount)
	}
}

func TestAddReviewInvalidRatingShows400(t *testing.T) {
	h, _ := newTestServer(t)
	form := validAddrValues()
	form.Set("apartment", "15")
	form.Set("rating", "9") // вне диапазона

	rec := postReview(t, h, form)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("статус = %d, ожидалось 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "оценка") {
		t.Error("на странице должно быть сообщение об ошибке валидации")
	}
}

func TestAddReviewMissingCoordsShows400(t *testing.T) {
	h, _ := newTestServer(t)
	form := validAddrValues()
	form.Set("lat", "0")
	form.Set("lon", "0")
	form.Set("apartment", "15")
	form.Set("rating", "4")

	rec := postReview(t, h, form)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("статус = %d, ожидалось 400", rec.Code)
	}
}

func TestAddReviewNoAddressRedirectsHome(t *testing.T) {
	h, _ := newTestServer(t)
	form := url.Values{"apartment": {"15"}, "rating": {"4"}} // адрес не задан
	rec := postReview(t, h, form)
	// Сервис вернёт ErrInvalidAddress → renderBuilding редиректит на главную.
	if rec.Code != http.StatusSeeOther {
		t.Errorf("статус = %d, ожидалось 303", rec.Code)
	}
}

func TestAddReviewStoreErrorIs500(t *testing.T) {
	h := newFailingServer(t)
	form := validAddrValues()
	form.Set("apartment", "3")
	form.Set("rating", "3")
	rec := postReview(t, h, form)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("статус = %d, ожидалось 500", rec.Code)
	}
}

// --- Прочее ---

func TestMethodNotAllowedOnIndexPost(t *testing.T) {
	h, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST / : статус = %d, ожидалось 405", rec.Code)
	}
}

func TestStaticAssetsServed(t *testing.T) {
	h, _ := newTestServer(t)
	for _, path := range []string{"/static/style.css", "/static/app.js"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: статус = %d, ожидалось 200", path, rec.Code)
		}
	}
}

func TestBuildingURLHelper(t *testing.T) {
	got := buildingURL(model.Address{City: "Москва", Street: "Тверская", House: "7", Lat: 55.76, Lon: 37.6})
	for _, want := range []string{"city=", "street=", "house=", "lat=55.760000", "lon=37.600000"} {
		if !strings.Contains(got, want) {
			t.Errorf("buildingURL = %q, не содержит %q", got, want)
		}
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
		{-1, "☆☆☆☆☆"},
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

func TestCoordHelper(t *testing.T) {
	if got := coord(55.7558); got != "55.755800" {
		t.Errorf("coord = %q", got)
	}
}
