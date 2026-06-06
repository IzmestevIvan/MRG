// Package web реализует HTTP-слой MRG: серверный рендеринг страниц через
// html/template и обработку формы отправки отзыва.
package web

import (
	"embed"
	"errors"
	"html/template"
	"net/http"
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
var pages = []string{"index.html", "apartment.html"}

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
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /apartment/{num}", s.handleApartment)
	mux.HandleFunc("POST /apartment/{num}/review", s.handleAddReview)
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))
	return mux
}

type indexData struct {
	Apartments []model.ApartmentSummary
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	apartments, err := s.svc.Apartments()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, http.StatusOK, "index.html", indexData{Apartments: apartments})
}

type apartmentData struct {
	Summary  model.ApartmentSummary
	Reviews  []model.Review
	ProsTags []model.Tag
	ConsTags []model.Tag
	Error    string
}

func (s *Server) handleApartment(w http.ResponseWriter, r *http.Request) {
	num, ok := parseApartmentNum(w, r)
	if !ok {
		return
	}
	s.renderApartment(w, r, num, http.StatusOK, "")
}

func (s *Server) handleAddReview(w http.ResponseWriter, r *http.Request) {
	num, ok := parseApartmentNum(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "некорректная форма", http.StatusBadRequest)
		return
	}

	rating, _ := strconv.Atoi(r.FormValue("rating"))
	in := service.NewReview{
		ApartmentNum: num,
		Rating:       rating,
		Pros:         toTags(r.Form["pros"]),
		Cons:         toTags(r.Form["cons"]),
		Comment:      r.FormValue("comment"),
	}

	if _, err := s.svc.AddReview(in); err != nil {
		// Ошибки валидации показываем на той же странице со статусом 400.
		if isValidationError(err) {
			s.renderApartment(w, r, num, http.StatusBadRequest, err.Error())
			return
		}
		s.serverError(w, err)
		return
	}

	// Перенаправляем (POST-Redirect-GET), чтобы избежать повторной отправки.
	http.Redirect(w, r, "/apartment/"+strconv.Itoa(num), http.StatusSeeOther)
}

func (s *Server) renderApartment(w http.ResponseWriter, r *http.Request, num, status int, errMsg string) {
	summary, reviews, err := s.svc.Apartment(num)
	if err != nil {
		if errors.Is(err, service.ErrInvalidApartment) {
			http.NotFound(w, r)
			return
		}
		s.serverError(w, err)
		return
	}
	s.render(w, status, "apartment.html", apartmentData{
		Summary:  summary,
		Reviews:  reviews,
		ProsTags: model.AllProsTags(),
		ConsTags: model.AllConsTags(),
		Error:    errMsg,
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

// parseApartmentNum извлекает и проверяет {num} из пути.
func parseApartmentNum(w http.ResponseWriter, r *http.Request) (int, bool) {
	num, err := strconv.Atoi(r.PathValue("num"))
	if err != nil || num < 1 {
		http.NotFound(w, r)
		return 0, false
	}
	return num, true
}

func toTags(values []string) []model.Tag {
	out := make([]model.Tag, 0, len(values))
	for _, v := range values {
		out = append(out, model.Tag(v))
	}
	return out
}

func isValidationError(err error) bool {
	return errors.Is(err, service.ErrInvalidApartment) ||
		errors.Is(err, service.ErrInvalidRating) ||
		errors.Is(err, service.ErrUnknownTag) ||
		errors.Is(err, service.ErrCommentTooLong)
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
