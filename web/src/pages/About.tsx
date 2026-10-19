import { useState } from 'react';
import { Field, Modal, RowList, TextArea, Toggle, WordsField } from '../components/form';
import { usePage } from '../components/Layout';
import { api } from '../lib/api';
import { durationLabel, monthLabel, periodLabel, phoneLabel } from '../lib/format';
import type { Certificate, Education, Experience, Link, Profile, SkillGroup } from '../lib/types';

/** about me: 기본 이력서 양식 (인적 사항 · 소개 · 경력 · 학력 · 기술 · 자격) */
export const About = () => {
  const { page, setPage } = usePage();
  const [editing, setEditing] = useState(false);
  const p = page.profile;
  const r = p.resume;
  const blank = !p.summary && !r.experiences.length && !r.educations.length && !r.skills.length && !r.certificates.length;

  return (
    <article className="resume">
      {page.is_owner && (
        <div className="page-tools no-print">
          {!p.is_public && <p className="note">비공개 상태예요. 편집에서 ‘공개’를 켜면 링크로 누구나 볼 수 있어요.</p>}
          <span className="grow" />
          <button type="button" className="btn ghost small" onClick={() => window.print()}>
            PDF로 저장
          </button>
          <button type="button" className="btn primary small" onClick={() => setEditing(true)}>
            편집
          </button>
        </div>
      )}

      <section className="resume-head">
        {p.photo_url && <img className="resume-photo" src={p.photo_url} alt={`${p.name} 사진`} />}
        <div>
          <h1>{p.name || p.slug}</h1>
          {p.headline && <p className="headline">{p.headline}</p>}
          <ul className="contact-line">
            {p.location && <li>{p.location}</li>}
            {p.public_email && (
              <li>
                <a href={`mailto:${p.public_email}`}>{p.public_email}</a>
              </li>
            )}
            {p.public_phone && (
              <li>
                <a href={`tel:${p.public_phone}`}>{phoneLabel(p.public_phone)}</a>
              </li>
            )}
            {p.links.map((l) => (
              <li key={l.url}>
                <a href={l.url} target="_blank" rel="noopener noreferrer">
                  {l.label}
                </a>
              </li>
            ))}
          </ul>
        </div>
      </section>

      {blank && (
        <p className="empty">{page.is_owner ? '아직 비어 있어요. ‘편집’을 눌러 이력서를 채워 보세요.' : '아직 준비 중인 페이지입니다.'}</p>
      )}

      {p.summary && (
        <section className="resume-section">
          <h2>소개</h2>
          <p className="prose">{p.summary}</p>
        </section>
      )}

      {r.experiences.length > 0 && (
        <section className="resume-section">
          <h2>경력</h2>
          <ol className="timeline">
            {r.experiences.map((e, i) => (
              <li key={i}>
                <div className="tl-when">
                  {periodLabel(e.started_on, e.ended_on, '재직 중')}
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
        </section>
      )}

      {r.educations.length > 0 && (
        <section className="resume-section">
          <h2>학력</h2>
          <ol className="timeline">
            {r.educations.map((e, i) => (
              <li key={i}>
                <div className="tl-when">{periodLabel(e.started_on, e.ended_on, '재학 중')}</div>
                <div className="tl-what">
                  <h3>{e.school}</h3>
                  <p className="sub">{[e.major, e.degree].filter(Boolean).join(' · ')}</p>
                </div>
              </li>
            ))}
          </ol>
        </section>
      )}

      {r.skills.length > 0 && (
        <section className="resume-section">
          <h2>기술</h2>
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
        </section>
      )}

      {r.certificates.length > 0 && (
        <section className="resume-section">
          <h2>자격 · 수상</h2>
          <ul className="plain-list">
            {r.certificates.map((c, i) => (
              <li key={i}>
                <strong>{c.name}</strong>
                <span className="muted">{[c.issuer, c.acquired_on && monthLabel(c.acquired_on)].filter(Boolean).join(' · ')}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {editing && (
        <ProfileEditor
          profile={p}
          onClose={() => setEditing(false)}
          onSaved={(next) => {
            setEditing(false);
            setPage({ ...page, profile: next });
            // 주소를 바꿨으면 새 주소로
            if (next.slug !== p.slug) location.replace(`${import.meta.env.BASE_URL}${next.slug}`);
          }}
        />
      )}
    </article>
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
const emptyCertificate = (): Certificate => ({ name: '', issuer: '', acquired_on: '' });
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
    <Modal title="About me 편집" onClose={onClose} onSubmit={save}>
      <fieldset>
        <legend>공개</legend>
        <Toggle
          label="링크로 공개"
          checked={draft.is_public}
          onChange={(v) => set('is_public', v)}
          hint="끄면 나만 볼 수 있어요 (Project · Contact 포함)"
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

      <RowList
        title="자격 · 수상"
        rows={draft.resume.certificates}
        onChange={(rows) => setResume('certificates', rows)}
        empty={emptyCertificate}
        addLabel="자격 · 수상"
        render={(row, update) => (
          <div className="grid-3">
            <Field label="이름" value={row.name} onChange={(v) => update({ name: v })} />
            <Field label="발급 기관" value={row.issuer} onChange={(v) => update({ issuer: v })} />
            <Field label="취득" type="month" value={row.acquired_on} onChange={(v) => update({ acquired_on: v })} />
          </div>
        )}
      />
    </Modal>
  );
};
