import assert from 'node:assert/strict'
import test from 'node:test'

import { SchedulerClient } from './dist/index.js'

test('normalizes owner projections and preserves command evidence', async () => {
  const calls = []
  const client = new SchedulerClient({
    request: async (path, options) => {
      calls.push({ path, options })
      if (path.endsWith('/definitions')) return { items: [{ key: 'daily', status: 'enabled' }] }
      if (path.endsWith('/state')) return { provisioned: true, runs: [{ id: 'run-1' }], dead_letters: [{ id: 'dead-1' }] }
      return { accepted: true }
    },
    evidence: (reason, confirmation) => ({ idempotencyKey: 'key-1', headers: { 'Idempotency-Key': 'key-1', 'X-Operation-Reason': reason, ...(confirmation ? { 'X-Operation-Confirmation': confirmation } : {}) } }),
  })

  assert.deepEqual(await client.definitions(), { items: [{ id: 'daily', data: { key: 'daily', status: 'enabled' }, created_at: '', updated_at: '' }], count: 1 })
  assert.equal((await client.state()).deadLetters[0].id, 'dead-1')
  await client.cancel('run/1', 'stuck run')
  assert.equal(calls.at(-1).path, '/operations/scheduler/runs/run%2F1/cancel')
  assert.equal(calls.at(-1).options.headers['X-Operation-Confirmation'], 'confirmed')
})
