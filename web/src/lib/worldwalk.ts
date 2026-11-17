// 월드맵 위에서 캐릭터가 걷는 규칙. 화면과 상관없는 순수 함수만 둔다.

export const MAP_W = 50;
export const MAP_H = 30;

export interface Tile {
  x: number;
  y: number;
}

export type Dir = 'up' | 'down' | 'left' | 'right';

const DELTA: Record<Dir, Tile> = {
  up: { x: 0, y: -1 },
  down: { x: 0, y: 1 },
  left: { x: -1, y: 0 },
  right: { x: 1, y: 0 },
};

/** 걸을 수 있는 타일: 모래 a · 풀 g h · 숲 t u · 길 p. 바다 w v · 산 m n 은 막힌다 */
const WALKABLE = new Set(['a', 'g', 'h', 't', 'u', 'p']);
const ROAD = 'p';

export const isWalkable = (rows: readonly string[], x: number, y: number): boolean =>
  x >= 0 && x < MAP_W && y >= 0 && y < MAP_H && WALKABLE.has(rows[y]?.[x] ?? '');

/** 메뉴 위치(%)가 놓이는 타일 (buildMap 의 장소 타일과 같은 계산) */
export const tileOf = (p: { x: number; y: number }): Tile => ({
  x: Math.round((p.x / 100) * MAP_W - 0.5),
  y: Math.round((p.y / 100) * MAP_H - 0.5),
});

/** 타일 중앙의 지도 위 위치(%) */
export const percentOf = (t: Tile): { left: number; top: number } => ({
  left: ((t.x + 0.5) / MAP_W) * 100,
  top: ((t.y + 0.5) / MAP_H) * 100,
});

/** 방향키 한 칸. 막힌 곳이면 제자리 */
export const stepTile = (rows: readonly string[], from: Tile, dir: Dir): Tile => {
  const d = DELTA[dir];
  const next = { x: from.x + d.x, y: from.y + d.y };
  return isWalkable(rows, next.x, next.y) ? next : from;
};

/** 장소에 들어갈 수 있는 자리: 장소 타일과 그 좌우 한 칸(3칸 광장) */
export const onPlaza = (place: Tile, at: Tile): boolean => at.y === place.y && Math.abs(at.x - place.x) <= 1;

/** 길 위는 1, 길 밖 땅은 3. 길을 따라가는 최단 경로를 찾는다 (출발 칸은 빼고 도착 칸은 포함) */
export const findPath = (rows: readonly string[], from: Tile, to: Tile): Tile[] => {
  const idx = (t: Tile) => t.y * MAP_W + t.x;
  const dist = new Array<number>(MAP_W * MAP_H).fill(Infinity);
  const prev = new Array<number>(MAP_W * MAP_H).fill(-1);
  const done = new Array<boolean>(MAP_W * MAP_H).fill(false);
  dist[idx(from)] = 0;
  const open: number[] = [idx(from)];
  while (open.length > 0) {
    let best = 0;
    for (let i = 1; i < open.length; i += 1) if (dist[open[i]!]! < dist[open[best]!]!) best = i;
    const cur = open.splice(best, 1)[0]!;
    if (done[cur]) continue;
    done[cur] = true;
    if (cur === idx(to)) break;
    const cx = cur % MAP_W;
    const cy = Math.floor(cur / MAP_W);
    for (const d of Object.values(DELTA)) {
      const nx = cx + d.x;
      const ny = cy + d.y;
      if (!isWalkable(rows, nx, ny)) continue;
      const ni = ny * MAP_W + nx;
      const cost = dist[cur]! + (rows[ny]![nx] === ROAD ? 1 : 3);
      if (cost < dist[ni]!) {
        dist[ni] = cost;
        prev[ni] = cur;
        open.push(ni);
      }
    }
  }
  const path: Tile[] = [];
  for (let i = idx(to); i !== idx(from); i = prev[i]!) {
    if (i < 0) return []; // 길이 없다
    path.push({ x: i % MAP_W, y: Math.floor(i / MAP_W) });
  }
  return path.reverse();
};
