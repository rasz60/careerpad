import { useEffect, useRef, type RefObject } from 'react';

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/**
 * 대화상자 접근성: 열리면 안으로 포커스를 옮기고, Tab 이 밖으로 새지 않게 가두고,
 * Escape 로 닫고, 닫히면 열기 전의 자리로 포커스를 돌려준다. 열려 있는 동안 배경 스크롤을 막는다.
 */
export const useDialog = (ref: RefObject<HTMLElement | null>, onClose: () => void, busy = false) => {
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  const busyRef = useRef(busy);
  busyRef.current = busy;

  useEffect(() => {
    const root = ref.current;
    const before = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const items = () => [...(root?.querySelectorAll<HTMLElement>(FOCUSABLE) ?? [])].filter((el) => el.offsetParent !== null);
    // 첫 입력칸이 있으면 거기로, 없으면 대화상자 자체로
    const first = root?.querySelector<HTMLElement>('input:not([type=hidden]), textarea, select') ?? items()[0] ?? root;
    first?.focus({ preventScroll: true });

    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !busyRef.current) {
        e.preventDefault();
        closeRef.current();
        return;
      }
      if (e.key !== 'Tab') return;
      const list = items();
      if (list.length === 0) return e.preventDefault();
      const head = list[0]!;
      const tail = list[list.length - 1]!;
      if (e.shiftKey && (document.activeElement === head || !root?.contains(document.activeElement))) {
        e.preventDefault();
        tail.focus();
      } else if (!e.shiftKey && (document.activeElement === tail || !root?.contains(document.activeElement))) {
        e.preventDefault();
        head.focus();
      }
    };
    document.addEventListener('keydown', onKey);
    document.body.classList.add('no-scroll');
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.classList.remove('no-scroll');
      before?.focus?.({ preventScroll: true });
    };
  }, [ref]);
};
