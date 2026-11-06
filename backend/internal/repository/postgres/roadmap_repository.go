package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/devsixt/careerpad/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RoadmapRepository 는 career.roadmaps · roadmap_items · roadmap_habits 를 다룬다.
// 항목 · 습관 쿼리는 모두 roadmaps.user_id 를 조인해 주인만 건드리게 한다.
//
// 날짜는 문자열(YYYY-MM-DD)로 주고받는다: 읽을 때 to_char, 쓸 때 ::text::date.
// 쿼리 실행 방식(cache_statement · exec · simple)에 상관없이 같은 값이 오가게 하려는 것이다.
type RoadmapRepository struct {
	pool *pgxpool.Pool
}

func NewRoadmapRepository(pool *pgxpool.Pool) *RoadmapRepository {
	return &RoadmapRepository{pool: pool}
}

var _ domain.RoadmapRepository = (*RoadmapRepository)(nil)

const roadmapColumns = `
	r.id, r.title, r.lead, to_char(r.starts_on, 'YYYY-MM-DD'), to_char(r.ends_on, 'YYYY-MM-DD'),
	r.categories, r.phases::text, r.created_at, r.updated_at`

const itemColumns = `
	i.id, i.due_month, i.category, i.body, i.is_key, COALESCE(to_char(i.done_on, 'YYYY-MM-DD'), ''), i.sort_order`

const habitColumns = `h.id, h.name, h.weeks, h.sort_order`

// List 는 idx_roadmaps_user_starts 순서(시작일 최신순)로 읽고 항목 수를 센다.
func (r *RoadmapRepository) List(ctx context.Context, userID, curMonth string) ([]domain.RoadmapSummary, error) {
	q := `
		SELECT ` + roadmapColumns + `,
		       count(i.id), count(i.done_on),
		       count(i.id) FILTER (WHERE i.done_on IS NULL AND i.due_month < $2)
		  FROM career.roadmaps r
		  LEFT JOIN career.roadmap_items i ON i.roadmap_id = r.id
		 WHERE r.user_id = $1
		 GROUP BY r.id
		 ORDER BY r.starts_on DESC, r.created_at DESC`
	rows, err := r.pool.Query(ctx, q, userID, curMonth)
	if err != nil {
		return nil, translate("roadmap repository: list", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.RoadmapSummary, error) {
		var s domain.RoadmapSummary
		var phases string
		if err := row.Scan(&s.ID, &s.Title, &s.Lead, &s.StartsOn, &s.EndsOn, &s.Categories, &phases,
			&s.CreatedAt, &s.UpdatedAt, &s.Total, &s.Done, &s.Overdue); err != nil {
			return s, err
		}
		return s, fillRoadmap(&s.Roadmap, phases)
	})
	if err != nil {
		return nil, translate("roadmap repository: list scan", err)
	}
	return out, nil
}

func (r *RoadmapRepository) Find(ctx context.Context, userID, id string) (*domain.RoadmapDetail, error) {
	q := `SELECT ` + roadmapColumns + ` FROM career.roadmaps r WHERE r.id = $1 AND r.user_id = $2`
	d := &domain.RoadmapDetail{}
	if err := scanRoadmap(r.pool.QueryRow(ctx, q, id, userID), &d.Roadmap); err != nil {
		return nil, translate("roadmap repository: find", err)
	}

	rows, err := r.pool.Query(ctx, `SELECT `+itemColumns+` FROM career.roadmap_items i
		WHERE i.roadmap_id = $1 ORDER BY i.due_month, i.sort_order, i.created_at`, d.ID)
	if err != nil {
		return nil, translate("roadmap repository: items", err)
	}
	if d.Items, err = pgx.CollectRows(rows, scanItemRow); err != nil {
		return nil, translate("roadmap repository: items scan", err)
	}

	rows, err = r.pool.Query(ctx, `SELECT `+habitColumns+` FROM career.roadmap_habits h
		WHERE h.roadmap_id = $1 ORDER BY h.sort_order, h.created_at`, d.ID)
	if err != nil {
		return nil, translate("roadmap repository: habits", err)
	}
	if d.Habits, err = pgx.CollectRows(rows, scanHabitRow); err != nil {
		return nil, translate("roadmap repository: habits scan", err)
	}
	return d, nil
}

// Create 는 로드맵 한 장을 한 트랜잭션으로 넣는다. 항목은 배열을 펼쳐(unnest) 한 문장으로 넣는다.
func (r *RoadmapRepository) Create(ctx context.Context, userID string, d *domain.RoadmapDetail) error {
	phases, err := json.Marshal(d.Phases)
	if err != nil {
		return fmt.Errorf("roadmap repository: marshal phases: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return translate("roadmap repository: begin", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := `
		INSERT INTO career.roadmaps AS r (user_id, title, lead, starts_on, ends_on, categories, phases)
		VALUES ($1, $2, $3, $4::text::date, $5::text::date, $6, $7::text::jsonb)
		RETURNING ` + roadmapColumns
	var saved domain.Roadmap
	if err := scanRoadmap(tx.QueryRow(ctx, q, userID, d.Title, d.Lead, d.StartsOn, d.EndsOn, d.Categories, string(phases)), &saved); err != nil {
		return translate("roadmap repository: create", err)
	}

	n := len(d.Items)
	months, cats, bodies, dones := make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	keys, orders := make([]bool, n), make([]int32, n)
	for i, it := range d.Items {
		months[i], cats[i], bodies[i], dones[i], keys[i], orders[i] = it.Month, it.Category, it.Body, it.DoneOn, it.IsKey, int32(it.SortOrder)
	}
	rows, err := tx.Query(ctx, `
		WITH ins AS (
			INSERT INTO career.roadmap_items (roadmap_id, due_month, category, body, is_key, done_on, sort_order)
			SELECT $1, m, c, b, k, NULLIF(dn, '')::date, o
			  FROM unnest($2::text[], $3::text[], $4::text[], $5::bool[], $6::text[], $7::int[]) AS t(m, c, b, k, dn, o)
			RETURNING *
		)
		SELECT `+itemColumns+` FROM ins i ORDER BY i.due_month, i.sort_order`,
		saved.ID, months, cats, bodies, keys, dones, orders)
	if err != nil {
		return translate("roadmap repository: create items", err)
	}
	items, err := pgx.CollectRows(rows, scanItemRow)
	if err != nil {
		return translate("roadmap repository: create items scan", err)
	}

	names, horders := make([]string, len(d.Habits)), make([]int32, len(d.Habits))
	for i, h := range d.Habits {
		names[i], horders[i] = h.Name, int32(h.SortOrder)
	}
	rows, err = tx.Query(ctx, `
		WITH ins AS (
			INSERT INTO career.roadmap_habits (roadmap_id, name, sort_order)
			SELECT $1, n, o FROM unnest($2::text[], $3::int[]) AS t(n, o)
			RETURNING *
		)
		SELECT `+habitColumns+` FROM ins h ORDER BY h.sort_order`,
		saved.ID, names, horders)
	if err != nil {
		return translate("roadmap repository: create habits", err)
	}
	habits, err := pgx.CollectRows(rows, scanHabitRow)
	if err != nil {
		return translate("roadmap repository: create habits scan", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return translate("roadmap repository: commit", err)
	}
	*d = domain.RoadmapDetail{Roadmap: saved, Items: items, Habits: habits}
	return nil
}

func (r *RoadmapRepository) Update(ctx context.Context, userID string, rm *domain.Roadmap) error {
	phases, err := json.Marshal(rm.Phases)
	if err != nil {
		return fmt.Errorf("roadmap repository: marshal phases: %w", err)
	}
	// 분류 목록에서 뺀 분류는 그 할 일들에서도 비운다 (한 문장이라 함께 반영되거나 함께 실패한다).
	// 그래야 화면의 분류 색 · md 내보내기 → 다시 올리기가 목록과 어긋나지 않는다.
	q := `
		WITH up AS (
			UPDATE career.roadmaps AS r
			   SET title = $3, lead = $4, starts_on = $5::text::date, ends_on = $6::text::date,
			       categories = $7, phases = $8::text::jsonb, updated_at = now()
			 WHERE r.id = $1 AND r.user_id = $2
			RETURNING r.*
		), cleared AS (
			UPDATE career.roadmap_items i
			   SET category = '', updated_at = now()
			  FROM up
			 WHERE i.roadmap_id = up.id AND i.category <> '' AND NOT (i.category = ANY (up.categories))
		)
		SELECT ` + roadmapColumns + ` FROM up r`
	var saved domain.Roadmap
	if err := scanRoadmap(r.pool.QueryRow(ctx, q, rm.ID, userID, rm.Title, rm.Lead, rm.StartsOn, rm.EndsOn, rm.Categories, string(phases)), &saved); err != nil {
		return translate("roadmap repository: update", err)
	}
	*rm = saved
	return nil
}

func (r *RoadmapRepository) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM career.roadmaps WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return translate("roadmap repository: delete", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("roadmap repository: delete %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

// ── 항목 ─────────────────────────────────

func (r *RoadmapRepository) FindItem(ctx context.Context, userID, roadmapID, itemID string) (*domain.RoadmapItem, error) {
	q := `SELECT ` + itemColumns + `
		FROM career.roadmap_items i JOIN career.roadmaps r ON r.id = i.roadmap_id
		WHERE i.id = $1 AND i.roadmap_id = $2 AND r.user_id = $3`
	it, err := scanItem(r.pool.QueryRow(ctx, q, itemID, roadmapID, userID))
	return it, translate("roadmap repository: find item", err)
}

// CreateItem 은 그 달의 맨 뒤에 넣는다. 로드맵이 남의 것이면 0행 → ErrNotFound.
func (r *RoadmapRepository) CreateItem(ctx context.Context, userID, roadmapID string, it *domain.RoadmapItem) error {
	q := `
		WITH ins AS (
			INSERT INTO career.roadmap_items (roadmap_id, due_month, category, body, is_key, done_on, sort_order)
			SELECT r.id, $3, $4, $5, $6, NULLIF($7::text, '')::date,
			       COALESCE((SELECT max(sort_order) + 1 FROM career.roadmap_items WHERE roadmap_id = r.id), 0)
			  FROM career.roadmaps r
			 WHERE r.id = $1 AND r.user_id = $2
			RETURNING *
		)
		SELECT ` + itemColumns + ` FROM ins i`
	saved, err := scanItem(r.pool.QueryRow(ctx, q, roadmapID, userID, it.Month, it.Category, it.Body, it.IsKey, it.DoneOn))
	if err != nil {
		return translate("roadmap repository: create item", err)
	}
	*it = *saved
	return nil
}

func (r *RoadmapRepository) UpdateItem(ctx context.Context, userID, roadmapID string, it *domain.RoadmapItem) error {
	q := `
		WITH up AS (
			UPDATE career.roadmap_items i
			   SET due_month = $4, category = $5, body = $6, is_key = $7, done_on = NULLIF($8::text, '')::date, updated_at = now()
			  FROM career.roadmaps r
			 WHERE i.id = $1 AND i.roadmap_id = $2 AND r.id = i.roadmap_id AND r.user_id = $3
			RETURNING i.*
		)
		SELECT ` + itemColumns + ` FROM up i`
	saved, err := scanItem(r.pool.QueryRow(ctx, q, it.ID, roadmapID, userID, it.Month, it.Category, it.Body, it.IsKey, it.DoneOn))
	if err != nil {
		return translate("roadmap repository: update item", err)
	}
	*it = *saved
	return nil
}

func (r *RoadmapRepository) DeleteItem(ctx context.Context, userID, roadmapID, itemID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM career.roadmap_items i USING career.roadmaps r
		 WHERE i.id = $1 AND i.roadmap_id = $2 AND r.id = i.roadmap_id AND r.user_id = $3`, itemID, roadmapID, userID)
	if err != nil {
		return translate("roadmap repository: delete item", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("roadmap repository: delete item %s: %w", itemID, domain.ErrNotFound)
	}
	return nil
}

// ── 습관 ─────────────────────────────────

func (r *RoadmapRepository) CreateHabit(ctx context.Context, userID, roadmapID string, h *domain.RoadmapHabit) error {
	q := `
		WITH ins AS (
			INSERT INTO career.roadmap_habits (roadmap_id, name, sort_order)
			SELECT r.id, $3, COALESCE((SELECT max(sort_order) + 1 FROM career.roadmap_habits WHERE roadmap_id = r.id), 0)
			  FROM career.roadmaps r
			 WHERE r.id = $1 AND r.user_id = $2
			   AND (SELECT count(*) FROM career.roadmap_habits WHERE roadmap_id = r.id) < $4
			RETURNING *
		)
		SELECT ` + habitColumns + ` FROM ins h`
	saved, err := scanHabit(r.pool.QueryRow(ctx, q, roadmapID, userID, h.Name, maxHabits))
	if err != nil {
		translated := translate("roadmap repository: create habit", err)
		if errors.Is(translated, domain.ErrNotFound) {
			return r.habitLimitOrMissing(ctx, userID, roadmapID, translated)
		}
		return translated
	}
	*h = *saved
	return nil
}

// maxHabits 는 유스케이스의 습관 한도와 같다 (md 가져오기도 같은 수까지만 받는다).
const maxHabits = 10

// habitLimitOrMissing 은 습관을 못 넣은 이유가 한도인지(400) 로드맵이 없어서인지(404) 가린다.
func (r *RoadmapRepository) habitLimitOrMissing(ctx context.Context, userID, roadmapID string, notFound error) error {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM career.roadmaps WHERE id = $1 AND user_id = $2)`, roadmapID, userID).Scan(&exists)
	if err != nil || !exists {
		return notFound
	}
	return fmt.Errorf("매주 습관은 %d개까지 넣을 수 있습니다: %w", maxHabits, domain.ErrInvalidInput)
}

func (r *RoadmapRepository) DeleteHabit(ctx context.Context, userID, roadmapID, habitID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM career.roadmap_habits h USING career.roadmaps r
		 WHERE h.id = $1 AND h.roadmap_id = $2 AND r.id = h.roadmap_id AND r.user_id = $3`, habitID, roadmapID, userID)
	if err != nil {
		return translate("roadmap repository: delete habit", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("roadmap repository: delete habit %s: %w", habitID, domain.ErrNotFound)
	}
	return nil
}

// SetHabitWeek 는 그 주를 더하거나(이미 있으면 그대로) 뺀다. 주는 정렬해 둔다.
func (r *RoadmapRepository) SetHabitWeek(ctx context.Context, userID, roadmapID, habitID, week string, on bool) (*domain.RoadmapHabit, error) {
	q := `
		WITH up AS (
			UPDATE career.roadmap_habits h
			   SET weeks = CASE
			                 WHEN $4 THEN ARRAY(SELECT DISTINCT w FROM unnest(array_append(h.weeks, $5::text)) AS w ORDER BY w)
			                 ELSE array_remove(h.weeks, $5::text)
			               END
			  FROM career.roadmaps r
			 WHERE h.id = $1 AND h.roadmap_id = $2 AND r.id = h.roadmap_id AND r.user_id = $3
			RETURNING h.*
		)
		SELECT ` + habitColumns + ` FROM up h`
	h, err := scanHabit(r.pool.QueryRow(ctx, q, habitID, roadmapID, userID, on, week))
	return h, translate("roadmap repository: set habit week", err)
}

// ── 읽기 도우미 ───────────────────────────

func scanRoadmap(row pgx.Row, rm *domain.Roadmap) error {
	var phases string
	if err := row.Scan(&rm.ID, &rm.Title, &rm.Lead, &rm.StartsOn, &rm.EndsOn, &rm.Categories, &phases, &rm.CreatedAt, &rm.UpdatedAt); err != nil {
		return err
	}
	return fillRoadmap(rm, phases)
}

func fillRoadmap(rm *domain.Roadmap, phases string) error {
	if rm.Categories == nil {
		rm.Categories = []string{}
	}
	rm.Phases = []domain.RoadmapPhase{}
	if err := json.Unmarshal([]byte(phases), &rm.Phases); err != nil {
		return fmt.Errorf("roadmap repository: decode phases: %w", err)
	}
	return nil
}

func scanItem(row pgx.Row) (*domain.RoadmapItem, error) {
	var it domain.RoadmapItem
	if err := row.Scan(&it.ID, &it.Month, &it.Category, &it.Body, &it.IsKey, &it.DoneOn, &it.SortOrder); err != nil {
		return nil, err
	}
	return &it, nil
}

func scanItemRow(row pgx.CollectableRow) (domain.RoadmapItem, error) {
	it, err := scanItem(row)
	if err != nil {
		return domain.RoadmapItem{}, err
	}
	return *it, nil
}

func scanHabit(row pgx.Row) (*domain.RoadmapHabit, error) {
	var h domain.RoadmapHabit
	if err := row.Scan(&h.ID, &h.Name, &h.Weeks, &h.SortOrder); err != nil {
		return nil, err
	}
	if h.Weeks == nil {
		h.Weeks = []string{}
	}
	return &h, nil
}

func scanHabitRow(row pgx.CollectableRow) (domain.RoadmapHabit, error) {
	h, err := scanHabit(row)
	if err != nil {
		return domain.RoadmapHabit{}, err
	}
	return *h, nil
}
