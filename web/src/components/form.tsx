import { useEffect, useId, useState, type FormEvent, type ReactNode } from 'react';
import { splitWords } from '../lib/format';

interface FieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: 'text' | 'email' | 'tel' | 'url' | 'month' | 'date' | 'number';
  placeholder?: string;
  hint?: string;
  required?: boolean;
  maxLength?: number;
}

export const Field = ({ label, value, onChange, type = 'text', placeholder, hint, required, maxLength }: FieldProps) => {
  const id = useId();
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        type={type}
        value={value}
        placeholder={placeholder}
        required={required}
        maxLength={maxLength}
        onChange={(e) => onChange(e.target.value)}
      />
      {hint && <small>{hint}</small>}
    </div>
  );
};

interface TextAreaProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  rows?: number;
  placeholder?: string;
  hint?: string;
  required?: boolean;
  maxLength?: number;
}

export const TextArea = ({ label, value, onChange, rows = 4, placeholder, hint, required, maxLength }: TextAreaProps) => {
  const id = useId();
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <textarea
        id={id}
        rows={rows}
        value={value}
        placeholder={placeholder}
        required={required}
        maxLength={maxLength}
        onChange={(e) => onChange(e.target.value)}
      />
      {hint && <small>{hint}</small>}
    </div>
  );
};

interface ToggleProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  hint?: string;
  disabled?: boolean;
}

export const Toggle = ({ label, checked, onChange, hint, disabled }: ToggleProps) => (
  <label className={`toggle ${disabled ? 'is-disabled' : ''}`}>
    <input type="checkbox" checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)} />
    <span className="toggle-track" aria-hidden />
    <span>
      {label}
      {hint && <small>{hint}</small>}
    </span>
  </label>
);

interface WordsFieldProps {
  label: string;
  value: string[];
  onChange: (value: string[]) => void;
  placeholder?: string;
  hint?: string;
}

/** 쉼표로 나눠 쓰는 낱말 목록 (기술 스택 · 키워드). 입력 중에는 글자 그대로 두고 칸을 벗어날 때 정리한다 */
export const WordsField = ({ label, value, onChange, placeholder, hint }: WordsFieldProps) => {
  const id = useId();
  const [text, setText] = useState(value.join(', '));
  useEffect(() => setText(value.join(', ')), [value]);
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        value={text}
        placeholder={placeholder}
        onChange={(e) => {
          setText(e.target.value);
          onChange(splitWords(e.target.value));
        }}
        onBlur={() => setText(splitWords(text).join(', '))}
      />
      <small>{hint ?? '쉼표(,)로 나눠 적어요'}</small>
    </div>
  );
};

interface ModalProps {
  title: string;
  onClose: () => void;
  onSubmit: () => Promise<void>;
  submitLabel?: string;
  /** 지우기 같은 왼쪽 버튼 */
  extra?: ReactNode;
  children: ReactNode;
}

/** 편집 대화상자. 저장 중 오류는 상자 안에 보여 주고 닫지 않는다 */
export const Modal = ({ title, onClose, onSubmit, submitLabel = '저장', extra, children }: ModalProps) => {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && !busy && onClose();
    window.addEventListener('keydown', onKey);
    document.body.classList.add('no-scroll');
    return () => {
      window.removeEventListener('keydown', onKey);
      document.body.classList.remove('no-scroll');
    };
  }, [busy, onClose]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      await onSubmit();
    } catch (err) {
      setError(err instanceof Error ? err.message : '저장하지 못했습니다.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="modal-backdrop" onMouseDown={(e) => e.target === e.currentTarget && !busy && onClose()}>
      <form className="modal" role="dialog" aria-modal="true" aria-label={title} onSubmit={submit}>
        <header className="modal-head">
          <h2>{title}</h2>
          <button type="button" className="icon-btn" onClick={onClose} disabled={busy} aria-label="닫기">
            ✕
          </button>
        </header>
        <div className="modal-body">{children}</div>
        {error && <p className="error">{error}</p>}
        <footer className="modal-foot">
          {extra}
          <span className="grow" />
          <button type="button" className="btn ghost" onClick={onClose} disabled={busy}>
            취소
          </button>
          <button type="submit" className="btn primary" disabled={busy}>
            {busy ? '저장 중…' : submitLabel}
          </button>
        </footer>
      </form>
    </div>
  );
};

interface RowListProps<T> {
  title: string;
  rows: T[];
  onChange: (rows: T[]) => void;
  empty: () => T;
  addLabel: string;
  render: (row: T, update: (patch: Partial<T>) => void) => ReactNode;
}

/** 이력서의 경력 · 학력처럼 같은 모양의 줄을 더하고 빼고 순서를 바꾸는 목록 */
export const RowList = <T,>({ title, rows, onChange, empty, addLabel, render }: RowListProps<T>) => {
  const update = (i: number, patch: Partial<T>) => onChange(rows.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  const move = (i: number, d: -1 | 1) => {
    const next = [...rows];
    const [row] = next.splice(i, 1);
    next.splice(i + d, 0, row!);
    onChange(next);
  };
  return (
    <fieldset className="row-list">
      <legend>{title}</legend>
      {rows.map((row, i) => (
        <div className="row-item" key={i}>
          <div className="row-fields">{render(row, (patch) => update(i, patch))}</div>
          <div className="row-tools">
            <button type="button" className="icon-btn" disabled={i === 0} onClick={() => move(i, -1)} aria-label="위로">
              ↑
            </button>
            <button type="button" className="icon-btn" disabled={i === rows.length - 1} onClick={() => move(i, 1)} aria-label="아래로">
              ↓
            </button>
            <button type="button" className="icon-btn danger" onClick={() => onChange(rows.filter((_, j) => j !== i))} aria-label="빼기">
              ✕
            </button>
          </div>
        </div>
      ))}
      <button type="button" className="btn ghost small" onClick={() => onChange([...rows, empty()])}>
        + {addLabel}
      </button>
    </fieldset>
  );
};
