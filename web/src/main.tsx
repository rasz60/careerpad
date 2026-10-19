import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { Layout } from './components/Layout';
import { SessionProvider } from './lib/session';
import { About } from './pages/About';
import { Contact } from './pages/Contact';
import { Home } from './pages/Home';
import { Jobs } from './pages/Jobs';
import { Projects } from './pages/Projects';
import 'pretendard/dist/web/variable/pretendardvariable-dynamic-subset.css';
import './styles.css';

// ae:p 포털(같은 도메인) 아래 /career/ 에서 돈다. vite.config.ts 의 base 와 같은 값
const BASENAME = import.meta.env.BASE_URL.replace(/\/$/, '');

// 공개 페이지(/:slug/…)는 로그인 없이 열린다. 주인이면 같은 화면에 편집 · Job++ 이 더해진다
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <SessionProvider>
      <BrowserRouter basename={BASENAME}>
        <Routes>
          <Route index element={<Home />} />
          <Route path=":slug" element={<Layout />}>
            <Route index element={<About />} />
            <Route path="projects" element={<Projects />} />
            <Route path="jobs" element={<Jobs />} />
            <Route path="contact" element={<Contact />} />
            <Route path="*" element={<Navigate to=".." replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </SessionProvider>
  </StrictMode>,
);
