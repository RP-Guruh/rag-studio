import * as UpdateService from '../../bindings/rag_project/internal/update/service'
import { waitForWailsRuntime } from './settings'

export interface UpdateInfo {
  currentVersion: string
  latestVersion: string
  updateAvailable: boolean
  releaseUrl: string
  notes: string
  assetName: string
  assetUrl: string
}

export async function checkForUpdate(): Promise<UpdateInfo | null> {
  if (!(await waitForWailsRuntime())) return null
  return (await UpdateService.CheckForUpdate()) || null
}

export async function openReleasePage(): Promise<void> {
  if (!(await waitForWailsRuntime())) return
  await UpdateService.OpenReleasePage()
}

export async function downloadLatestInstaller(): Promise<string> {
  if (!(await waitForWailsRuntime())) return ''
  return UpdateService.DownloadLatestInstaller()
}
