import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Field, Modal, TextArea } from '../components/form';
import { PageHead, useWorkspace } from '../components/Workspace';
import { api } from '../lib/api';
import { dayLabel, daysUntil, monthLabel } from '../lib/format';
import type { Certificate, CertStatus } from '../lib/types';

/** 유효기간이 이만큼 남으면 경고 */
const EXPIRY_WARN_DAYS = 90;

const STATUSES: { status: CertStatus; label: string }[] = [
  { status: 'preparing', label: '준비 중' },
  { status: 'acquired', label: '취득' },
];

const emptyCertificate = (status: CertStatus): Certificate => ({
  name: '', issuer: '', acquired_on: '', status, grade: '', cert_no: '', exam_on: '', expires_on: '', memo: '',
});

interface Editing {
  /** null 이면 새로 */
  index: number | null;
  draft: Certificate;
}

/** /me/certs — 준비 중인 자격(시험일 D-day)과 취득한 자격(유효기간). 취득한 것만 공개 이력서에 나간다 */
export const Certificates = () => {
  const { page, setPage } = useWorkspace();
  const [params, setParams] = useSearchParams();
  const [editing, setEditing] = useState<Editing | null>(
    params.get('new') === '1' ? { index: null, draft: emptyCertificate('preparing') } : null,
  );
  const certs = page.profile.resume.certificates;

  // 오늘 화면의 '준비 중인 자격증 추가'는 ?new=1 로 온다
  useEffect(() => {
    if (params.has('new')) setParams({}, { replace: true });
  }, [params, setParams]);

  const rows = certs.map((c, index) => ({ c, index }));
  const preparing = rows
    .filter((r) => r.c.status === 'preparing')
    .sort((a, b) => (a.c.exam_on || '9999').localeCompare(b.c.exam_on || '9999'));
  const acquired = rows
    .filter((r) => r.c.status !== 'preparing')
    .sort((a, b) => b.c.acquired_on.localeCompare(a.c.acquired_on));

  const save = async (next: Certificate[]) => {
    const { updated_at: _ignored, ...body } = page.profile;
    const saved = await api.updateProfile({ ...body, resume: { ...body.resume, certificates: next } });
    setPage({ ...page, profile: saved });
    setEditing(null);
  };
  const edit = (index: number, patch: Partial<Certificate> = {}) => setEditing({ index, draft: { ...certs[index]!, ...patch } });

  return (
    <>
      <PageHead title="자격증" sub="취득한 자격은 공개 이력서에 보여요. 준비 중인 자격, 자격번호, 메모는 나만 봐요.">
        <button type="button" className="btn primary small" onClick={() => setEditing({ index: null, draft: emptyCertificate('preparing') })}>
          자격증 추가
        </button>
      </PageHead>

      <section className="cert-group" aria-labelledby="prep-title">
        <h2 id="prep-title">
          준비 중 <span className="count">{preparing.length}</span>
        </h2>
        {preparing.length === 0 ? (
          <p className="empty">준비하는 시험이 있으면 시험일과 함께 적어 두세요. 오늘 화면에서 남은 날을 세어 줘요.</p>
        ) : (
          <ul className="prep-list">
            {preparing.map(({ c, index }) => (
              <PrepRow
                key={index}
                cert={c}
                onEdit={() => edit(index)}
                onPassed={() => edit(index, { status: 'acquired', acquired_on: (c.exam_on || '').slice(0, 7) })}
              />
            ))}
          </ul>
        )}
      </section>

      <section className="cert-group" aria-labelledby="got-title">
        <h2 id="got-title">
          취득 <span className="count">{acquired.length}</span>
        </h2>
        {acquired.length === 0 ? (
          <p className="empty">아직 기록한 자격이 없어요. 자격증이나 어학 점수, 수상 이력을 적어 두면 이력서에도 들어가요.</p>
        ) : (
          <table className="cert-table">
            <thead>
              <tr>
                <th scope="col">자격</th>
                <th scope="col">점수 · 등급</th>
                <th scope="col">발급 기관</th>
                <th scope="col">취득</th>
                <th scope="col">유효기간</th>
                <th scope="col">
                  <span className="sr-only">편집</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {acquired.map(({ c, index }) => (
                <tr key={index}>
                  <th scope="row" data-label="자격">
                    {c.name}
                    {c.cert_no && <small className="muted">자격번호 {c.cert_no}</small>}
                    {c.memo && <small className="memo">{c.memo}</small>}
                  </th>
                  <td data-label="점수 · 등급">{c.grade || <span className="muted">-</span>}</td>
                  <td data-label="발급 기관">{c.issuer || <span className="muted">-</span>}</td>
                  <td data-label="취득" className="num">
                    {c.acquired_on ? monthLabel(c.acquired_on) : <span className="muted">-</span>}
                  </td>
                  <td data-label="유효기간">
                    <Expiry date={c.expires_on} />
                  </td>
                  <td className="cell-tools">
                    <button type="button" className="text-btn" onClick={() => edit(index)}>
                      편집
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      {editing && (
        <CertEditor
          editing={editing}
          onClose={() => setEditing(null)}
          onSave={(draft) =>
            save(editing.index === null ? [...certs, draft] : certs.map((c, i) => (i === editing.index ? draft : c)))
          }
          onDelete={
            editing.index === null
              ? undefined
              : async () => {
                  if (!confirm(`‘${certs[editing.index!]!.name}’을(를) 지울까요?`)) return;
                  await save(certs.filter((_, i) => i !== editing.index));
                }
          }
        />
      )}
    </>
  );
};

interface PrepRowProps {
  cert: Certificate;
  onEdit: () => void;
  onPassed: () => void;
}

const PrepRow = ({ cert: c, onEdit, onPassed }: PrepRowProps) => {
  const days = daysUntil(c.exam_on);
  const past = days !== null && days < 0;
  return (
    <li className={`prep ${days !== null && days >= 0 && days <= 7 ? 'is-soon' : ''}`}>
      <div className="prep-when">
        {days === null ? (
          <span className="dday is-unset">날짜 미정</span>
        ) : past ? (
          <span className="dday is-past">시험 끝</span>
        ) : (
          <span className="dday">{days === 0 ? 'D-day' : `D-${days}`}</span>
        )}
        {c.exam_on && <small>{dayLabel(c.exam_on)}</small>}
      </div>
      <div className="prep-main">
        <h3>{c.name}</h3>
        {(c.issuer || c.grade) && <p className="sub">{[c.issuer, c.grade && `목표 ${c.grade}`].filter(Boolean).join(', ')}</p>}
        {c.memo && <p className="memo prose">{c.memo}</p>}
        {past && <p className="quiet">시험이 끝났어요. 결과가 나오면 ‘합격 기록’으로 취득에 옮겨 두세요.</p>}
      </div>
      <div className="prep-tools">
        <button type="button" className="btn ghost small" onClick={onPassed}>
          합격 기록
        </button>
        <button type="button" className="text-btn" onClick={onEdit}>
          편집
        </button>
      </div>
    </li>
  );
};

const Expiry = ({ date }: { date: string }) => {
  const days = daysUntil(date);
  if (days === null) return <span className="muted">평생</span>;
  if (days < 0) return <span className="badge muted">만료됨</span>;
  return (
    <span className="expiry">
      <span className="num">{date.replaceAll('-', '.')}</span>
      {days <= EXPIRY_WARN_DAYS && <span className="badge warn">{days === 0 ? '오늘 끝' : `D-${days}`}</span>}
    </span>
  );
};

interface CertEditorProps {
  editing: Editing;
  onClose: () => void;
  onSave: (draft: Certificate) => Promise<void>;
  onDelete?: () => Promise<void>;
}

const CertEditor = ({ editing, onClose, onSave, onDelete }: CertEditorProps) => {
  const [draft, setDraft] = useState<Certificate>(editing.draft);
  const set = <K extends keyof Certificate>(key: K, value: Certificate[K]) => setDraft((d) => ({ ...d, [key]: value }));
  const preparing = draft.status === 'preparing';

  return (
    <Modal
      title={editing.index === null ? '자격증 추가' : '자격증 편집'}
      onClose={onClose}
      onSubmit={() => onSave(draft)}
      extra={
        onDelete && (
          <button type="button" className="btn danger small" onClick={() => void onDelete()}>
            지우기
          </button>
        )
      }
    >
      <div className="segmented" role="radiogroup" aria-label="상태">
        {STATUSES.map((s) => (
          <button key={s.status} type="button" role="radio" aria-checked={draft.status === s.status} onClick={() => set('status', s.status)}>
            {s.label}
          </button>
        ))}
      </div>
      <div className="grid-2">
        <Field label="자격 이름" value={draft.name} onChange={(v) => set('name', v)} placeholder="정보처리기사" required maxLength={60} />
        <Field label="발급 기관" value={draft.issuer} onChange={(v) => set('issuer', v)} placeholder="한국산업인력공단" maxLength={60} />
        <Field
          label={preparing ? '목표 점수 · 등급' : '점수 · 등급'}
          value={draft.grade}
          onChange={(v) => set('grade', v)}
          placeholder="TOEIC 900, 1급"
          maxLength={30}
        />
        {preparing ? (
          <Field label="시험일" type="date" value={draft.exam_on} onChange={(v) => set('exam_on', v)} hint="오늘 화면에서 남은 날을 세어 줘요" />
        ) : (
          <Field label="취득" type="month" value={draft.acquired_on} onChange={(v) => set('acquired_on', v)} />
        )}
        {!preparing && (
          <>
            <Field label="자격번호" value={draft.cert_no} onChange={(v) => set('cert_no', v)} hint="나만 봐요" maxLength={40} />
            <Field
              label="유효기간 끝"
              type="date"
              value={draft.expires_on}
              onChange={(v) => set('expires_on', v)}
              hint="어학 점수처럼 기한이 있으면. 비우면 평생"
            />
          </>
        )}
      </div>
      <TextArea
        label="메모"
        value={draft.memo}
        onChange={(v) => set('memo', v)}
        rows={3}
        placeholder={preparing ? '접수 기간, 공부 계획, 교재' : '갱신 방법, 보수교육'}
        hint="나만 봐요"
        maxLength={500}
      />
    </Modal>
  );
};
