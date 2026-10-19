/**
 * pm2 프로세스 정의 (ae:p Oracle 서버). 최초 1회:  bash scripts/deploy.sh && pm2 save
 * Go 바이너리는 .env 를 읽지 않으므로 여기서 읽어 환경변수로 넘긴다.
 */
const fs = require('node:fs');
const path = require('node:path');

function readEnv(file) {
  if (!fs.existsSync(file)) return {};
  const env = {};
  for (const line of fs.readFileSync(file, 'utf8').split(/\r?\n/)) {
    const m = line.match(/^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$/);
    if (m) env[m[1]] = m[2].replace(/^(['"])(.*)\1$/, '$2');
  }
  return env;
}

module.exports = {
  apps: [
    {
      // 커리어++ API (Caddy /api/career/* → 127.0.0.1:PORT)
      name: 'careerpad-api',
      script: path.join(__dirname, 'backend/bin/careerpad-api'),
      interpreter: 'none',
      cwd: __dirname,
      env: readEnv(path.join(__dirname, '.env')),
      max_memory_restart: '200M',
      restart_delay: 3000,
    },
  ],
};
