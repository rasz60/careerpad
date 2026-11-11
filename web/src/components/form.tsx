import { useCallback, useEffect, useId, useRef, useState, type FormEvent, type ReactNode, type RefObject } from 'react';
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

/** 아이콘 전용 버튼 안에 넣는 선 아이콘 (글자 ✕ ↑ ↓ 는 글꼴마다 모양과 크기가 달라서 SVG 로) */
const ICONS = {
  close: 'M6 6l12 12M18 6L6 18',
  up: 'M12 19V5M6 11l6-6 6 6',
  down: 'M12 5v14M6 13l6 6 6-6',
} as const;

export const Icon = ({ name, size = 16 }: { name: keyof typeof ICONS; size?: number }) => (
  <svg aria-hidden="true" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d={ICONS[name]} />
  </svg>
);

/** styles.css 의 .modal-backdrop[data-closing] 애니메이션 길이와 같은 값 */
const DIALOG_EXIT_MS = 140;

const FOCUSABLE = 'a[href], button:not(:disabled), input:not(:disabled):not([type="hidden"]), select:not(:disabled), textarea:not(:disabled), summary, [tabindex]:not([tabindex="-1"])';

/**
 * 대화상자 공통 동작: 열릴 때 안으로 포커스를 옮기고 닫히면 원래 자리로 돌려 놓는다. Tab 은 상자 안에서만 돌고,
 * Escape 로 닫히며, 뒤 화면은 스크롤이 잠긴다. 닫을 때는 열 때보다 짧게 사라진 뒤 onClose 를 부른다.
 * close() 를 닫기 버튼 · 배경 · 취소에 쓴다 (저장 성공처럼 부모가 바로 치우는 경우는 그대로 사라진다)
 */
export const useDialog = (ref: RefObject<HTMLElement | null>, onClose: () => void, busy: boolean) => {
  const [closing, setClosing] = useState(false);
  const state = useRef({ onClose, busy, closing: false });
  state.current.onClose = onClose;
  state.current.busy = busy;

  useEffect(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const box = ref.current;
    // 첫 입력칸으로. 없으면 상자 자체로 (스크린리더가 제목부터 읽는다)
    const first = box?.querySelector<HTMLElement>('.modal-body input:not([type="file"]), .modal-body textarea, .modal-body select');
    (first ?? box)?.focus({ preventScroll: true });
    document.body.classList.add('no-scroll');
    return () => {
      document.body.classList.remove('no-scroll');
      if (opener?.isConnected) opener.focus({ preventScroll: true });
    };
  }, [ref]);

  const close = useCallback(() => {
    const s = state.current;
    if (s.busy || s.closing) return;
    s.closing = true;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return s.onClose();
    setClosing(true);
    window.setTimeout(() => state.current.onClose(), DIALOG_EXIT_MS);
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') return close();
      if (e.key !== 'Tab' || !ref.current) return;
      const items = [...ref.current.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => el.offsetParent !== null);
      if (items.length === 0) return e.preventDefault();
      const first = items[0]!;
      const last = items[items.length - 1]!;
      const at = document.activeElement;
      if (e.shiftKey && (at === first || at === ref.current || !ref.current.contains(at))) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && (at === last || !ref.current.contains(at))) {
        e.preventDefault();
        first.focus();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [ref, close]);

  const onBackdropMouseDown = (e: { target: unknown; currentTarget: unknown }) => {
    if (e.target === e.currentTarget) close();
  };
  return { close, closing, onBackdropMouseDown };
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

  const box = useRef<HTMLFormElement>(null);
  const { close, closing, onBackdropMouseDown } = useDialog(box, onClose, busy);

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
    <div className="modal-backdrop" data-closing={closing || undefined} onMouseDown={onBackdropMouseDown}>
      <form ref={box} className="modal" role="dialog" aria-modal="true" aria-label={title} tabIndex={-1} onSubmit={submit}>
        <header className="modal-head">
          <h2>{title}</h2>
          <button type="button" className="icon-btn" onClick={close} disabled={busy} aria-label="닫기">
            <Icon name="close" />
          </button>
        </header>
        <div className="modal-body">{children}</div>
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
        <footer className="modal-foot">
          {extra}
          <span className="grow" />
          <button type="button" className="btn ghost" onClick={close} disabled={busy}>
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
              <Icon name="up" />
            </button>
            <button type="button" className="icon-btn" disabled={i === rows.length - 1} onClick={() => move(i, 1)} aria-label="아래로">
              <Icon name="down" />
            </button>
            <button type="button" className="icon-btn danger" onClick={() => onChange(rows.filter((_, j) => j !== i))} aria-label="빼기">
              <Icon name="close" />
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
