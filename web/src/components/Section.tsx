import type { ReactNode } from 'react';

interface SectionProps {
  title: string;
  id?: string;
  /** 제목 아래 오른쪽 도구 (주인의 편집 버튼 등) */
  tools?: ReactNode;
  children: ReactNode;
  className?: string;
}

/** 이력서 한 칸: 왼쪽 이름 + 오른쪽 내용 */
export const Section = ({ title, id, tools, children, className = '' }: SectionProps) => (
  <section className={`r-section ${className}`} id={id}>
    {/* 주인의 편집 도구는 왼쪽 이름 아래에 둬서 본문을 밀어내지 않는다 */}
    <div className="r-label">
      <h2>{title}</h2>
      {tools ? <div className="r-section-tools">{tools}</div> : null}
    </div>
    <div className="r-body">{children}</div>
  </section>
);
