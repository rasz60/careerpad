package domain

import (
	"context"
	"time"
)

// RoadmapPhase 는 로드맵의 구간이다. 항목은 달(due_month)이 From ~ To 안에 있으면 이 구간에 속한다.
type RoadmapPhase struct {
	Title string `json:"title"`
	From  string `json:"from"` // YYYY-MM
	To    string `json:"to"`   // YYYY-MM
	Goal  string `json:"goal"`
}

// Roadmap 은 기간을 정해 두고 달마다 할 일을 체크해 가는 계획표 한 장이다.
type Roadmap struct {
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	Lead       string         `json:"lead"`
	StartsOn   string         `json:"starts_on"` // YYYY-MM-DD
	EndsOn     string         `json:"ends_on"`   // YYYY-MM-DD
	Categories []string       `json:"categories"`
	Phases     []RoadmapPhase `json:"phases"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// RoadmapItem 은 그 달까지 할 일 하나다. DoneOn 이 비면 아직이다.
type RoadmapItem struct {
	ID        string `json:"id"`
	Month     string `json:"month"` // YYYY-MM
	Category  string `json:"category"`
	Body      string `json:"body"`
	IsKey     bool   `json:"is_key"`
	DoneOn    string `json:"done_on"` // YYYY-MM-DD, 비면 미완료
	SortOrder int    `json:"sort_order"`
}

// RoadmapHabit 은 매주 할 일이다. Weeks 는 체크한 ISO 주(YYYY-Www)다.
type RoadmapHabit struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Weeks     []string `json:"weeks"`
	SortOrder int      `json:"sort_order"`
}

// RoadmapDetail 은 로드맵 한 장의 전부다.
type RoadmapDetail struct {
	Roadmap
	Items  []RoadmapItem  `json:"items"`
	Habits []RoadmapHabit `json:"habits"`
}

// RoadmapSummary 는 목록에 보일 로드맵과 항목 집계다.
type RoadmapSummary struct {
	Roadmap
	Total   int `json:"total"`
	Done    int `json:"done"`
	Overdue int `json:"overdue"` // 이번 달보다 앞선 달인데 아직인 항목
}

// RoadmapRepository 는 career.roadmaps · roadmap_items · roadmap_habits 를 다룬다.
// 항목 · 습관도 언제나 로드맵의 주인(userID)으로 거른다. 남의 것이면 ErrNotFound.
type RoadmapRepository interface {
	// List 는 로드맵마다 항목 집계를 붙인다. curMonth(YYYY-MM)보다 앞선 미완료가 Overdue.
	List(ctx context.Context, userID, curMonth string) ([]RoadmapSummary, error)
	Find(ctx context.Context, userID, id string) (*RoadmapDetail, error)
	// Create 는 로드맵 · 항목 · 습관을 한 트랜잭션으로 넣고 d 를 저장된 값으로 바꾼다.
	Create(ctx context.Context, userID string, d *RoadmapDetail) error
	Update(ctx context.Context, userID string, r *Roadmap) error
	Delete(ctx context.Context, userID, id string) error

	FindItem(ctx context.Context, userID, roadmapID, itemID string) (*RoadmapItem, error)
	CreateItem(ctx context.Context, userID, roadmapID string, it *RoadmapItem) error
	UpdateItem(ctx context.Context, userID, roadmapID string, it *RoadmapItem) error
	DeleteItem(ctx context.Context, userID, roadmapID, itemID string) error

	CreateHabit(ctx context.Context, userID, roadmapID string, h *RoadmapHabit) error
	DeleteHabit(ctx context.Context, userID, roadmapID, habitID string) error
	// SetHabitWeek 는 그 주를 체크(on)하거나 지운다.
	SetHabitWeek(ctx context.Context, userID, roadmapID, habitID, week string, on bool) (*RoadmapHabit, error)
}
