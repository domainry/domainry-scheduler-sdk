import type { SchedulerClientDependencies, SchedulerDefinitionVersion, SchedulerRecord, SchedulerState } from './types.js';
export declare class SchedulerClient {
    #private;
    constructor(dependencies: SchedulerClientDependencies);
    definitions(): Promise<{
        items: SchedulerRecord[];
        count: number;
    }>;
    state(): Promise<SchedulerState>;
    previewDefinition(data: Record<string, unknown>): Promise<{
        next_runs: string[];
    }>;
    authoringContract<T = Record<string, unknown>>(): Promise<T>;
    definition(definitionID: string): Promise<Record<string, unknown>>;
    versions(definitionID: string): Promise<SchedulerDefinitionVersion[]>;
    simulate(definitionID: string): Promise<unknown>;
    run(definitionID: string, reason: string): Promise<unknown>;
    retry(runID: string, reason: string): Promise<unknown>;
    cancel(runID: string, reason: string): Promise<unknown>;
    resolve(deadLetterID: string, note: string): Promise<unknown>;
    private command;
}
