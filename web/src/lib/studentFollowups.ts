export const studentStages = ['suggested', 'informed', 'interested', 'applied', 'interview', 'offer', 'closed'] as const;
export type StudentStage = typeof studentStages[number];
export const contactChannels = ['email', 'phone', 'whatsapp', 'messenger', 'other'] as const;
export type ContactChannel = typeof contactChannels[number];
export interface StudentFollowup {
  id: number; job_id: number; student_ref: string; student_name: string; profile_url: string;
  reason: string; status: StudentStage; notes: string; follow_up: string | null; version: number;
  created_at: string; updated_at: string;
  public_slug: string; title: string; company: string; job_url: string; closed_at: string | null;
  assignee: string; overdue: boolean; contact_count: number;
}
export interface StudentFollowupCreate { student_ref: string; student_name: string; profile_url: string; reason: string }
export interface StudentFollowupUpdate { student_name: string; profile_url: string; reason: string; status: StudentStage; notes: string; follow_up: string | null; expected_version: number }
export interface StudentContactInput { channel: ContactChannel; contacted_at: string; note: string; request_id: string; expected_version: number }
export interface StudentFollowupEvent {
  id: number; actor: string; action: string; student_name: string; profile_url: string; reason: string;
  status: StudentStage; notes: string; follow_up: string | null; version: number; channel: string;
  contacted_at: string | null; contact_note: string; created_at: string;
}
export interface StudentFollowupQueue { data: StudentFollowup[]; meta: { has_more: boolean; limit: number; offset: number } }
export interface StudentFollowupFilter { status?: string; q?: string; overdue?: boolean; offset?: number }
export function safeStudentLink(value: string): string {
  try { const u = new URL(value); return (u.protocol === 'https:' || u.protocol === 'http:') && !u.username && !u.password ? u.href : ''; } catch { return ''; }
}
export function studentMessage(row: Pick<StudentFollowup, 'student_name' | 'title' | 'company' | 'job_url'>): string {
  return `Hi${row.student_name ? ` ${row.student_name}` : ''},\n\nThis opportunity may be relevant to you: ${row.title} at ${row.company}.\n${safeStudentLink(row.job_url)}\n\nPlease check the original requirements and deadline. Let us know if you are interested or have applied.`;
}
export function bangladeshDateTime(now = new Date()): string {
  const parts = new Intl.DateTimeFormat('en-GB', { timeZone: 'Asia/Dhaka', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).formatToParts(now);
  const get = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)?.value ?? '';
  return `${get('year')}-${get('month')}-${get('day')}T${get('hour')}:${get('minute')}`;
}
export function bangladeshTime(value: string): string {
  return new Intl.DateTimeFormat('en-GB', { timeZone: 'Asia/Dhaka', dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}
