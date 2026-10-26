import { useEffect, useState } from 'react';
import { Link as RouterLink, useLocation, useSearchParams } from 'react-router-dom';
import { Field, Modal, RowList, TextArea, Toggle, WordsField } from '../components/form';
import { Identity, usePage } from '../components/Layout';
import { Section } from '../components/Section';
import { PageHead, useWorkspace } from '../components/Workspace';
import { api } from '../lib/api';
import { durationLabel, monthLabel, periodLabel } from '../lib/format';
import type { Education, Experience, Link, Profile, PublicPage, SkillGroup } from '../lib/types';
import { ContactSection } from './Contact';
import { ProjectsSection } from './Projects';

/** /:slug — 방문자가 보는 이력서 본문 (이름 · 연락처는 왼쪽 칸). 고치기는 작업 공간에서 */
export const About = () => {
  const { page, reload } = usePage();
  const { hash } = useLocation();

  // 예전 주소(/projects · /contact)나 #contact 로 오면 그 섹션으로
  useEffect(() => {
    if (hash) document.getElementById(hash.slice(1))?.scrollIntoView();
  }, [hash]);

  return (
    <article className="resume">
      <ResumeBody page={page} reload={reload} />
      <ContactSection profile={page.profile} />
    </article>
  );
};

/** /me/resume — 내 이력서: 방문자와 같은 본문 + 편집 · PDF · 링크 복사 */
export const ResumePage = () => {
  const { page, setPage, reload } = useWorkspace();
  const [params, setParams] = useSearchParams();
  const [editing, setEditing] = useState(params.get('edit') === '1');
  const [copied, setCopied] = useState(false);
  const p = page.profile;
  const url = `${location.origin}${import.meta.env.BASE_URL}${p.slug}`;

  // 다른 화면의 '이력서 채우기'는 ?edit=1 로 온다
  useEffect(() => {
    if (params.has('edit')) setParams({}, { replace: true });
  }, [params, setParams]);

  const copy = async () => {
    await navigator.clipboard.writeText(url);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <>
      <PageHead
        title="이력서"
        sub={
          <>
            <span className={`badge ${p.is_public ? 'ok' : 'muted'}`}>{p.is_public ? '링크로 공개 중' : '비공개'}</span>
            <a href={url} target="_blank" rel="noopener noreferrer" className="plain-link">
              {url.replace(/^https?:\/\//, '')}
            </a>
          </>
        }
      >
        <button type="button" className="btn ghost small" onClick={() => void copy()}>
          {copied ? '복사했어요' : '링크 복사'}
        </button>
        <button type="button" className="btn ghost small" onClick={() => window.print()}>
          PDF로 저장
        </button>
        <button type="button" className="btn primary small" onClick={() => setEditing(true)}>
          이력서 편집
        </button>
      </PageHead>

      <article className="resume in-sheet">
        <div className="resume-id">
          <Identity profile={p} />
        </div>
        <ResumeBody page={page} reload={reload} onEdit={() => setEditing(true)} />
      </article>

      {editing && (
        <ProfileEditor
          profile={p}
          onClose={() => setEditing(false)}
          onSaved={(next) => {
            setEditing(false);
            setPage({ ...page, profile: next });
          }}
        />
      )}
    </>
  );
};

interface ResumeBodyProps {
  page: PublicPage;
  reload: () => void;
  /** 있으면 주인의 편집 도구를 보인다 (작업 공간) */
  onEdit?: () => void;
}

/** 소개 · 경력 · 프로젝트 · 학력 · 기술 · 자격 */
const ResumeBody = ({ page, reload, onEdit }: ResumeBodyProps) => {
  const p = page.profile;
  const r = p.resume;
  // 이력서에는 취득한 자격만 (준비 중인 자격은 작업 공간의 자격증 탭에서)
  const certs = r.certificates.filter((c) => c.status !== 'preparing');
  const blank = !p.summary && !r.experiences.length && !r.educations.length && !r.skills.length && !certs.length;

  const editButton = onEdit && (
    <button type="button" className="text-btn no-print" onClick={onEdit}>
      편집
    </button>
  );

  return (
    <>
      {blank && (
        <Section title="소개">
          <p className="muted">{onEdit ? '아직 비어 있어요. 소개와 경력부터 채워 보세요.' : '아직 준비 중인 페이지예요.'}</p>
          {onEdit && (
            <button type="button" className="btn primary small no-print" style={{ marginTop: 12 }} onClick={onEdit}>
              이력서 채우기
            </button>
          )}
        </Section>
      )}

      {p.summary && (
        <Section title="소개" tools={editButton}>
          <p className="prose">{p.summary}</p>
        </Section>
      )}

      {r.experiences.length > 0 && (
        <Section title="경력" tools={editButton}>
          <ol className="timeline">
            {r.experiences.map((e, i) => (
              <li key={i}>
                <div className="tl-when">
                  <span className="num">{periodLabel(e.started_on, e.ended_on, '재직 중')}</span>
                  <small>{durationLabel(e.started_on, e.ended_on)}</small>
                </div>
                <div className="tl-what">
                  <h3>{e.company}</h3>
                  {e.role && <p className="sub">{e.role}</p>}
                  {e.description && <p className="prose">{e.description}</p>}
                </div>
              </li>
            ))}
          </ol>
        </Section>
      )}

      <ProjectsSection page={page} reload={reload} editable={!!onEdit} />

      {r.educations.length > 0 && (
        <Section title="학력" tools={editButton}>
          <ol className="timeline">
            {r.educations.map((e, i) => (
              <li key={i}>
                <div className="tl-when">
                  <span className="num">{periodLabel(e.started_on, e.ended_on, '재학 중')}</span>
                </div>
                <div className="tl-what">
                  <h3>{e.school}</h3>
                  <p className="sub">{[e.major, e.degree].filter(Boolean).join(', ')}</p>
                </div>
              </li>
            ))}
          </ol>
        </Section>
      )}

      {r.skills.length > 0 && (
        <Section title="기술" tools={editButton}>
          <dl className="skills">
            {r.skills.map((s, i) => (
              <div key={i}>
                <dt>{s.category || '기타'}</dt>
                <dd>
                  {s.items.map((item) => (
                    <span className="chip" key={item}>
                      {item}
                    </span>
                  ))}
                </dd>
              </div>
            ))}
          </dl>
        </Section>
      )}

      {certs.length > 0 && (
        <Section
          title="자격 · 수상"
          tools={
            onEdit && (
              <RouterLink className="text-btn no-print" to="/me/certs">
                자격증에서 관리
              </RouterLink>
            )
          }
        >
          <ul className="cert-lines">
            {certs.map((c, i) => (
              <li key={i}>
                <strong>{c.name}</strong>
                {c.grade && <span>{c.grade}</span>}
                <span className="muted">{c.issuer}</span>
                {c.acquired_on && <span className="muted num">{monthLabel(c.acquired_on)}</span>}
              </li>
            ))}
          </ul>
        </Section>
      )}
    </>
  );
};

interface ProfileEditorProps {
  profile: Profile;
  onClose: () => void;
  onSaved: (next: Profile) => void;
}

const emptyExperience = (): Experience => ({ company: '', role: '', started_on: '', ended_on: '', description: '' });
const emptyEducation = (): Education => ({ school: '', major: '', degree: '', started_on: '', ended_on: '' });
const emptySkill = (): SkillGroup => ({ category: '', items: [] });
const emptyLink = (): Link => ({ label: '', url: '' });

const ProfileEditor = ({ profile, onClose, onSaved }: ProfileEditorProps) => {
  const [draft, setDraft] = useState<Profile>(profile);
  const set = <K extends keyof Profile>(key: K, value: Profile[K]) => setDraft((d) => ({ ...d, [key]: value }));
  const setResume = <K extends keyof Profile['resume']>(key: K, value: Profile['resume'][K]) =>
    setDraft((d) => ({ ...d, resume: { ...d.resume, [key]: value } }));

  const save = async () => {
    const { updated_at: _ignored, ...body } = draft;
    onSaved(await api.updateProfile(body));
  };

  return (
    <Modal title="이력서 편집" onClose={onClose} onSubmit={save}>
      <fieldset>
        <legend>공개</legend>
        <Toggle
          label="링크로 공개"
          checked={draft.is_public}
          onChange={(v) => set('is_public', v)}
          hint="끄면 나만 볼 수 있어요 (프로젝트 · 연락하기 포함)"
        />
        <Field
          label="공개 주소"
          value={draft.slug}
          onChange={(v) => set('slug', v.toLowerCase())}
          hint={`${location.origin}${import.meta.env.BASE_URL}${draft.slug || '…'} · 영문 소문자 · 숫자 · 하이픈 3~32자`}
          required
        />
      </fieldset>

      <fieldset>
        <legend>인적 사항</legend>
        <div className="grid-2">
          <Field label="이름" value={draft.name} onChange={(v) => set('name', v)} maxLength={40} />
          <Field label="한 줄 소개" value={draft.headline} onChange={(v) => set('headline', v)} placeholder="백엔드 개발자 · 8년차" maxLength={80} />
          <Field label="지역" value={draft.location} onChange={(v) => set('location', v)} placeholder="서울" maxLength={40} />
          <Field label="사진 주소" type="url" value={draft.photo_url} onChange={(v) => set('photo_url', v)} placeholder="https://…" />
          <Field label="공개 이메일" type="email" value={draft.public_email} onChange={(v) => set('public_email', v)} hint="비우면 보이지 않아요" />
          <Field label="공개 전화번호" type="tel" value={draft.public_phone} onChange={(v) => set('public_phone', v)} hint="비우면 보이지 않아요" />
        </div>
        <TextArea label="소개" value={draft.summary} onChange={(v) => set('summary', v)} rows={5} maxLength={3000} />
      </fieldset>

      <RowList
        title="링크"
        rows={draft.links}
        onChange={(rows) => set('links', rows)}
        empty={emptyLink}
        addLabel="링크"
        render={(row, update) => (
          <div className="grid-2">
            <Field label="이름" value={row.label} onChange={(v) => update({ label: v })} placeholder="GitHub" />
            <Field label="주소" type="url" value={row.url} onChange={(v) => update({ url: v })} placeholder="https://" />
          </div>
        )}
      />

      <RowList
        title="경력"
        rows={draft.resume.experiences}
        onChange={(rows) => setResume('experiences', rows)}
        empty={emptyExperience}
        addLabel="경력"
        render={(row, update) => (
          <>
            <div className="grid-2">
              <Field label="회사" value={row.company} onChange={(v) => update({ company: v })} />
              <Field label="직무 · 직급" value={row.role} onChange={(v) => update({ role: v })} />
              <Field label="시작" type="month" value={row.started_on} onChange={(v) => update({ started_on: v })} />
              <Field label="종료" type="month" value={row.ended_on} onChange={(v) => update({ ended_on: v })} hint="비우면 재직 중" />
            </div>
            <TextArea label="주요 업무" value={row.description} onChange={(v) => update({ description: v })} rows={3} />
          </>
        )}
      />

      <RowList
        title="학력"
        rows={draft.resume.educations}
        onChange={(rows) => setResume('educations', rows)}
        empty={emptyEducation}
        addLabel="학력"
        render={(row, update) => (
          <div className="grid-2">
            <Field label="학교" value={row.school} onChange={(v) => update({ school: v })} />
            <Field label="전공" value={row.major} onChange={(v) => update({ major: v })} />
            <Field label="학위" value={row.degree} onChange={(v) => update({ degree: v })} placeholder="학사" />
            <span />
            <Field label="입학" type="month" value={row.started_on} onChange={(v) => update({ started_on: v })} />
            <Field label="졸업" type="month" value={row.ended_on} onChange={(v) => update({ ended_on: v })} hint="비우면 재학 중" />
          </div>
        )}
      />

      <RowList
        title="기술"
        rows={draft.resume.skills}
        onChange={(rows) => setResume('skills', rows)}
        empty={emptySkill}
        addLabel="기술 분류"
        render={(row, update) => (
          <div className="grid-2">
            <Field label="분류" value={row.category} onChange={(v) => update({ category: v })} placeholder="Backend" />
            <WordsField label="기술" value={row.items} onChange={(v) => update({ items: v })} placeholder="Go, Java, PostgreSQL" />
          </div>
        )}
      />

      <p className="muted">자격증은 ‘자격증’ 탭에서 따로 관리해요. 취득한 것만 이력서에 보여요.</p>
    </Modal>
  );
};
