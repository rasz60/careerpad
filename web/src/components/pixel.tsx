import { useMemo, type ReactNode } from 'react';
import { Link } from 'react-router-dom';

/** 도트 그림. 글자 한 칸이 한 픽셀이고 '.' 는 투명 */
export type Bitmap = readonly string[];
export type Palette = Record<string, string>;

/** 같은 줄에서 같은 색이 이어지면 사각형 하나로 합쳐 그린다 */
export const bitmapRects = (rows: Bitmap, pal: Palette): { x: number; y: number; w: number; fill: string }[] => {
  const out: { x: number; y: number; w: number; fill: string }[] = [];
  rows.forEach((row, y) => {
    let x = 0;
    while (x < row.length) {
      const ch = row[x]!;
      let w = 1;
      while (x + w < row.length && row[x + w] === ch) w += 1;
      const fill = pal[ch];
      if (fill) out.push({ x, y, w, fill });
      x += w;
    }
  });
  return out;
};

export const Pix = ({ rows, pal, label, className = '' }: { rows: Bitmap; pal: Palette; label?: string; className?: string }) => (
  <svg
    className={`px ${className}`}
    viewBox={`0 0 ${rows[0]!.length} ${rows.length}`}
    {...(label ? { role: 'img', 'aria-label': label } : { 'aria-hidden': true })}
  >
    {bitmapRects(rows, pal).map((r, i) => (
      <rect key={i} x={r.x} y={r.y} width={r.w} height="1" fill={r.fill} />
    ))}
  </svg>
);

const INK = '#0d141b';
export const HERO: Bitmap = [
  '....kkkk....',
  '...kddddk...',
  '..kdddddak..',
  '..kddsssdk..',
  '..ksskskssk.',
  '..ksssssssk.',
  '...ksrrrsk..',
  '..krrrrrrrk.',
  '.ksrrrrrrrsk',
  '.ksrrkkrrrsk',
  '..kkb.kb.kk.',
  '..kbbk.kbbk.',
];
export const HERO_PAL: Palette = { k: INK, d: '#6b4a2b', s: '#f1c9a0', a: '#d94a4a', r: '#3d63b8', b: '#5a3a22' };

export const Hero = ({ className = '' }: { className?: string }) => <Pix rows={HERO} pal={HERO_PAL} className={className} />;

/** 타일맵 (50x30): w 바다, v 물결, a 모래, g 풀, h 짙은 풀, t 숲, u 짙은 숲, m 산, n 산봉우리, p 길 */
const MAP_W = 50;
const MAP_H = 30;
const MAP_PAL: Palette = {
  w: '#7f9fb3', v: '#8aaabd', a: '#d8cda6', g: '#9db484', h: '#93ab7b',
  t: '#6f8d62', u: '#617f56', m: '#8d8b86', n: '#b3b1aa', p: '#d9ca9b',
};

/** 섬 모양: 타원에 완만한 굴곡을 준 값. 1 이하면 땅, 0.91 초과면 해변 */
/** 좌표마다 같은 값이 나오는 잔무늬용 해시 (무작위가 아니라 새로 그려도 모양이 같다) */
const hash = (x: number, y: number): number => Math.abs(Math.imul(x + 11, 73856093) ^ Math.imul(y + 7, 19349663)) >>> 3;

const islandDist = (x: number, y: number): number => {
  const dx = (x + 0.5 - 25) / 22;
  const dy = (y + 0.5 - 15.5) / 12;
  const a = Math.atan2(dy, dx);
  return Math.hypot(dx, dy) / (1 + 0.07 * Math.sin(a * 3 + 1) + 0.04 * Math.sin(a * 5 + 2));
};

/** 숲 · 산 덩어리 [중심 x, 중심 y, 반지름] */
const FORESTS: readonly [number, number, number][] = [[5, 16, 2.6], [32, 23, 2.8], [19, 11, 2.2], [43, 20, 2.2]];
const PEAKS: readonly [number, number, number][] = [[41, 12.5, 2.8], [18, 5.5, 2.2]];
const inBlob = (blobs: readonly [number, number, number][], x: number, y: number) =>
  blobs.find(([cx, cy, r]) => Math.hypot((x - cx) * 1.1, (y - cy) * 1.3) <= r);

/** 메뉴 위치(%)를 타일 좌표로 바꿔, 첫 장소에서 나머지로 ㄱ자 길을 낸다. 장소 자리는 3칸 광장 */
export const buildMap = (points: readonly { x: number; y: number }[]): Bitmap => {
  const g: string[][] = [];
  for (let y = 0; y < MAP_H; y += 1) {
    const row: string[] = [];
    for (let x = 0; x < MAP_W; x += 1) {
      const d = islandDist(x, y);
      let c: string;
      if (d > 1) c = hash(x, y) % 23 === 0 ? 'v' : 'w';
      else if (d > 0.92) c = 'a';
      else if (inBlob(PEAKS, x, y)) {
        const [cx, cy, r] = inBlob(PEAKS, x, y)!;
        c = y <= cy - r / 3 && Math.abs(x - cx) <= 1 ? 'n' : 'm';
      } else if (inBlob(FORESTS, x, y)) c = hash(x, y) % 3 === 0 ? 'u' : 't';
      else c = hash(x, y) % 6 === 0 ? 'h' : 'g';
      row.push(c);
    }
    g.push(row);
  }
  const tiles = points.map((p) => ({ x: Math.round((p.x / 100) * MAP_W - 0.5), y: Math.round((p.y / 100) * MAP_H - 0.5) }));
  const put = (x: number, y: number) => {
    if (x >= 0 && x < MAP_W && y >= 0 && y < MAP_H) g[y]![x] = 'p';
  };
  const home = tiles[0];
  if (home) {
    for (const t of tiles.slice(1)) {
      for (let x = Math.min(home.x, t.x); x <= Math.max(home.x, t.x); x += 1) put(x, home.y);
      for (let y = Math.min(home.y, t.y); y <= Math.max(home.y, t.y); y += 1) put(t.x, y);
    }
  }
  for (const t of tiles) for (let dx = -1; dx <= 1; dx += 1) put(t.x + dx, t.y);
  return g.map((r) => r.join(''));
};

export interface MapNode {
  to: string;
  label: string;
  /** 지도 위 위치 (%) */
  x: number;
  y: number;
  /** 알림 수 같은 짧은 꼬리표 */
  note?: ReactNode;
  noteLabel?: string;
}

/** 월드맵: 메뉴가 장소다. 장소 사이는 길로 이어지고, 지금 있는 곳은 빨간 점이 깜빡이며 캐릭터가 선다 */
export const WorldMap = ({ nodes, current }: { nodes: MapNode[]; current: string }) => {
  const cur = nodes.find((n) => n.to === current);
  const key = nodes.map((n) => `${n.x},${n.y}`).join('|');
  const rows = useMemo(() => buildMap(nodes), [key]); // 위치가 같으면 다시 그리지 않는다
  return (
    <nav className="map" aria-label="메뉴 지도">
      <Pix rows={rows} pal={MAP_PAL} />
      {nodes.map((n) => (
        <Link
          key={n.to}
          to={n.to}
          className={`n${n.to === current ? ' cur' : ''}`}
          style={{ left: `${n.x}%`, top: `${n.y}%` }}
          aria-current={n.to === current ? 'page' : undefined}
        >
          <span>
            {n.label}
            {n.note ? (
              <b className="n-note" aria-label={n.noteLabel}>
                {n.note}
              </b>
            ) : null}
          </span>
          <i />
        </Link>
      ))}
      {cur && (
        <span className="me" style={{ left: `${cur.x}%`, top: `${Math.min(cur.y + 13, 92)}%` }} aria-hidden="true">
          <Hero />
        </span>
      )}
    </nav>
  );
};

/** 로딩: 도트 캐릭터가 제자리에서 걷는다 (모션을 줄이는 설정이면 정지) */
export const Loading = ({ label = '불러오는 중…', full = false }: { label?: string; full?: boolean }) => (
  <div className={`loading${full ? ' is-full' : ''}`} role="status" aria-live="polite">
    <span className="loading-hero" aria-hidden="true">
      <Hero />
    </span>
    <span className="loading-road" aria-hidden="true" />
    <p>{label}</p>
  </div>
);
