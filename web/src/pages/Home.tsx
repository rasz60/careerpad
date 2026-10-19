import { useEffect, useState, type FormEvent } from 'react';
import { Navigate } from 'react-router-dom';
import { Field } from '../components/form';
import { api, errorText } from '../lib/api';
import { useSession } from '../lib/session';
import { goToPortalLogin, supabase } from '../lib/supabase';
import { useAsync } from '../lib/useAsync';

/** /career/ — 주인은 내 공개 페이지로, 로그인 안 한 사람은 포털 로그인으로 (공개 페이지는 /career/<slug>) */
export const Home = () => {
  const { ready, session } = useSession();
  if (!ready) return <div className="page-loading" aria-busy="true" />;
  if (!session) return import.meta.env.DEV ? <DevLogin /> : <ToPortalLogin />;
  return <ToMyPage />;
};

const ToPortalLogin = () => {
  useEffect(goToPortalLogin, []);
  return <div className="page-loading" aria-busy="true" />;
};

/** 처음 들어오면 서버가 이메일 아이디로 공개 주소를 정해 빈 프로필을 만든다 */
const ToMyPage = () => {
  const profile = useAsync(() => api.myProfile(), []);
  if (profile.error) {
    return (
      <main className="center-note">
        <h1>불러오지 못했어요</h1>
        <p>{errorText(profile.error)}</p>
        <button type="button" className="btn primary" onClick={profile.reload}>
          다시 시도
        </button>
      </main>
    );
  }
  if (!profile.data) return <div className="page-loading" aria-busy="true" />;
  return <Navigate to={`/${profile.data.slug}`} replace />;
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
