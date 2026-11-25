import * as ProjectService from '../../bindings/rag_project/internal/service/projectservice'
import type {
  Project as BackendProject,
  PersonaConfig as BackendPersonaConfig,
  Document as BackendDocument,
  ChatAnswer as BackendChatAnswer,
  ChunkHit as BackendChunkHit,
  EvalSet as BackendEvalSet,
  EvalCase as BackendEvalCase,
  EvalReport as BackendEvalReport,
  RetrievalConfig as BackendRetrievalConfig,
  ChatDebug as BackendChatDebug
} from '../../bindings/rag_project/internal/projects/models'
import type { Progress as BackendTrainingProgress } from '../../bindings/rag_project/internal/training/models'
import {
  mockStore,
  addFileToProject,
  removeFileFromProject,
  trainProjectKnowledge,
  type KnowledgeFile,
  type Project,
  type PersonaConfig
} from '../mock/data'
import { waitForWailsRuntime } from './settings'

export type { BackendProject, BackendPersonaConfig, BackendDocument }
export type TrainingProgress = BackendTrainingProgress
export type ChatAnswer = BackendChatAnswer
export type ChunkHit = BackendChunkHit

/** Largest document the backend accepts (50 MB). */
export const MAX_DOCUMENT_BYTES = 50 * 1024 * 1024

const ACCEPTED_EXTENSIONS = ['.pdf', '.md', '.txt', '.doc', '.docx']

export interface ImportDocumentsResult {
  imported: KnowledgeFile[]
  errors: string[]
}

export async function listProjects(): Promise<Project[]> {
  if (!(await waitForWailsRuntime())) {
    return [...mockStore.projects]
  }
  try {
    const list = await ProjectService.ListProjects()
    if (!list) {
      return []
    }
    return list.map(mapBackendProjectToUI)
  } catch (err) {
    console.warn('Failed to load projects from PostgreSQL:', err)
    return []
  }
}

export async function getProject(id: string): Promise<Project | undefined> {
  if (!(await waitForWailsRuntime())) {
    return mockStore.projects.find((p) => p.id === id)
  }
  try {
    const p = await ProjectService.GetProject(id)
    if (!p) {
      return mockStore.projects.find((item) => item.id === id)
    }
    return mapBackendProjectToUI(p)
  } catch (err) {
    console.warn('Failed to get project from PostgreSQL:', err)
    return mockStore.projects.find((item) => item.id === id)
  }
}

export async function createProject(name: string, description: string): Promise<Project> {
  if (!(await waitForWailsRuntime())) {
    const newProj: Project = {
      id: `proj-${Date.now()}`,
      name,
      description,
      knowledgeFilesCount: 0,
      personaName: `${name} Assistant`,
      lastUpdated: new Date().toISOString().split('T')[0],
      files: [],
      persona: {
        name: `${name} Assistant`,
        role: 'Knowledge Assistant',
        instructions: 'You are a helpful assistant. Use the uploaded files to answer questions accurately.',
        tone: 'Neutral',
        answerLanguage: 'English',
        strictKnowledge: true
      }
    }
    mockStore.projects.unshift(newProj)
    return newProj
  }
  const created = await ProjectService.CreateProject(name, description)
  if (!created) {
    throw new Error('Failed to create project')
  }
  const mapped = mapBackendProjectToUI(created)
  mockStore.projects.unshift(mapped)
  return mapped
}

export async function updateProject(id: string, name: string, description: string): Promise<Project> {
  if (!(await waitForWailsRuntime())) {
    const p = mockStore.projects.find((item) => item.id === id)
    if (p) {
      p.name = name
      p.description = description
      p.lastUpdated = new Date().toISOString().split('T')[0]
      return p
    }
    throw new Error('Project not found')
  }
  const updated = await ProjectService.UpdateProject(id, name, description)
  if (!updated) {
    throw new Error('Failed to update project')
  }
  const mapped = mapBackendProjectToUI(updated)
  const idx = mockStore.projects.findIndex((item) => item.id === id)
  if (idx !== -1) {
    mockStore.projects[idx] = mapped
  }
  return mapped
}

export async function deleteProject(id: string): Promise<void> {
  if (!(await waitForWailsRuntime())) {
    mockStore.projects = mockStore.projects.filter((p) => p.id !== id)
    return
  }
  await ProjectService.DeleteProject(id)
  mockStore.projects = mockStore.projects.filter((p) => p.id !== id)
}

export async function saveProjectPersona(projectId: string, persona: PersonaConfig): Promise<void> {
  if (!(await waitForWailsRuntime())) {
    const p = mockStore.projects.find((item) => item.id === projectId)
    if (p) {
      p.persona = { ...persona }
      p.personaName = persona.name
      p.lastUpdated = new Date().toISOString().split('T')[0]
    }
    return
  }
  await ProjectService.UpdatePersona(projectId, {
    name: persona.name,
    role: persona.role,
    instructions: persona.instructions,
    tone: persona.tone.toLowerCase(),
    answerLanguage: persona.answerLanguage,
    strictKnowledge: persona.strictKnowledge
  })
  const p = mockStore.projects.find((item) => item.id === projectId)
  if (p) {
    p.persona = { ...persona }
    p.personaName = persona.name
    p.lastUpdated = new Date().toISOString().split('T')[0]
  }
}

function mapBackendProjectToUI(bp: BackendProject): Project {
  // Normalize tone for UI
  const toneRaw = bp.persona?.tone ? bp.persona.tone.toLowerCase() : 'neutral'
  let tone: PersonaConfig['tone'] = 'Neutral'
  if (toneRaw === 'friendly') tone = 'Friendly'
  else if (toneRaw === 'formal') tone = 'Formal'
  else if (toneRaw === 'concise') tone = 'Concise'

  const persona: PersonaConfig = {
    name: bp.persona?.name || bp.personaName || 'General Assistant',
    role: bp.persona?.role || 'Knowledge Assistant',
    instructions: bp.persona?.instructions || 'You are a helpful assistant.',
    tone,
    answerLanguage: bp.persona?.answerLanguage || 'auto',
    strictKnowledge: bp.persona?.strictKnowledge ?? true
  }

  // Prefer documents loaded from PostgreSQL; fall back to local UI state.
  const existing = mockStore.projects.find((item) => item.id === bp.id)
  const files = bp.documents
    ? bp.documents.map(mapDocumentToKnowledgeFile)
    : existing?.files || []

  return {
    id: bp.id,
    name: bp.name,
    description: bp.description,
    knowledgeFilesCount: bp.knowledgeFilesCount,
    personaName: persona.name,
    lastUpdated: bp.lastUpdated || (bp.updatedAt ? bp.updatedAt.slice(0, 10) : new Date().toISOString().split('T')[0]),
    files,
    persona,
    isTraining: existing?.isTraining,
    trainingProgress: existing?.trainingProgress,
    trainingStep: existing?.trainingStep,
    lastIndexedAt: existing?.lastIndexedAt,
    totalChunks: existing?.totalChunks
  }
}

/** Imports files into a project, storing bytes locally and metadata in PostgreSQL. */
export async function importDocuments(
  projectId: string,
  files: File[]
): Promise<ImportDocumentsResult> {
  const imported: KnowledgeFile[] = []
  const errors: string[] = []

  const valid: File[] = []
  for (const file of files) {
    const dotIndex = file.name.lastIndexOf('.')
    const extension = dotIndex >= 0 ? file.name.slice(dotIndex).toLowerCase() : ''
    if (!ACCEPTED_EXTENSIONS.includes(extension)) {
      errors.push(`${file.name}: unsupported file type (use PDF, Markdown, TXT, or Word)`)
    } else if (file.size > MAX_DOCUMENT_BYTES) {
      errors.push(`${file.name}: file is larger than 50 MB`)
    } else if (file.size === 0) {
      errors.push(`${file.name}: file is empty`)
    } else {
      valid.push(file)
    }
  }

  if (!(await waitForWailsRuntime())) {
    for (const file of valid) {
      const created = addFileToProject(projectId, {
        name: file.name,
        type: fileTypeFromName(file.name),
        size: formatFileSize(file.size)
      })
      if (created) imported.push(created)
      else errors.push(`${file.name}: project not found`)
    }
    return { imported, errors }
  }

  try {
    const inputs = []
    for (const file of valid) {
      inputs.push({
        filename: file.name,
        mimeType: file.type || '',
        contentBase64: await readFileAsBase64(file)
      })
    }
    const documents = await ProjectService.ImportDocuments(projectId, inputs)
    if (!documents || documents.length === 0) {
      errors.push('Upload failed: no documents were stored')
    } else {
      const mapped = documents.map(mapDocumentToKnowledgeFile)
      imported.push(...mapped)
      appendFilesToStore(projectId, mapped)
    }
  } catch (err) {
    errors.push(`Upload failed: ${errorMessage(err)}`)
  }

  return { imported, errors }
}

/** Deletes a document row from PostgreSQL and removes its local file. */
export async function deleteDocument(projectId: string, fileId: string): Promise<void> {
  if (!(await waitForWailsRuntime())) {
    removeFileFromProject(projectId, fileId)
    return
  }
  await ProjectService.DeleteDocument(projectId, fileId)
  removeFileFromProject(projectId, fileId)
}

/**
 * Starts chunking the project's pending documents. Returns the initial job
 * snapshot; subsequent progress arrives through Wails events.
 */
export async function trainProject(projectId: string): Promise<TrainingProgress | null> {
  if (!(await waitForWailsRuntime())) {
    await trainProjectKnowledge(projectId)
    return null
  }
  return ProjectService.TrainProject(projectId)
}

/** Returns the current training job for a project, if one exists. */
export async function getTrainingStatus(projectId: string): Promise<TrainingProgress | null> {
  if (!(await waitForWailsRuntime())) {
    return null
  }
  return ProjectService.GetTrainingStatus(projectId)
}

/** Embeds a question and retrieves the closest chunks of a project. */
export async function retrieveChunks(
  projectId: string,
  query: string,
  limit = 8
): Promise<ChunkHit[]> {
  if (!(await waitForWailsRuntime())) {
    return []
  }
  const hits = await ProjectService.RetrieveChunks(projectId, query, limit)
  return hits || []
}

export type EvalSet = BackendEvalSet
export type EvalCase = BackendEvalCase
export type EvalReport = BackendEvalReport
export type RetrievalConfig = BackendRetrievalConfig
export type ChatDebug = BackendChatDebug

const DEFAULT_RETRIEVAL: RetrievalConfig = {
  topK: 8,
  minScore: 0,
  hybrid: false,
  rerank: false,
  guardrails: false,
  hybridWeight: 0.25,
  rerankWeight: 0.25
}

export async function getRetrievalConfig(projectId: string): Promise<RetrievalConfig> {
  if (!(await waitForWailsRuntime())) {
    return DEFAULT_RETRIEVAL
  }
  const cfg = await ProjectService.GetRetrievalConfig(projectId)
  return cfg || DEFAULT_RETRIEVAL
}

export async function updateRetrievalConfig(projectId: string, cfg: RetrievalConfig): Promise<void> {
  if (!(await waitForWailsRuntime())) return
  await ProjectService.UpdateRetrievalConfig(projectId, cfg)
}

export async function listEvalSets(projectId: string): Promise<EvalSet[]> {
  if (!(await waitForWailsRuntime())) return []
  return (await ProjectService.ListEvalSets(projectId)) || []
}

export async function createEvalSet(projectId: string, name: string): Promise<EvalSet> {
  const set = await ProjectService.CreateEvalSet(projectId, name)
  if (!set) throw new Error('Failed to create evaluation set')
  return set
}

export async function listEvalCases(setId: string): Promise<EvalCase[]> {
  if (!(await waitForWailsRuntime())) return []
  return (await ProjectService.ListEvalCases(setId)) || []
}

export async function addEvalCase(setId: string, question: string, expectedAnswer: string, expectedSource: string): Promise<EvalCase> {
  const evalCase = await ProjectService.AddEvalCase(setId, question, expectedAnswer, expectedSource)
  if (!evalCase) throw new Error('Failed to add evaluation case')
  return evalCase
}

export async function runEvaluation(setId: string): Promise<EvalReport> {
  const report = await ProjectService.RunEvaluation(setId)
  if (!report) throw new Error('Evaluation failed')
  return report
}

export interface PromptTemplate {
  id: string
  name: string
  instructions: string
}

export async function listPromptTemplates(): Promise<PromptTemplate[]> {
  if (!(await waitForWailsRuntime())) return []
  return (await ProjectService.ListPromptTemplates()) || []
}

export async function savePromptTemplate(name: string, instructions: string): Promise<PromptTemplate | null> {
  if (!(await waitForWailsRuntime())) return null
  return ProjectService.SavePromptTemplate(name, instructions)
}

export async function reindexDocument(projectId: string, documentId: string): Promise<void> {
  if (!(await waitForWailsRuntime())) return
  await ProjectService.ReindexDocument(projectId, documentId)
}

export async function cancelTraining(projectId: string): Promise<void> {
  if (!(await waitForWailsRuntime())) return
  await ProjectService.CancelTraining(projectId)
}

export async function cancelChat(projectId: string): Promise<void> {
  if (!(await waitForWailsRuntime())) return
  await ProjectService.CancelChat(projectId)
}

export async function askProjectStream(projectId: string, question: string): Promise<ChatAnswer> {
  const answer = await ProjectService.AskProjectStream(projectId, question)
  if (!answer) throw new Error('The playground did not return an answer')
  return answer
}

/** Asks the project a question using its persona and indexed knowledge. */
export async function askProject(projectId: string, question: string): Promise<ChatAnswer> {
  if (!(await waitForWailsRuntime())) {
    throw new Error('The playground needs the desktop runtime to answer questions')
  }
  const answer = await ProjectService.AskProject(projectId, question)
  if (!answer) {
    throw new Error('The playground did not return an answer')
  }
  return answer
}

function appendFilesToStore(projectId: string, files: KnowledgeFile[]) {
  const project = mockStore.projects.find((item) => item.id === projectId)
  if (!project) return
  for (const file of files) {
    if (!project.files.some((existing) => existing.id === file.id)) {
      project.files.unshift(file)
    }
  }
  project.knowledgeFilesCount = project.files.length
  project.lastUpdated = new Date().toISOString().split('T')[0]
}

export function mapDocumentToKnowledgeFile(document: BackendDocument): KnowledgeFile {
  let status: KnowledgeFile['status'] = 'Untrained'
  if (document.status === 'indexed') status = 'Ready'
  else if (document.status === 'chunked') status = 'Chunked'
  else if (document.status === 'processing') status = 'Processing'
  else if (document.status === 'failed') status = 'Failed'

  const created = document.createdAt ? new Date(document.createdAt) : new Date()
  const addedDate = Number.isNaN(created.getTime())
    ? new Date().toISOString().split('T')[0]
    : created.toISOString().split('T')[0]

  return {
    id: document.id,
    name: document.filename,
    type: fileTypeFromName(document.filename),
    size: formatFileSize(document.sizeBytes),
    addedDate,
    status,
    chunksCount: document.chunkCount || 0,
    errorMessage: document.errorMessage || undefined
  }
}

function fileTypeFromName(name: string): KnowledgeFile['type'] {
  const lower = name.toLowerCase()
  if (lower.endsWith('.pdf')) return 'PDF'
  if (lower.endsWith('.md')) return 'Markdown'
  if (lower.endsWith('.doc') || lower.endsWith('.docx')) return 'Word'
  return 'TXT'
}

function formatFileSize(bytes: number): string {
  if (!bytes || bytes < 0) return '0 KB'
  const kb = bytes / 1024
  if (kb >= 1024) return `${(kb / 1024).toFixed(1)} MB`
  return `${Math.max(1, Math.round(kb))} KB`
}

function readFileAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '')
    reader.onerror = () => reject(reader.error ?? new Error('failed to read file'))
    reader.readAsDataURL(file)
  })
}

function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message
  if (typeof err === 'string') return err
  return 'upload failed'
}
