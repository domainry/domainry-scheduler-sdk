export interface SchedulerRecord {
    id: string;
    data: Record<string, unknown>;
    created_at?: string;
    updated_at?: string;
}
export interface SchedulerDefinitionVersion {
    version_id: string;
    event: string;
    data: Record<string, unknown>;
    created_at: string;
}
export interface SchedulerState {
    provisioned: boolean;
    runs: SchedulerRecord[];
    deadLetters: SchedulerRecord[];
}
export interface SchedulerRequestOptions {
    method?: 'GET' | 'POST';
    body?: unknown;
    headers?: Record<string, string>;
}
export type SchedulerRequest = <T>(path: string, options?: SchedulerRequestOptions) => Promise<T>;
export interface SchedulerEvidence {
    idempotencyKey: string;
    headers: Record<string, string>;
}
export interface SchedulerClientDependencies {
    request: SchedulerRequest;
    evidence(reason: string, confirmation?: 'confirmed'): SchedulerEvidence;
}
