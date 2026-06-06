// Package model описывает доменные сущности приложения MRG —
// анонимные отзывы и оценки жильцов многоквартирного дома по квартирам.
package model

import "time"

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

// Review — один анонимный отзыв на квартиру.
type Review struct {
	ID           int64
	ApartmentNum int
	Rating       int
	Pros         []Tag
	Cons         []Tag
	Comment      string
	CreatedAt    time.Time
}

// ApartmentSummary — агрегированная карточка квартиры для списка/детальной страницы.
type ApartmentSummary struct {
	Number      int
	ReviewCount int
	AvgRating   float64
}
