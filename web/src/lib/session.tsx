import type { Session } from '@supabase/supabase-js';
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { supabase } from './supabase';

interface SessionState {
  /** 세션을 아직 확인하는 중이면 false */
  ready: boolean;
  session: Session | null;
}

const SessionContext = createContext<SessionState>({ ready: false, session: null });

interface SessionProviderProps {
  children: ReactNode;
}

/** 포털과 같은 Supabase 세션. 방문자(로그인 안 함)는 session = null 로 그대로 공개 화면을 본다 */
export const SessionProvider = ({ children }: SessionProviderProps) => {
  const [state, setState] = useState<SessionState>({ ready: false, session: null });

  useEffect(() => {
    void supabase.auth.getSession().then(({ data }) => setState({ ready: true, session: data.session }));
    const { data } = supabase.auth.onAuthStateChange((_event, session) => setState({ ready: true, session }));
    return () => data.subscription.unsubscribe();
  }, []);

  return <SessionContext.Provider value={state}>{children}</SessionContext.Provider>;
};

export const useSession = (): SessionState => useContext(SessionContext);
