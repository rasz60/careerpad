import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { Layout } from './components/Layout';
import { Workspace } from './components/Workspace';
import { SessionProvider } from './lib/session';
import { About, ResumePage } from './pages/About';
import { Certificates } from './pages/Certificates';
import { Contact, InboxPage } from './pages/Contact';
import { Home } from './pages/Home';
import { Jobs } from './pages/Jobs';
import { Projects } from './pages/Projects';
import { Today } from './pages/Today';
import '@fontsource/barlow-condensed/500.css';
import '@fontsource/barlow-condensed/700.css';
import 'pretendard/dist/web/variable/pretendardvariable-dynamic-subset.css';
import './styles.css';

// ae:p 포털(같은 도메인) 아래 /career/ 에서 돈다. vite.config.ts 의 base 와 같은 값
const BASENAME = import.meta.env.BASE_URL.replace(/\/$/, '');

// /me 는 나만 쓰는 작업 공간 (오늘 · 채용공고 · 자격증 · 이력서 · 메시지).
// /:slug 는 로그인 없이 열리는 공개 이력서 한 장 ('me' 는 서버가 주소로 못 쓰게 막아 둔 이름)
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <SessionProvider>
      <BrowserRouter basename={BASENAME}>
        <Routes>
          <Route index element={<Home />} />
          <Route path="me" element={<Workspace />}>
            <Route index element={<Today />} />
            <Route path="jobs" element={<Jobs />} />
            <Route path="certs" element={<Certificates />} />
            <Route path="resume" element={<ResumePage />} />
            <Route path="inbox" element={<InboxPage />} />
            <Route path="*" element={<Navigate to="/me" replace />} />
          </Route>
          <Route path=":slug" element={<Layout />}>
            <Route index element={<About />} />
            <Route path="projects" element={<Projects />} />
            <Route path="contact" element={<Contact />} />
            {/* 예전 관리 화면 주소 */}
            <Route path="jobs" element={<Navigate to="/me/jobs" replace />} />
            <Route path="inbox" element={<Navigate to="/me/inbox" replace />} />
            <Route path="*" element={<Navigate to=".." replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </SessionProvider>
  </StrictMode>,
);
