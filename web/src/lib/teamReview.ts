import type { User } from './types.js';

export type TeamRole = 'admin' | 'reviewer';
export interface TeamAccess { user_id: number; name: string; role: TeamRole | 'none' }
export interface TeamMember { id: number; email: string; name: string; team_role: TeamRole; active: boolean; version: number; assignment_count: number }
export interface TeamMemberInput { name: string; email: string; password?: string; use_existing: boolean }
export interface TeamMemberUpdate { name: string; active: boolean; expected_version: number }
export interface TeamMemberEvent { id: number; action: string; actor: string; created_at: string }
export type ReviewStatus = 'pending' | 'in_review' | 'reviewed' | 'not_relevant';
export interface TeamReview {
  job_id: number; public_slug: string; title: string; company: string; closed_at: string | null;
  assignee_id: number | null; assignee: string; assignee_active: boolean;
  status: ReviewStatus; notes: string; follow_up: string | null; version: number;
  updated_at: string | null; overdue: boolean;
}
export interface TeamReviewInput { assignee_id: number | null; status: ReviewStatus; notes: string; follow_up: string | null; expected_version: number }
export interface TeamReviewEvent { id: number; action: string; actor: string; assignee: string; status: ReviewStatus; notes: string; follow_up: string | null; version: number; created_at: string }
export interface TeamQueue { data: TeamReview[]; meta: { has_more: boolean; limit: number; offset: number } }
export interface TeamQueueFilter { scope?: 'all' | 'mine' | 'unassigned'; status?: string; overdue?: boolean; offset?: number }
export const reviewStatuses: { value: ReviewStatus; label: string }[] = [
  { value: 'pending', label: 'Pending' }, { value: 'in_review', label: 'In review' },
  { value: 'reviewed', label: 'Reviewed' }, { value: 'not_relevant', label: 'Not relevant' },
];
export const reviewStatusLabel = (status: ReviewStatus): string => reviewStatuses.find((s) => s.value === status)?.label ?? status;

/** Navigation affordance only. Every private read and write checks current access. */
export function teamRole(user: Pick<User, 'role' | 'team_role'> | null): TeamRole | null {
  if (user?.role === 'moderator' || user?.role === 'admin') return 'admin';
  return user?.team_role === 'reviewer' ? 'reviewer' : null;
}
