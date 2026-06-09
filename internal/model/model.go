// Package model описывает доменные сущности приложения MRG —
// анонимные отзывы и оценки жильцов многоквартирного дома по квартирам.
package model

import (
	"strings"
	"time"
)

// MinRating и MaxRating ограничивают диапазон оценки в звёздах.
const (
	MinRating = 1
	MaxRating = 5
)

// Tag — заранее заданная отметка «что понравилось / не понравилось».
// Использование фиксированного набора тегов упрощает агрегацию и UI.
type Tag string

// Положительные теги (что понравилось в соседях).
const (
	TagQuiet     Tag = "тихие"
	TagClean     Tag = "чистоплотные"
	TagFriendly  Tag = "дружелюбные"
	TagHelpful   Tag = "отзывчивые"
	TagPetA      Tag = "любят животных"
	TagRespectTo Tag = "уважают тишину"
)

// Отрицательные теги (что не понравилось).
const (
	TagNoisy    Tag = "шумные"
	TagMessy    Tag = "мусорят"
	TagConflict Tag = "конфликтные"
	TagSmoking  Tag = "курят на лестнице"
	TagParking  Tag = "занимают чужую парковку"
)

// prosTags и consTags — допустимые множества тегов для каждой категории.
var (
	prosTags = map[Tag]struct{}{
		TagQuiet: {}, TagClean: {}, TagFriendly: {},
		TagHelpful: {}, TagPetA: {}, TagRespectTo: {},
	}
	consTags = map[Tag]struct{}{
		TagNoisy: {}, TagMessy: {}, TagConflict: {},
		TagSmoking: {}, TagParking: {},
	}
)

// AllProsTags возвращает все допустимые положительные теги в стабильном порядке.
func AllProsTags() []Tag {
	return []Tag{TagQuiet, TagClean, TagFriendly, TagHelpful, TagPetA, TagRespectTo}
}

// AllConsTags возвращает все допустимые отрицательные теги в стабильном порядке.
func AllConsTags() []Tag {
	return []Tag{TagNoisy, TagMessy, TagConflict, TagSmoking, TagParking}
}

// IsValidPro сообщает, входит ли тег в множество положительных.
func IsValidPro(t Tag) bool {
	_, ok := prosTags[t]
	return ok
}

// IsValidCon сообщает, входит ли тег в множество отрицательных.
func IsValidCon(t Tag) bool {
	_, ok := consTags[t]
	return ok
}

// Address описывает конкретный дом во всероссийском масштабе: город, улица,
// номер дома и географические координаты (выбираются на карте/в поиске).
type Address struct {
	City   string
	Street string
	House  string
	Lat    float64
	Lon    float64
}

// normalize приводит часть адреса к каноничному виду для построения ключа:
// нижний регистр, обрезка и схлопывание пробелов.
func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// Key возвращает стабильный идентификатор дома, не зависящий от регистра и
// лишних пробелов. Координаты в ключ не входят — дом определяется адресом.
func (a Address) Key() string {
	return normalize(a.City) + "|" + normalize(a.Street) + "|" + normalize(a.House)
}

// Display форматирует адрес для отображения, например «Москва, Тверская, д. 1».
func (a Address) Display() string {
	return a.City + ", " + a.Street + ", д. " + a.House
}

// Building — дом, к которому привязаны отзывы по квартирам.
type Building struct {
	Key     string
	Address Address
}

// BuildingSummary — агрегированная карточка дома для главной/карты.
type BuildingSummary struct {
	Building    Building
	ReviewCount int
	AvgRating   float64
}

// Review — один анонимный отзыв на квартиру в конкретном доме.
type Review struct {
	ID           int64
	BuildingKey  string
	ApartmentNum int
	Rating       int
	Pros         []Tag
	Cons         []Tag
	Comment      string
	CreatedAt    time.Time
}

// ApartmentSummary — агрегированная карточка квартиры внутри дома.
type ApartmentSummary struct {
	Number      int
	ReviewCount int
	AvgRating   float64
}
