import * as StackService from '../../bindings/rag_project/internal/stack/service'
import type {
  Status as StackStatus,
  Result as StackResult
} from '../../bindings/rag_project/internal/stack/models'
import { waitForWailsRuntime } from './settings'

export type { StackStatus, StackResult }

export async function stackStatus(): Promise<StackStatus | null> {
  if (!(await waitForWailsRuntime())) return null
  return StackService.StackStatus()
}

export async function startLocalStack(): Promise<StackResult | null> {
  if (!(await waitForWailsRuntime())) return null
  return StackService.StartLocalStack()
}

export async function stopLocalStack(): Promise<StackResult | null> {
  if (!(await waitForWailsRuntime())) return null
  return StackService.StopLocalStack()
}

export async function applyLocalPreset(): Promise<StackResult | null> {
  if (!(await waitForWailsRuntime())) return null
  return StackService.ApplyLocalPreset()
}
