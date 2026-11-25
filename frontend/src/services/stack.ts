import * as StackService from '../../bindings/rag_project/internal/stack/service'
import type {
  Status as StackStatus,
  Result as StackResult
} from '../../bindings/rag_project/internal/stack/models'
import { isWailsRuntime, waitForWailsRuntime } from './settings'

export type { StackStatus, StackResult }

async function ensureRuntime(): Promise<void> {
  if (isWailsRuntime()) return
  if (!(await waitForWailsRuntime())) {
    throw new Error('Desktop runtime is not available. Restart the app and try again.')
  }
}

export async function stackStatus(): Promise<StackStatus | null> {
  try {
    await ensureRuntime()
    return await StackService.StackStatus()
  } catch {
    return null
  }
}

export async function startLocalStack(): Promise<StackResult | null> {
  await ensureRuntime()
  return StackService.StartLocalStack()
}

export async function startLocalStackClean(): Promise<StackResult | null> {
  await ensureRuntime()
  return StackService.StartLocalStackClean()
}

export async function stopLocalStack(): Promise<StackResult | null> {
  await ensureRuntime()
  return StackService.StopLocalStack()
}

export async function applyLocalPreset(): Promise<StackResult | null> {
  await ensureRuntime()
  return StackService.ApplyLocalPreset()
}
