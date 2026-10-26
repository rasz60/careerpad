import { useEffect, useState, type FormEvent } from 'react';
import { Navigate } from 'react-router-dom';
import { Field } from '../components/form';
import { useSession } from '../lib/session';
import { goToPortalLogin, supabase } from '../lib/supabase';

/** /career/ — 로그인했으면 작업 공간(/me)으로, 안 했으면 포털 로그인으로 (공개 페이지는 /career/<slug>) */
export const Home = () => {
  const { ready, session } = useSession();
  if (!ready) return <div className="page-loading" aria-busy="true" />;
  if (!session) return import.meta.env.DEV ? <DevLogin /> : <ToPortalLogin />;
  return <Navigate to="/me" replace />;
};

const ToPortalLogin = () => {
  useEffect(goToPortalLogin, []);
  return <div className="page-loading" aria-busy="true" />;
};

/** 개발 서버에는 포털이 없어서 같은 ae:p 계정으로 여기서 로그인한다 */
const DevLogin = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const { error: err } = await supabase.auth.signInWithPassword({ email, password });
    if (err) setError(err.message);
  };

  return (
    <main className="center-note">
      <form className="card" onSubmit={submit}>
        <h1>커리어++ (개발)</h1>
        <p className="muted">배포본은 ae:p 포털에서 로그인합니다.</p>
        <Field label="이메일" type="email" value={email} onChange={setEmail} required />
        <div className="field">
          <label htmlFor="dev-password">비밀번호</label>
          <input id="dev-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </div>
        {error && <p className="error">{error}</p>}
        <button type="submit" className="btn primary">
          로그인
        </button>
      </form>
    </main>
  );
};
