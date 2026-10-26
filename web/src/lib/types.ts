// Go 백엔드(domain 패키지)의 JSON 과 1:1. 필드 이름은 백엔드와 같은 snake_case

export interface Link {
  label: string;
  url: string;
}

/** 기간은 "YYYY-MM". ended_on 이 비면 재직·진행 중 */
export interface Experience {
  company: string;
  role: string;
  started_on: string;
  ended_on: string;
  description: string;
}

export interface Education {
  school: string;
  major: string;
  degree: string;
  started_on: string;
  ended_on: string;
}

export interface SkillGroup {
  category: string;
  items: string[];
}

export type CertStatus = 'acquired' | 'preparing';

/** 준비 중인 자격 · 자격번호 · 메모 · 시험일은 주인에게만 온다 (방문자 응답에서는 빠진다) */
export interface Certificate {
  name: string;
  issuer: string;
  /** YYYY-MM */
  acquired_on: string;
  status: CertStatus;
  /** 점수 · 등급 */
  grade: string;
  cert_no: string;
  /** 시험일 YYYY-MM-DD */
  exam_on: string;
  /** 유효기간 끝 YYYY-MM-DD */
  expires_on: string;
  memo: string;
}

export interface Resume {
  experiences: Experience[];
  educations: Education[];
  skills: SkillGroup[];
  certificates: Certificate[];
}

export interface Profile {
  slug: string;
  is_public: boolean;
  name: string;
  headline: string;
  summary: string;
  photo_url: string;
  location: string;
  public_email: string;
  public_phone: string;
  links: Link[];
  resume: Resume;
  updated_at: string;
}

export type ProjectKind = 'work' | 'personal';

export interface Project {
  id: string;
  kind: ProjectKind;
  title: string;
  organization: string;
  role: string;
  started_on: string;
  ended_on: string;
  summary: string;
  description: string;
  tech_stack: string[];
  links: Link[];
  is_public: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

/** 새로 만들 때 보내는 값 (id·시각은 서버가 정한다) */
export type ProjectInput = Omit<Project, 'id' | 'created_at' | 'updated_at'>;

/** GET /public/{slug} */
export interface PublicPage {
  profile: Profile;
  projects: Project[];
  is_owner: boolean;
}

export interface JobSite {
  id: string;
  name: string;
  /** false 면 목록 없이 사이트 검색 링크만 */
  listable: boolean;
}

export interface JobSearch {
  id: string;
  name: string;
  keywords: string[];
  sites: string[];
  location: string;
  career_years: number | null;
  exclude_words: string[];
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export type JobSearchInput = Omit<JobSearch, 'id' | 'created_at' | 'updated_at'>;

export interface JobPosting {
  id: string;
  title: string;
  company: string;
  location: string;
  career: string;
  career_min: number | null;
  career_max: number | null;
  /** YYYY-MM-DD, 비면 상시 · 모름 */
  deadline: string;
  url: string;
  tags: string[];
  /** 이 공고가 걸린 검색어들 */
  keywords: string[];
}

export interface KeywordLink {
  keyword: string;
  url: string;
}

export interface SiteResult {
  site: JobSite;
  links: KeywordLink[];
  postings: JobPosting[];
  error?: string;
  /** 조건이 다 걸리지 않았을 때의 안내 (근무지를 모르는 공고를 남겼다 · 링크에는 키워드만) */
  note?: string;
}

export interface ContactSettings {
  notify_email: string;
  notify_phone: string;
  mail_enabled: boolean;
  sms_enabled: boolean;
}

export interface ContactSettingsResponse extends ContactSettings {
  /** 서버에 실제로 설정된 발송 수단 */
  available: { mail: boolean; sms: boolean };
}

export interface ContactInput {
  name: string;
  email: string;
  phone: string;
  message: string;
  /** 사람에게는 안 보이는 칸 (봇 걸러내기) */
  website: string;
}

export interface Message {
  id: string;
  sender_name: string;
  sender_email: string;
  sender_phone: string;
  body: string;
  delivered_by: string[];
  is_read: boolean;
  created_at: string;
}
