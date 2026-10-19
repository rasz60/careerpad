import { useState, type FormEvent } from 'react';
import { Field, TextArea, Toggle } from '../components/form';
import { usePage } from '../components/Layout';
import { api, errorText } from '../lib/api';
import { dateTimeLabel, phoneLabel } from '../lib/format';
import type { ContactInput, ContactSettings, Message } from '../lib/types';
import { useAsync } from '../lib/useAsync';

/** contact: 방문자 → 나. 공개 연락처(메일 · 전화 · 문자) + 메시지 남기기. 주인은 알림 설정과 받은 메시지 */
export const Contact = () => {
  const { page } = usePage();
  const p = page.profile;

  return (
    <section className="contact">
      <div className="contact-grid">
        <div>
          <h2>연락하기</h2>
          <p className="muted">
            {p.name || p.slug} 님에게 메시지를 남기면 바로 알림이 갑니다. 답장 받을 연락처를 꼭 적어 주세요.
          </p>
          {(p.public_email || p.public_phone) && (
            <ul className="contact-points">
              {p.public_email && (
                <li>
                  <span>메일</span>
                  <a href={`mailto:${p.public_email}`}>{p.public_email}</a>
                </li>
              )}
              {p.public_phone && (
                <li>
                  <span>전화 · 문자</span>
                  <a href={`tel:${p.public_phone}`}>{phoneLabel(p.public_phone)}</a>
                  <a className="btn ghost small" href={`sms:${p.public_phone}`}>
                    문자 보내기
                  </a>
                </li>
              )}
            </ul>
          )}
        </div>
        <ContactForm slug={p.slug} disabled={!p.is_public} />
      </div>

      {page.is_owner && (
        <>
          <NotifySettings />
          <Inbox />
        </>
      )}
    </section>
  );
};

interface ContactFormProps {
  slug: string;
  /** 비공개 페이지에는 메시지를 받을 수 없다 (주인 미리보기) */
  disabled: boolean;
}

const emptyInput = (): ContactInput => ({ name: '', email: '', phone: '', message: '', website: '' });

const ContactForm = ({ slug, disabled }: ContactFormProps) => {
  const [input, setInput] = useState<ContactInput>(emptyInput);
  const [state, setState] = useState<'idle' | 'sending' | 'sent'>('idle');
  const [error, setError] = useState('');
  const set = <K extends keyof ContactInput>(key: K, value: ContactInput[K]) => setInput((d) => ({ ...d, [key]: value }));

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!input.email.trim() && !input.phone.trim()) {
      setError('답장 받을 이메일이나 전화번호 중 하나는 적어 주세요.');
      return;
    }
    setState('sending');
    setError('');
    try {
      await api.sendContact(slug, input);
      setState('sent');
      setInput(emptyInput());
    } catch (err) {
      setState('idle');
      setError(errorText(err));
    }
  };

  if (state === 'sent') {
    return (
      <div className="card sent">
        <h3>보냈어요 ✓</h3>
        <p className="muted">확인하는 대로 남겨 주신 연락처로 답장드릴게요.</p>
        <button type="button" className="btn ghost small" onClick={() => setState('idle')}>
          하나 더 보내기
        </button>
      </div>
    );
  }

  return (
    <form className="card contact-form" onSubmit={submit}>
      <Field label="이름 · 회사" value={input.name} onChange={(v) => set('name', v)} required maxLength={60} />
      <div className="grid-2">
        <Field label="이메일" type="email" value={input.email} onChange={(v) => set('email', v)} />
        <Field label="전화번호" type="tel" value={input.phone} onChange={(v) => set('phone', v)} placeholder="010-0000-0000" />
      </div>
      <TextArea label="내용" value={input.message} onChange={(v) => set('message', v)} rows={6} required maxLength={2000} />
      {/* 봇 걸러내기: 사람에게는 안 보이는 칸. 채워져 오면 서버가 조용히 버린다 */}
      <input
        className="hp"
        tabIndex={-1}
        autoComplete="off"
        aria-hidden="true"
        value={input.website}
        onChange={(e) => set('website', e.target.value)}
        name="website"
      />
      {error && <p className="error">{error}</p>}
      {disabled && <p className="note">비공개 페이지라 지금은 메시지를 받을 수 없어요 (About me 편집에서 공개).</p>}
      <button type="submit" className="btn primary" disabled={disabled || state === 'sending'}>
        {state === 'sending' ? '보내는 중…' : '메시지 보내기'}
      </button>
    </form>
  );
};

/** 주인: 메시지를 어디로 알릴지 */
const NotifySettings = () => {
  const settings = useAsync(() => api.contactSettings(), []);
  const [draft, setDraft] = useState<ContactSettings | null>(null);
  const [note, setNote] = useState('');
  const [busy, setBusy] = useState(false);

  if (!settings.data) return settings.error ? <p className="error">{errorText(settings.error)}</p> : null;
  const { available, ...saved } = settings.data;
  const value = draft ?? saved;
  const set = <K extends keyof ContactSettings>(key: K, v: ContactSettings[K]) => setDraft({ ...value, [key]: v });

  const save = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setNote('');
    try {
      settings.setData(await api.saveContactSettings(value));
      setDraft(null);
      setNote('저장했어요.');
    } catch (err) {
      setNote(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form className="card owner-card" onSubmit={save}>
      <h3>알림 받을 곳 <span className="badge muted">나만 보여요</span></h3>
      <div className="grid-2">
        <div>
          <Toggle
            label="메일로 알림"
            checked={value.mail_enabled}
            onChange={(v) => set('mail_enabled', v)}
            disabled={!available.mail}
            hint={available.mail ? undefined : '서버에 메일(SMTP) 설정이 없어요'}
          />
          <Field label="받을 이메일" type="email" value={value.notify_email} onChange={(v) => set('notify_email', v)} />
        </div>
        <div>
          <Toggle
            label="문자(SMS)로 알림"
            checked={value.sms_enabled}
            onChange={(v) => set('sms_enabled', v)}
            disabled={!available.sms}
            hint={available.sms ? '하루 발송 한도가 있어요' : '서버에 문자(솔라피) 설정이 없어요'}
          />
          <Field label="받을 휴대폰" type="tel" value={value.notify_phone} onChange={(v) => set('notify_phone', v)} />
        </div>
      </div>
      <div className="row-end">
        {note && <span className="muted">{note}</span>}
        <button type="submit" className="btn primary small" disabled={busy || !draft}>
          저장
        </button>
      </div>
    </form>
  );
};

/** 주인: 받은 메시지 (알림이 실패해도 여기에는 남는다) */
const Inbox = () => {
  const messages = useAsync(() => api.messages(), []);
  const list = messages.data ?? [];
  const unread = list.filter((m) => !m.is_read).length;

  const patch = (id: string, change: Partial<Message> | null) =>
    messages.setData(change ? list.map((m) => (m.id === id ? { ...m, ...change } : m)) : list.filter((m) => m.id !== id));

  const toggleRead = async (m: Message) => {
    await api.markMessage(m.id, !m.is_read);
    patch(m.id, { is_read: !m.is_read });
  };
  const remove = async (m: Message) => {
    if (!confirm(`${m.sender_name} 님의 메시지를 지울까요?`)) return;
    await api.deleteMessage(m.id);
    patch(m.id, null);
  };

  return (
    <section className="card owner-card">
      <h3>
        받은 메시지 {unread > 0 && <span className="badge warn">새 메시지 {unread}</span>}
      </h3>
      {messages.error ? <p className="error">{errorText(messages.error)}</p> : null}
      {messages.data && list.length === 0 && <p className="muted">아직 받은 메시지가 없어요.</p>}
      <ul className="inbox">
        {list.map((m) => (
          <li key={m.id} className={m.is_read ? '' : 'is-unread'}>
            <header>
              <strong>{m.sender_name}</strong>
              {m.sender_email && <a href={`mailto:${m.sender_email}`}>{m.sender_email}</a>}
              {m.sender_phone && <a href={`tel:${m.sender_phone}`}>{phoneLabel(m.sender_phone)}</a>}
              <span className="grow" />
              <small className="muted">
                {dateTimeLabel(m.created_at)}
                {m.delivered_by.length > 0 ? ` · ${m.delivered_by.map((c) => (c === 'sms' ? '문자' : '메일')).join('·')} 알림` : ' · 알림 없음'}
              </small>
            </header>
            <p className="prose">{m.body}</p>
            <div className="row-end">
              <button type="button" className="btn ghost small" onClick={() => void toggleRead(m)}>
                {m.is_read ? '안 읽음으로' : '읽음'}
              </button>
              <button type="button" className="btn danger small" onClick={() => void remove(m)}>
                지우기
              </button>
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
};
