import type {
  SchedulerClientDependencies,
  SchedulerDefinitionVersion,
  SchedulerRecord,
  SchedulerState,
} from './types.js'

function record(item: Record<string, unknown>): SchedulerRecord {
  return {
    id: String(item.id ?? item.key ?? ''),
    data: item,
    created_at: String(item.created_at ?? ''),
    updated_at: String(item.updated_at ?? ''),
  }
}

export class SchedulerClient {
  readonly #dependencies: SchedulerClientDependencies

  constructor(dependencies: SchedulerClientDependencies) {
    this.#dependencies = dependencies
  }

  async definitions(): Promise<{ items: SchedulerRecord[]; count: number }> {
    const response = await this.#dependencies.request<{ items?: Array<Record<string, unknown>>; count?: number }>('/tenant-admin/scheduler/definitions')
    const items = (response.items ?? []).map(record)
    return { items, count: response.count ?? items.length }
  }

  async state(): Promise<SchedulerState> {
    const response = await this.#dependencies.request<{
      provisioned: boolean
      runs?: Array<Record<string, unknown>>
      dead_letters?: Array<Record<string, unknown>>
    }>('/operations/scheduler/state')
    return {
      provisioned: response.provisioned,
      runs: (response.runs ?? []).map(record),
      deadLetters: (response.dead_letters ?? []).map(record),
    }
  }

  previewDefinition(data: Record<string, unknown>) {
    return this.#dependencies.request<{ next_runs: string[] }>('/tenant-admin/scheduler/definitions/validate', { method: 'POST', body: { data } })
  }

  authoringContract<T = Record<string, unknown>>() {
    return this.#dependencies.request<T>('/tenant-admin/scheduler/authoring-contract')
  }

  definition(definitionID: string) {
    return this.#dependencies.request<Record<string, unknown>>(`/tenant-admin/scheduler/definitions/${encodeURIComponent(definitionID)}`)
  }

  async versions(definitionID: string): Promise<SchedulerDefinitionVersion[]> {
    const response = await this.#dependencies.request<{ items?: SchedulerDefinitionVersion[] }>(`/tenant-admin/scheduler/definitions/${encodeURIComponent(definitionID)}/versions`)
    return response.items ?? []
  }

  simulate(definitionID: string) {
    return this.#dependencies.request(`/tenant-admin/scheduler/definitions/${encodeURIComponent(definitionID)}/simulate`, { method: 'POST' })
  }

  run(definitionID: string, reason: string) {
    return this.command(`/operations/scheduler/definitions/${encodeURIComponent(definitionID)}/run`, reason)
  }

  retry(runID: string, reason: string) {
    return this.command(`/operations/scheduler/runs/${encodeURIComponent(runID)}/retry`, reason)
  }

  cancel(runID: string, reason: string) {
    return this.command(`/operations/scheduler/runs/${encodeURIComponent(runID)}/cancel`, reason, 'confirmed')
  }

  resolve(deadLetterID: string, note: string) {
    return this.command(`/operations/scheduler/dead-letters/${encodeURIComponent(deadLetterID)}/resolve`, note, 'confirmed', { note })
  }

  private command(path: string, reason: string, confirmation?: 'confirmed', body?: unknown) {
    const evidence = this.#dependencies.evidence(reason, confirmation)
    return this.#dependencies.request(path, { method: 'POST', body, headers: evidence.headers })
  }
}
