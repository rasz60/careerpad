import { useCallback, useEffect, useState, type DependencyList } from 'react';

export interface AsyncState<T> {
  data: T | undefined;
  error: unknown;
  loading: boolean;
  /** 같은 조건으로 다시 불러온다 (저장 · 삭제 뒤) */
  reload: () => void;
  /** 다시 묻지 않고 화면 값만 바꾼다 (저장 응답을 바로 반영할 때) */
  setData: (next: T) => void;
}

/** deps 가 바뀌면 fn 을 다시 부른다. 늦게 도착한 이전 응답은 버린다 */
export const useAsync = <T,>(fn: () => Promise<T>, deps: DependencyList): AsyncState<T> => {
  const [data, setData] = useState<T>();
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError(null);
    fn().then(
      (v) => alive && (setData(v), setLoading(false)),
      (e: unknown) => alive && (setError(e), setLoading(false)),
    );
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick]);

  const reload = useCallback(() => setTick((t) => t + 1), []);
  return { data, error, loading, reload, setData };
};
