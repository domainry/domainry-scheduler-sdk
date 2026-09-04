function record(item) {
    return {
        id: String(item.id ?? item.key ?? ''),
        data: item,
        created_at: String(item.created_at ?? ''),
        updated_at: String(item.updated_at ?? ''),
    };
}
export class SchedulerClient {
    #dependencies;
    constructor(dependencies) {
        this.#dependencies = dependencies;
    }
    async definitions() {
        const response = await this.#dependencies.request('/scheduler/definitions');
        const items = (response.items ?? []).map(record);
        return { items, count: response.count ?? items.length };
    }
    async state() {
        const response = await this.#dependencies.request('/scheduler/state');
        return {
            provisioned: response.provisioned,
            runs: (response.runs ?? []).map(record),
            deadLetters: (response.dead_letters ?? []).map(record),
        };
    }
    previewDefinition(data) {
        return this.#dependencies.request('/scheduler/definitions/validate', { method: 'POST', body: { data } });
    }
    authoringContract() {
        return this.#dependencies.request('/scheduler/authoring-contract');
    }
    definition(definitionID) {
        return this.#dependencies.request(`/scheduler/definitions/${encodeURIComponent(definitionID)}`);
    }
    async versions(definitionID) {
        const response = await this.#dependencies.request(`/scheduler/definitions/${encodeURIComponent(definitionID)}/versions`);
        return response.items ?? [];
    }
    simulate(definitionID) {
        return this.#dependencies.request(`/scheduler/definitions/${encodeURIComponent(definitionID)}/simulate`, { method: 'POST' });
    }
    run(definitionID, reason) {
        return this.command(`/scheduler/definitions/${encodeURIComponent(definitionID)}/run`, reason);
    }
    retry(runID, reason) {
        return this.command(`/scheduler/runs/${encodeURIComponent(runID)}/retry`, reason);
    }
    cancel(runID, reason) {
        return this.command(`/scheduler/runs/${encodeURIComponent(runID)}/cancel`, reason, 'confirmed');
    }
    resolve(deadLetterID, note) {
        return this.command(`/scheduler/dead-letters/${encodeURIComponent(deadLetterID)}/resolve`, note, 'confirmed', { note });
    }
    command(path, reason, confirmation, body) {
        const evidence = this.#dependencies.evidence(reason, confirmation);
        return this.#dependencies.request(path, { method: 'POST', body, headers: evidence.headers, requestId: evidence.idempotencyKey });
    }
}
