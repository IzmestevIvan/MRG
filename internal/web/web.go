// Package web реализует HTTP-слой MRG: серверный рендеринг страниц через
// html/template, карту домов (Leaflet/OpenStreetMap на фронтенде) и обработку
// формы отправки отзыва.
package web

import (
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"

	"github.com/izmestevivan/mrg/internal/model"
	"github.com/izmestevivan/mrg/internal/service"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// pages перечисляет страницы приложения. Каждая собирается в собственный
// набор шаблонов вместе с base.html, чтобы блоки "content"/"title" разных
// страниц не пересекались в общем пространстве имён html/template.
var pages = []string{"welcome.html", "map.html", "building.html"}

// Server связывает сервис и HTTP-маршруты.
type Server struct {
	svc   *service.Service
	tmpls map[string]*template.Template
}

// NewServer парсит шаблоны и собирает обработчик. Возвращает ошибку, если
// шаблоны не удалось разобрать (нештатная ситуация на старте).
func NewServer(svc *service.Service) (*Server, error) {
	funcs := template.FuncMap{
		"stars":       stars,
		"roundi":      roundi,
		"ratingsDesc": ratingsDesc,
		"coord":       coord,
		"buildingURL": buildingURL,
	}
	tmpls := make(map[string]*template.Template, len(pages))
	for _, p := range pages {
		t, err := template.New(p).Funcs(funcs).
			ParseFS(templatesFS, "templates/base.html", "templates/"+p)
		if err != nil {
			return nil, err
		}
		tmpls[p] = t
	}
	return &Server{svc: svc, tmpls: tmpls}, nil
}

// Routes возвращает http.Handler со всеми маршрутами приложения.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleWelcome)
	mux.HandleFunc("GET /map", s.handleMap)
	mux.HandleFunc("GET /api/buildings", s.handleBuildingsAPI)
	mux.HandleFunc("GET /building", s.handleBuilding)
	mux.HandleFunc("POST /review", s.handleAddReview)
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))
	return mux
}

// handleWelcome отдаёт приветственную страницу с вводной информацией.
func (s *Server) handleWelcome(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "welcome.html", nil)
}

// handleMap отдаёт страницу с картой и поиском адреса. Маркеры домов
// подгружаются на фронтенде через /api/buildings.
func (s *Server) handleMap(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "map.html", nil)
}

// buildingMarker — компактное представление дома для карты на главной.
type buildingMarker struct {
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Avg     float64 `json:"avg"`
	Reviews int     `json:"reviews"`
}

func (s *Server) handleBuildingsAPI(w http.ResponseWriter, r *http.Request) {
	buildings, err := s.svc.Buildings()
	if err != nil {
		s.serverError(w, err)
		return
	}
	markers := make([]buildingMarker, 0, len(buildings))
	for _, b := range buildings {
		markers = append(markers, buildingMarker{
			Lat:     b.Building.Address.Lat,
			Lon:     b.Building.Address.Lon,
			Title:   b.Building.Address.Display(),
			URL:     buildingURL(b.Building.Address),
			Avg:     b.AvgRating,
			Reviews: b.ReviewCount,
		})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(markers)
}

type buildingData struct {
	Address    model.Address
	Summary    model.BuildingSummary
	Apartments []model.ApartmentSummary
	Reviews    []model.Review
	ProsTags   []model.Tag
	ConsTags   []model.Tag
	Error      string
}

func (s *Server) handleBuilding(w http.ResponseWriter, r *http.Request) {
	addr := addressFromValues(r.URL.Query())
	s.renderBuilding(w, r, addr, http.StatusOK, "")
}

func (s *Server) handleAddReview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "некорректная форма", http.StatusBadRequest)
		return
	}
	addr := addressFromValues(r.Form)
	rating, _ := strconv.Atoi(r.FormValue("rating"))
	apartment, _ := strconv.Atoi(r.FormValue("apartment"))

	in := service.NewReview{
		Address:      addr,
		ApartmentNum: apartment,
		Rating:       rating,
		Pros:         toTags(r.Form["pros"]),
		Cons:         toTags(r.Form["cons"]),
		Comment:      r.FormValue("comment"),
	}

	if _, err := s.svc.AddReview(in); err != nil {
		if isValidationError(err) {
			s.renderBuilding(w, r, addr, http.StatusBadRequest, err.Error())
			return
		}
		s.serverError(w, err)
		return
	}

	// POST-Redirect-GET: возвращаемся на страницу дома.
	http.Redirect(w, r, buildingURL(addr), http.StatusSeeOther)
}

func (s *Server) renderBuilding(w http.ResponseWriter, r *http.Request, addr model.Address, status int, errMsg string) {
	summary, apartments, reviews, err := s.svc.Building(addr)
	if err != nil {
		if errors.Is(err, service.ErrInvalidAddress) {
			// Адрес не выбран — отправляем на главную к карте.
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.serverError(w, err)
		return
	}
	s.render(w, status, "building.html", buildingData{
		Address:    summary.Building.Address,
		Summary:    summary,
		Apartments: apartments,
		Reviews:    reviews,
		ProsTags:   model.AllProsTags(),
		ConsTags:   model.AllConsTags(),
		Error:      errMsg,
	})
}

func (s *Server) render(w http.ResponseWriter, status int, name string, data any) {
	t, ok := s.tmpls[name]
	if !ok {
		s.serverError(w, nil)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	// Ответ уже начат — на ошибку рендера статус сменить нельзя, поэтому
	// шаблоны проверяются на старте в NewServer, а здесь ошибку игнорируем.
	_ = t.ExecuteTemplate(w, "base", data)
}

func (s *Server) serverError(w http.ResponseWriter, _ error) {
	http.Error(w, "внутренняя ошибка сервера", http.StatusInternalServerError)
}

// addressFromValues извлекает адрес из query- или form-параметров.
func addressFromValues(v url.Values) model.Address {
	lat, _ := strconv.ParseFloat(v.Get("lat"), 64)
	lon, _ := strconv.ParseFloat(v.Get("lon"), 64)
	return model.Address{
		City:   v.Get("city"),
		Street: v.Get("street"),
		House:  v.Get("house"),
		Lat:    lat,
		Lon:    lon,
	}
}

// buildingURL строит ссылку на страницу дома с адресом в query-параметрах.
func buildingURL(a model.Address) string {
	v := url.Values{}
	v.Set("city", a.City)
	v.Set("street", a.Street)
	v.Set("house", a.House)
	v.Set("lat", coord(a.Lat))
	v.Set("lon", coord(a.Lon))
	return "/building?" + v.Encode()
}

func toTags(values []string) []model.Tag {
	out := make([]model.Tag, 0, len(values))
	for _, v := range values {
		out = append(out, model.Tag(v))
	}
	return out
}

func isValidationError(err error) bool {
	return errors.Is(err, service.ErrInvalidAddress) ||
		errors.Is(err, service.ErrInvalidCoords) ||
		errors.Is(err, service.ErrInvalidApartment) ||
		errors.Is(err, service.ErrInvalidRating) ||
		errors.Is(err, service.ErrUnknownTag) ||
		errors.Is(err, service.ErrCommentTooLong)
}

// coord форматирует координату для URL/шаблона с фиксированной точностью.
func coord(f float64) string {
	return strconv.FormatFloat(f, 'f', 6, 64)
}

// ratingsDesc возвращает варианты оценки от 5 до 1 — порядок важен для
// CSS-приёма «звёзды по наведению» (radio-кнопки идут по убыванию).
func ratingsDesc() []int {
	return []int{5, 4, 3, 2, 1}
}

// roundi округляет среднюю оценку до ближайшего целого для отрисовки звёзд.
func roundi(f float64) int {
	return int(f + 0.5)
}

// stars формирует строку из закрашенных и пустых звёзд для шаблона.
func stars(rating int) string {
	if rating < 0 {
		rating = 0
	}
	if rating > model.MaxRating {
		rating = model.MaxRating
	}
	out := make([]rune, 0, model.MaxRating)
	for i := 0; i < rating; i++ {
		out = append(out, '★')
	}
	for i := rating; i < model.MaxRating; i++ {
		out = append(out, '☆')
	}
	return string(out)
}
