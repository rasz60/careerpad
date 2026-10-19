import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// ae:p 포털(ae2p.duckdns.org) 아래 /career/ 에서 돈다. 공개 페이지는 /career/<slug>
// PWA(manifest·서비스워커)는 도메인에 하나만 있어야 해서 포털이 맡는다
export default defineConfig({
  base: '/career/',
  plugins: [react()],
  server: {
    // 개발 중 Go API (backend, PORT=8092). 배포는 Caddy 가 /api/career/* 를 넘긴다
    proxy: { '/api/career': { target: 'http://127.0.0.1:8092', rewrite: (p) => p.replace(/^\/api\/career/, '') } },
  },
});
