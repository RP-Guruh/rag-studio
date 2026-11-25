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

export interface InstallResult {
  success: boolean
  message: string
  restartRequired: boolean
}

export async function installUpdate(path: string): Promise<InstallResult> {
  if (!(await waitForWailsRuntime())) {
    return { success: false, message: 'Desktop runtime is not available.', restartRequired: false }
  }
  const result = await UpdateService.InstallUpdate(path)
  return result || { success: false, message: 'Installer did not return a result.', restartRequired: false }
}
