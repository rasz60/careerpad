import type { ReactNode } from 'react';
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

/** 타일맵: w 바다, g 풀, t 숲, m 산, p 길, s 모래 */
const MAP: Bitmap = [
  'wwwwwwwwwwwwwwwwwwww',
  'wwggggwwwwwwmmwwwwww',
  'wgggggggwwwmmmmwwwww',
  'wgggtgggppppmmgwwwww',
  'wggtttgpgggggggggwww',
  'wwgggggpgggtttggwwww',
  'wwwgggppggtttgggwwww',
  'wwwwgpgggggggggwwwww',
  'wwwwgpggssssgwwwwwww',
  'wwwwwpppssswwwwwwwww',
  'wwwwwwwwwwwwwwwwwwww',
  'wwwwwwwwwwwwwwwwwwww',
];
const MAP_PAL: Palette = { w: '#5c8fb0', g: '#8fb36a', t: '#4f7a45', m: '#8a8279', p: '#d9c48a', s: '#ead9a6' };

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

/** 월드맵: 메뉴가 장소다. 지금 있는 곳은 빨간 점이 깜빡이고 캐릭터가 선다 */
export const WorldMap = ({ nodes, current }: { nodes: MapNode[]; current: string }) => {
  const cur = nodes.find((n) => n.to === current);
  return (
    <nav className="map" aria-label="메뉴 지도">
      <Pix rows={MAP} pal={MAP_PAL} />
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
