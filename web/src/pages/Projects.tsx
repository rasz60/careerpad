import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Field, Modal, RowList, TextArea, Toggle, WordsField } from '../components/form';
import { usePage } from '../components/Layout';
import { api } from '../lib/api';
import { periodLabel } from '../lib/format';
import type { Link, Project, ProjectInput, ProjectKind } from '../lib/types';

const KINDS: { kind: ProjectKind; label: string }[] = [
  { kind: 'work', label: '업무 프로젝트' },
  { kind: 'personal', label: '개인 프로젝트' },
];

/** project: 내가 했던 일. 업무 / 개인 탭으로 나눈다 (?kind= 로 링크를 걸 수 있다) */
export const Projects = () => {
  const { page, reload } = usePage();
  const [params, setParams] = useSearchParams();
  const kind: ProjectKind = params.get('kind') === 'personal' ? 'personal' : 'work';
  const [editing, setEditing] = useState<Project | 'new' | null>(null);
  const [open, setOpen] = useState<string | null>(null);

  const list = page.projects.filter((p) => p.kind === kind);
  const counts = { work: 0, personal: 0 } satisfies Record<ProjectKind, number>;
  page.projects.forEach((p) => counts[p.kind]++);

  return (
    <section className="projects">
      <div className="page-tools">
        <div className="segmented" role="tablist" aria-label="프로젝트 구분">
          {KINDS.map((k) => (
            <button
              key={k.kind}
              type="button"
              role="tab"
              aria-selected={kind === k.kind}
              onClick={() => setParams(k.kind === 'work' ? {} : { kind: k.kind }, { replace: true })}
            >
              {k.label} <span className="count">{counts[k.kind]}</span>
            </button>
          ))}
        </div>
        <span className="grow" />
        {page.is_owner && (
          <button type="button" className="btn primary small" onClick={() => setEditing('new')}>
            + 프로젝트
          </button>
        )}
      </div>

      {list.length === 0 ? (
        <p className="empty">{page.is_owner ? '아직 없어요. ‘+ 프로젝트’로 추가해 보세요.' : '등록된 프로젝트가 없습니다.'}</p>
      ) : (
        <ul className="project-list">
          {list.map((p) => (
            <li key={p.id} className={`project-card ${p.is_public ? '' : 'is-private'}`}>
              <button
                type="button"
                className="project-head"
                aria-expanded={open === p.id}
                onClick={() => setOpen(open === p.id ? null : p.id)}
              >
                <span className="project-when">{periodLabel(p.started_on, p.ended_on, '진행 중')}</span>
                <strong>{p.title}</strong>
                <span className="sub">{[p.organization, p.role].filter(Boolean).join(' · ')}</span>
                {p.summary && <span className="project-summary">{p.summary}</span>}
                {p.tech_stack.length > 0 && (
                  <span className="chips">
                    {p.tech_stack.map((t) => (
                      <span className="chip" key={t}>
                        {t}
                      </span>
                    ))}
                  </span>
                )}
              </button>
              {open === p.id && (
                <div className="project-body">
                  {p.description ? <p className="prose">{p.description}</p> : <p className="muted">상세 설명이 없습니다.</p>}
                  {p.links.length > 0 && (
                    <ul className="link-row">
                      {p.links.map((l) => (
                        <li key={l.url}>
                          <a href={l.url} target="_blank" rel="noopener noreferrer">
                            {l.label} ↗
                          </a>
                        </li>
                      ))}
                    </ul>
                  )}
                  {page.is_owner && (
                    <div className="row-end">
                      {!p.is_public && <span className="badge muted">비공개</span>}
                      <button type="button" className="btn ghost small" onClick={() => setEditing(p)}>
                        편집
                      </button>
                    </div>
                  )}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}

      {editing && (
        <ProjectEditor
          project={editing === 'new' ? null : editing}
          kind={kind}
          nextOrder={list.length}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            reload();
          }}
        />
      )}
    </section>
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
