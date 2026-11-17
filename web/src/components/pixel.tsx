import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent, type MouseEvent, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { findPath, MAP_H, MAP_W, onPlaza, percentOf, stepTile, tileOf, type Dir, type Tile } from '../lib/worldwalk';
import { placeTip } from '../lib/place';

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
  const tiles = points.map(tileOf);
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

/** 'X' 가 적힌 팻말 (들어갈 수 없는 곳에서 스페이스를 눌렀을 때 캐릭터가 든다) */
export const SIGN_X: Bitmap = [
  'kkkkkkkkk',
  'kwwwwwwwk',
  'kwrwwwrwk',
  'kwwrwrwwk',
  'kwwwrwwwk',
  'kwwrwrwwk',
  'kwrwwwrwk',
  'kwwwwwwwk',
  'kkkkkkkkk',
  '...kdk...',
  '...kdk...',
];
const SIGN_PAL: Palette = { k: INK, w: '#f3efe0', r: '#d94a4a', d: '#6b4a2b' };

const WALK_MS = 100; // 한 칸 걷는 시간
const ARRIVE_MS = 200; // 도착하고 화면이 바뀌기 전 잠깐 멈춤
const SIGN_MS = 1200; // X 팻말을 들고 있는 시간
const REPEAT_DELAY_MS = 300; // 방향 버튼을 길게 누를 때 연속 이동이 시작되기까지
const REPEAT_MS = 110;

const reducedMotion = () => typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

const ARROWS: { dir: Dir; mark: string; label: string }[] = [
  { dir: 'up', mark: '▲', label: '위로' },
  { dir: 'left', mark: '◀', label: '왼쪽으로' },
  { dir: 'right', mark: '▶', label: '오른쪽으로' },
  { dir: 'down', mark: '▼', label: '아래로' },
];
const KEY_DIR: Record<string, Dir> = { ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right' };

/** 월드맵: 메뉴가 장소다. 장소를 누르면 캐릭터가 길을 따라 걸어간 뒤 화면이 바뀐다.
 *  우측 상단 방향 패드(또는 지도에 포커스를 둔 방향키)로 직접 걷고, 장소에서 입장(스페이스)을 누르면 들어간다.
 *  장소가 아닌 곳에서 입장을 누르면 캐릭터가 X 팻말을 든다. 그림 아래의 안내창(.dlg)도 여기서 같이 그린다 */
export const WorldMap = ({
  nodes,
  current,
  hint,
  onGo,
}: {
  nodes: MapNode[];
  current: string;
  /** 안내창에 보여 줄 상태 안내 */
  hint: ReactNode;
  /** 걷기를 마치고 들어갈 메뉴 경로 */
  onGo: (to: string) => void;
}) => {
  const key = nodes.map((n) => `${n.x},${n.y}`).join('|');
  const rows = useMemo(() => buildMap(nodes), [key]); // 위치가 같으면 다시 그리지 않는다
  const places = useMemo(() => nodes.map((n) => ({ node: n, tile: tileOf(n) })), [key]);
  const cur = places.find((p) => p.node.to === current) ?? places[0];

  const [pos, setPos] = useState<Tile>(() => cur?.tile ?? { x: 0, y: 0 });
  const [going, setGoing] = useState<MapNode | null>(null);
  const [sign, setSign] = useState(false);
  // 좁은 화면에서는 패드가 장소를 가리지 않게 접어 두고 시작한다
  const [padOpen, setPadOpen] = useState(() => typeof window === 'undefined' || window.innerWidth > 820);
  const posRef = useRef(pos);
  const walkTimer = useRef<number | undefined>(undefined);
  const signTimer = useRef<number | undefined>(undefined);
  const repeatTimer = useRef<number | undefined>(undefined);
  const repeatDelay = useRef<number | undefined>(undefined);
  const goingRef = useRef(false);
  const mapRef = useRef<HTMLElement>(null);
  const latest = useRef({ rows, places, current, onGo });
  latest.current = { rows, places, current, onGo };

  const place = (t: Tile) => places.find((p) => onPlaza(p.tile, t));
  const here = place(pos);

  const put = useCallback((t: Tile) => {
    posRef.current = t;
    setPos(t);
  }, []);

  const stopWalk = useCallback(() => {
    window.clearTimeout(walkTimer.current);
    goingRef.current = false;
    setGoing(null);
  }, []);

  const stopRepeat = useCallback(() => {
    window.clearTimeout(repeatDelay.current);
    window.clearInterval(repeatTimer.current);
  }, []);

  useEffect(
    () => () => {
      window.clearTimeout(walkTimer.current);
      window.clearTimeout(signTimer.current);
      window.clearTimeout(repeatDelay.current);
      window.clearInterval(repeatTimer.current);
    },
    [],
  );

  /** 장소를 향해 걷는다. 걷는 중에 다시 부르면 새 목적지로 바꾼다 */
  const walkTo = useCallback(
    (node: MapNode, tile: Tile) => {
      const { rows: map, onGo: go } = latest.current;
      window.clearTimeout(walkTimer.current);
      window.clearTimeout(signTimer.current);
      setSign(false);
      if (reducedMotion()) {
        put(tile);
        go(node.to);
        return;
      }
      const path = findPath(map, posRef.current, tile);
      goingRef.current = true;
      setGoing(node);
      const tick = (i: number) => {
        const next = path[i];
        if (!next) {
          walkTimer.current = window.setTimeout(() => go(node.to), ARRIVE_MS);
          return;
        }
        put(next);
        walkTimer.current = window.setTimeout(() => tick(i + 1), WALK_MS);
      };
      tick(0);
    },
    [put],
  );

  const move = useCallback(
    (dir: Dir) => {
      stopWalk();
      window.clearTimeout(signTimer.current);
      setSign(false);
      put(stepTile(latest.current.rows, posRef.current, dir));
    },
    [put, stopWalk],
  );

  /** 입장(스페이스): 장소 위면 들어가고, 아니면 X 팻말 */
  const enter = useCallback(() => {
    if (goingRef.current) return;
    const { places: all, onGo: go, current: cu } = latest.current;
    const at = all.find((p) => onPlaza(p.tile, posRef.current));
    if (at) {
      if (at.node.to !== cu) go(at.node.to);
      return;
    }
    setSign(true);
    window.clearTimeout(signTimer.current);
    signTimer.current = window.setTimeout(() => setSign(false), SIGN_MS);
  }, []);

  const onLink = (e: MouseEvent<HTMLAnchorElement>, n: MapNode, tile: Tile) => {
    // 새 탭 · 새 창으로 여는 클릭은 링크 그대로 둔다
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    // 이미 이 장소(현재 화면) 위에 서 있으면 걸을 일이 없다. 다른 곳에 있다면 돌아간다
    if (n.to === current && !goingRef.current && onPlaza(tile, posRef.current)) return;
    walkTo(n, tile);
  };

  const onKey = (e: KeyboardEvent<HTMLElement>) => {
    if (e.altKey || e.ctrlKey || e.metaKey) return;
    const dir = KEY_DIR[e.key];
    if (dir) {
      e.preventDefault();
      move(dir);
      return;
    }
    // 패드 버튼 위의 스페이스 · 엔터는 버튼 자체의 클릭으로 처리한다
    if ((e.target as HTMLElement).closest('.dpad')) return;
    if (e.key === ' ') {
      e.preventDefault();
      if (!e.repeat) enter();
    }
  };

  const holdStart = (dir: Dir) => {
    stopRepeat();
    move(dir);
    repeatDelay.current = window.setTimeout(() => {
      repeatTimer.current = window.setInterval(() => move(dir), REPEAT_MS);
    }, REPEAT_DELAY_MS);
  };

  const me = percentOf(pos);
  return (
    <>
      <nav ref={mapRef} className="map" aria-label="메뉴 지도 (방향키로 걷고 스페이스로 입장)" tabIndex={0} onKeyDown={onKey}>
        <Pix rows={rows} pal={MAP_PAL} />
        {places.map(({ node: n, tile }) => (
          <Link
            key={n.to}
            to={n.to}
            className={`n${n.to === current ? ' cur' : ''}${here?.node.to === n.to ? ' here' : ''}`}
            style={{ left: `${n.x}%`, top: `${n.y}%` }}
            aria-current={n.to === current ? 'page' : undefined}
            onClick={(e) => onLink(e, n, tile)}
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
        <span className={`me${going ? ' walking' : ''}`} style={{ left: `${me.left}%`, top: `${me.top}%` }} aria-hidden="true">
          <span className="me-body">
            <Hero />
          </span>
          {sign && <Pix rows={SIGN_X} pal={SIGN_PAL} className="sign" />}
        </span>
        <div className="dpad">
          <button
            type="button"
            className="dpad-toggle"
            aria-expanded={padOpen}
            onClick={() => {
              stopRepeat();
              setPadOpen((v) => !v);
            }}
          >
            방향 패드 {padOpen ? '접기' : '펴기'}
          </button>
          {padOpen && (
            <div className="dpad-grid" role="group" aria-label="방향 패드">
              {ARROWS.map((a) => (
                <button
                  key={a.dir}
                  type="button"
                  className={`dpad-btn dpad-${a.dir}`}
                  aria-label={a.label}
                  onPointerDown={(e) => {
                    if (e.button !== 0) return;
                    mapRef.current?.focus({ preventScroll: true }); // 키보드(방향키 · 스페이스)가 이어서 지도에서 동작하게
                    holdStart(a.dir);
                  }}
                  onPointerUp={stopRepeat}
                  onPointerCancel={stopRepeat}
                  onPointerLeave={stopRepeat}
                  onContextMenu={(e) => e.preventDefault()}
                  onClick={(e) => {
                    if (e.detail === 0) move(a.dir); // 키보드로 누른 경우 (마우스 · 터치는 pointerdown 에서 처리)
                  }}
                >
                  <span aria-hidden="true">{a.mark}</span>
                </button>
              ))}
              <button
                type="button"
                className="dpad-btn dpad-go"
                aria-disabled={!here || undefined}
                aria-label={here ? `${here.node.label} 입장 (스페이스)` : '입장 (스페이스)'}
                onClick={(e) => {
                  if (e.detail > 0) mapRef.current?.focus({ preventScroll: true });
                  enter();
                }}
              >
                입장
              </button>
            </div>
          )}
        </div>
      </nav>
      <div className="dlg" role="status">
        <b>현재 위치</b>
        {hint}
        <p className="dlg-tip">{placeTip({ place: here?.node.label, isCurrent: here?.node.to === current, going: going?.label, sign })}</p>
      </div>
    </>
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
