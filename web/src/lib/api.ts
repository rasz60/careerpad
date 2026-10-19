import { supabase } from './supabase';
import type {
  ContactInput,
  ContactSettings,
  ContactSettingsResponse,
  JobSearch,
  JobSearchInput,
  JobSite,
  Message,
  Profile,
  Project,
  ProjectInput,
  PublicPage,
  SiteResult,
} from './types';

// Caddy: /api/career/* → Go 백엔드 (접두어를 떼고 넘긴다). 개발은 vite proxy 가 같은 일을 한다
const BASE = '/api/career/api/v1';

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: unknown;
}

const request = async <T>(path: string, { method = 'GET', body }: RequestOptions = {}): Promise<T> => {
  const { data } = await supabase.auth.getSession();
  const headers: Record<string, string> = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  // 공개 화면도 로그인돼 있으면 토큰을 붙인다 (주인이면 비공개 항목 · 편집 메뉴가 열린다)
  if (data.session) headers.Authorization = `Bearer ${data.session.access_token}`;

  const res = await fetch(`${BASE}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;

  const json = (await res.json().catch(() => ({}))) as { error?: string; message?: string };
  if (!res.ok) throw new ApiError(res.status, json.error ?? 'unknown', json.message ?? `요청 실패 (${res.status})`);
  return json as T;
};

const seg = encodeURIComponent;

export const api = {
  publicPage: (slug: string) => request<PublicPage>(`/public/${seg(slug)}`),
  sendContact: (slug: string, input: ContactInput) =>
    request<{ ok: boolean }>(`/public/${seg(slug)}/contact`, { method: 'POST', body: input }),

  myProfile: () => request<Profile>('/me/profile'),
  updateProfile: (p: Omit<Profile, 'updated_at'>) => request<Profile>('/me/profile', { method: 'PUT', body: p }),

  createProject: (p: ProjectInput) => request<Project>('/me/projects', { method: 'POST', body: p }),
  updateProject: (id: string, p: ProjectInput) => request<Project>(`/me/projects/${seg(id)}`, { method: 'PUT', body: p }),
  deleteProject: (id: string) => request<void>(`/me/projects/${seg(id)}`, { method: 'DELETE' }),

  jobSites: () => request<JobSite[]>('/me/job-sites'),
  jobSearches: () => request<JobSearch[]>('/me/job-searches'),
  createJobSearch: (s: JobSearchInput) => request<JobSearch>('/me/job-searches', { method: 'POST', body: s }),
  updateJobSearch: (id: string, s: JobSearchInput) =>
    request<JobSearch>(`/me/job-searches/${seg(id)}`, { method: 'PUT', body: s }),
  deleteJobSearch: (id: string) => request<void>(`/me/job-searches/${seg(id)}`, { method: 'DELETE' }),
  jobResults: (id: string) => request<SiteResult[]>(`/me/job-searches/${seg(id)}/results`),

  contactSettings: () => request<ContactSettingsResponse>('/me/contact-settings'),
  saveContactSettings: (s: ContactSettings) =>
    request<ContactSettingsResponse>('/me/contact-settings', { method: 'PUT', body: s }),
  messages: () => request<Message[]>('/me/messages'),
  markMessage: (id: string, isRead: boolean) =>
    request<void>(`/me/messages/${seg(id)}`, { method: 'PATCH', body: { is_read: isRead } }),
  deleteMessage: (id: string) => request<void>(`/me/messages/${seg(id)}`, { method: 'DELETE' }),
};

/** 화면에 보일 에러 문장 */
export const errorText = (e: unknown): string => (e instanceof Error ? e.message : '알 수 없는 오류가 났습니다.');
