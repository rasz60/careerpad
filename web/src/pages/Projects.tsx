import { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { Field, Modal, RowList, TextArea, Toggle, WordsField } from '../components/form';
import { usePage } from '../components/Layout';
import { api } from '../lib/api';
import { periodLabel } from '../lib/format';
import type { Link, Project, ProjectInput, ProjectKind, PublicPage } from '../lib/types';
import { Section } from '../components/Section';

const KINDS: { kind: ProjectKind; label: string }[] = [
  { kind: 'work', label: '업무 프로젝트' },
  { kind: 'personal', label: '개인 프로젝트' },
];

/** 예전 주소 /:slug/projects → 이력서의 프로젝트 섹션 */
export const Projects = () => {
  const { page } = usePage();
  return <Navigate to={{ pathname: `/${page.profile.slug}`, hash: 'projects' }} replace />;
};

interface ProjectsSectionProps {
  page: PublicPage;
  reload: () => void;
  /** 작업 공간에서만 추가 · 편집 */
  editable: boolean;
}

/** 이력서의 프로젝트 칸: 업무 · 개인을 나눠 펼친 채로 보여 준다. 긴 설명만 '자세히'로 */
export const ProjectsSection = ({ page, reload, editable }: ProjectsSectionProps) => {
  const [editing, setEditing] = useState<{ project: Project | null; kind: ProjectKind } | null>(null);
  const owner = editable;
  // 주인이 공개 화면으로 볼 때도 방문자와 똑같이 (비공개 프로젝트는 작업 공간에서만)
  const shown = owner ? page.projects : page.projects.filter((p) => p.is_public);
  const groups = KINDS.map((k) => ({ ...k, list: shown.filter((p) => p.kind === k.kind) })).filter((g) => g.list.length > 0);

  if (groups.length === 0 && !owner) return null;

  return (
    <Section
      title="프로젝트"
      id="projects"
      tools={
        owner && (
          <button type="button" className="text-btn no-print" onClick={() => setEditing({ project: null, kind: 'work' })}>
            + 프로젝트 추가
          </button>
        )
      }
    >
      {groups.length === 0 && <p className="muted">아직 없어요. ‘+ 프로젝트 추가’로 넣어 보세요.</p>}
      {groups.map((g) => (
        <div className="proj-group" key={g.kind}>
          {groups.length > 1 && <h3>{g.label}</h3>}
          <ul className="proj-list">
            {g.list.map((p) => (
              <ProjectItem key={p.id} project={p} owner={owner} onEdit={() => setEditing({ project: p, kind: p.kind })} />
            ))}
          </ul>
        </div>
      ))}

      {editing && (
        <ProjectEditor
          project={editing.project}
          kind={editing.kind}
          nextOrder={page.projects.filter((p) => p.kind === editing.kind).length}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            reload();
          }}
        />
      )}
    </Section>
  );
};

interface ProjectItemProps {
  project: Project;
  owner: boolean;
  onEdit: () => void;
}

const ProjectItem = ({ project: p, owner, onEdit }: ProjectItemProps) => {
  const [open, setOpen] = useState(false);
  const meta = [p.organization, p.role].filter(Boolean).join(' · ');
  return (
    <li className={`proj ${p.is_public ? '' : 'is-private'}`}>
      <div className="proj-head">
        <h4>{p.title}</h4>
        <span className="proj-when">{periodLabel(p.started_on, p.ended_on, '진행 중')}</span>
      </div>
      {meta && <p className="sub">{meta}</p>}
      {p.summary && <p className="proj-summary">{p.summary}</p>}
      {p.tech_stack.length > 0 && (
        <div className="chips">
          {p.tech_stack.map((t) => (
            <span className="chip small" key={t}>
              {t}
            </span>
          ))}
        </div>
      )}
      {p.description && (
        <p className="prose proj-desc" id={`proj-${p.id}`} hidden={!open}>
          {p.description}
        </p>
      )}
      <div className="proj-foot">
        {p.description && (
          <button type="button" className="text-btn proj-more" aria-expanded={open} aria-controls={`proj-${p.id}`} onClick={() => setOpen(!open)}>
            {open ? '접기' : '자세히'}
          </button>
        )}
        {p.links.length > 0 && (
          <ul className="link-row">
            {p.links.map((l) => (
              <li key={l.url}>
                <a href={l.url} target="_blank" rel="noopener noreferrer">
                  {l.label}
                </a>
              </li>
            ))}
          </ul>
        )}
        {owner && (
          <>
            <span className="grow" />
            {!p.is_public && <span className="badge muted no-print">비공개</span>}
            <button type="button" className="text-btn no-print" onClick={onEdit}>
              편집
            </button>
          </>
        )}
      </div>
    </li>
  );
};

interface ProjectEditorProps {
  /** null 이면 새로 만들기 */
  project: Project | null;
  kind: ProjectKind;
  nextOrder: number;
  onClose: () => void;
  onSaved: () => void;
}

const toInput = (p: Project): ProjectInput => {
  const { id: _id, created_at: _c, updated_at: _u, ...input } = p;
  return input;
};

const ProjectEditor = ({ project, kind, nextOrder, onClose, onSaved }: ProjectEditorProps) => {
  const [draft, setDraft] = useState<ProjectInput>(
    project
      ? toInput(project)
      : {
          kind, title: '', organization: '', role: '', started_on: '', ended_on: '', summary: '', description: '',
          tech_stack: [], links: [], is_public: true, sort_order: nextOrder,
        },
  );
  const set = <K extends keyof ProjectInput>(key: K, value: ProjectInput[K]) => setDraft((d) => ({ ...d, [key]: value }));

  const save = async () => {
    if (project) await api.updateProject(project.id, draft);
    else await api.createProject(draft);
    onSaved();
  };
  const remove = async () => {
    if (!project || !confirm(`‘${project.title}’ 프로젝트를 지울까요?`)) return;
    await api.deleteProject(project.id);
    onSaved();
  };

  return (
    <Modal
      title={project ? '프로젝트 편집' : '새 프로젝트'}
      onClose={onClose}
      onSubmit={save}
      extra={
        project && (
          <button type="button" className="btn danger small" onClick={() => void remove()}>
            지우기
          </button>
        )
      }
    >
      <div className="segmented" role="radiogroup" aria-label="구분">
        {KINDS.map((k) => (
          <button key={k.kind} type="button" role="radio" aria-checked={draft.kind === k.kind} onClick={() => set('kind', k.kind)}>
            {k.label}
          </button>
        ))}
      </div>
      <Field label="프로젝트 이름" value={draft.title} onChange={(v) => set('title', v)} required maxLength={200} />
      <div className="grid-2">
        <Field
          label={draft.kind === 'work' ? '회사 · 고객사' : '팀 · 소속'}
          value={draft.organization}
          onChange={(v) => set('organization', v)}
        />
        <Field label="역할" value={draft.role} onChange={(v) => set('role', v)} placeholder="백엔드 리드" />
        <Field label="시작" type="month" value={draft.started_on} onChange={(v) => set('started_on', v)} />
        <Field label="종료" type="month" value={draft.ended_on} onChange={(v) => set('ended_on', v)} hint="비우면 진행 중" />
      </div>
      <Field label="한 줄 요약" value={draft.summary} onChange={(v) => set('summary', v)} maxLength={300} />
      <TextArea
        label="상세 설명"
        value={draft.description}
        onChange={(v) => set('description', v)}
        rows={6}
        placeholder={'- 맡은 일\n- 성과 (수치로)\n- 배운 점'}
      />
      <WordsField label="기술 스택" value={draft.tech_stack} onChange={(v) => set('tech_stack', v)} placeholder="Go, PostgreSQL, React" />
      <RowList<Link>
        title="링크"
        rows={draft.links}
        onChange={(rows) => set('links', rows)}
        empty={() => ({ label: '', url: '' })}
        addLabel="링크"
        render={(row, update) => (
          <div className="grid-2">
            <Field label="이름" value={row.label} onChange={(v) => update({ label: v })} placeholder="GitHub · 데모" />
            <Field label="주소" type="url" value={row.url} onChange={(v) => update({ url: v })} placeholder="https://" />
          </div>
        )}
      />
      <div className="grid-2">
        <Toggle label="공개" checked={draft.is_public} onChange={(v) => set('is_public', v)} hint="끄면 나만 봐요" />
        <Field
          label="순서"
          type="number"
          value={String(draft.sort_order)}
          onChange={(v) => set('sort_order', Number(v) || 0)}
          hint="작을수록 위에 (같으면 최근 시작 순)"
        />
      </div>
    </Modal>
  );
};
