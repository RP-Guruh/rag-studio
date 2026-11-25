import { reactive } from 'vue'
import type {
  ChatModel,
  Database,
  DoclingConnection,
  EmbeddingModel
} from '../../bindings/rag_project/internal/settings/models'

export interface KnowledgeFile {
  id: string
  name: string
  type: 'PDF' | 'Markdown' | 'TXT' | 'Word'
  size: string
  addedDate: string
  status: 'Ready' | 'Processing' | 'Untrained' | 'Training' | 'Chunked' | 'Failed'
  chunksCount?: number
  tokenCount?: number
  errorMessage?: string
}

export interface PersonaConfig {
  name: string
  role: string
  instructions: string
  tone: 'Neutral' | 'Friendly' | 'Formal' | 'Concise'
  answerLanguage: string
  strictKnowledge: boolean
}

export interface Project {
  id: string
  name: string
  description: string
  knowledgeFilesCount: number
  personaName: string
  lastUpdated: string
  files: KnowledgeFile[]
  persona: PersonaConfig
  isTraining?: boolean
  trainingProgress?: number
  trainingStep?: string
  lastIndexedAt?: string
  totalChunks?: number
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  timestamp: string
  sources?: string[]
}

export interface RetrievedChunk {
  id: string
  fileName: string
  location: string
  excerpt: string
}

export type ChatModelSettings = ChatModel

export type EmbeddingModelSettings = EmbeddingModel

export type DatabaseSettings = Database

export interface AppSettings {
  chatModel: ChatModel
  embeddingModel: EmbeddingModel
  database: Database
  docling: DoclingConnection
}

export interface AppState {
  currentUser: {
    name: string
  } | null
  projects: Project[]
  activeProject: Project | null
  playground: {
    selectedProjectId: string
    messages: Record<string, ChatMessage[]>
    lastRetrievedChunks: Record<string, RetrievedChunk[]>
  }
  settings: AppSettings
}

export const initialProjects: Project[] = []

export const initialRetrievedChunks: Record<string, RetrievedChunk[]> = {
  'proj-legal-01': [
    {
      id: 'chunk-1',
      fileName: 'Master_Services_Agreement_2025.pdf',
      location: 'Section 8.2 (Limitation of Liability)',
      excerpt: "Neither party's total cumulative liability arising out of or related to this Agreement shall exceed the total amount paid by Customer hereunder in the twelve (12) months preceding the incident."
    },
    {
      id: 'chunk-2',
      fileName: 'Vendor_Security_Policy.docx',
      location: 'Section 3.1 (Audit Rights)',
      excerpt: 'Customer reserves the right to perform annual security assessments of Vendor facilities and hosting environments with a minimum of thirty (30) business days written notice.'
    }
  ],
  'proj-eng-02': [
    {
      id: 'chunk-3',
      fileName: 'Incident_Response_Runbook.txt',
      location: 'Section 2.4 (Database Failover)',
      excerpt: 'Execute `patronictl -c /etc/patroni/config.yml failover` during primary node partition. Check replication lag on secondary replicas before committing leader switch.'
    }
  ],
  'proj-support-03': [
    {
      id: 'chunk-4',
      fileName: 'Customer_Refund_Policy_2026.pdf',
      location: 'Section 1.3 (Refund Window)',
      excerpt: 'Annual subscription plans are eligible for a prorated refund if requested within fourteen (14) calendar days of the initial billing statement date.'
    }
  ]
}

export const initialMessages: Record<string, ChatMessage[]> = {
  'proj-legal-01': [
    {
      id: 'msg-1',
      role: 'user',
      content: 'What is the liability cap under the standard Master Services Agreement?',
      timestamp: '10:14 AM'
    },
    {
      id: 'msg-2',
      role: 'assistant',
      content: 'Under Section 8.2 of the Master Services Agreement, neither party is liable for cumulative damages exceeding the total amount paid by the customer in the twelve (12) months immediately preceding the incident.',
      timestamp: '10:14 AM',
      sources: ['Master_Services_Agreement_2025.pdf']
    }
  ],
  'proj-eng-02': [
    {
      id: 'msg-3',
      role: 'user',
      content: 'How do we trigger database failover during an outage?',
      timestamp: '09:30 AM'
    },
    {
      id: 'msg-4',
      role: 'assistant',
      content: 'According to the incident response runbook, run `patronictl -c /etc/patroni/config.yml failover`. Verify replication lag on secondary replicas before proceeding.',
      timestamp: '09:31 AM',
      sources: ['Incident_Response_Runbook.txt']
    }
  ],
  'proj-support-03': [
    {
      id: 'msg-5',
      role: 'user',
      content: 'Can annual customers request a refund after purchase?',
      timestamp: '08:45 AM'
    },
    {
      id: 'msg-6',
      role: 'assistant',
      content: 'Yes. Annual subscription plans can receive a prorated refund if requested within 14 calendar days of the initial billing date.',
      timestamp: '08:45 AM',
      sources: ['Customer_Refund_Policy_2026.pdf']
    }
  ]
}

export const mockStore = reactive<AppState>({
  currentUser: null,
  projects: [...initialProjects],
  activeProject: null,
  playground: {
    selectedProjectId: 'proj-legal-01',
    messages: { ...initialMessages },
    lastRetrievedChunks: { ...initialRetrievedChunks }
  },
  settings: {
    chatModel: {
      provider: 'openai-compatible',
      baseUrl: 'https://api.openai.com/v1',
      modelName: 'gpt-4o',
      temperature: 0.2,
      maxOutputTokens: 4096,
      apiKeySet: false,
      apiKeyHint: '',
      configured: true
    },
    embeddingModel: {
      provider: 'openai-compatible',
      baseUrl: 'https://api.openai.com/v1',
      modelName: 'text-embedding-3-small',
      vectorDimension: 1536,
      chunkMaxTokens: 512,
      tokenizer: 'BAAI/bge-m3',
      apiKeySet: false,
      apiKeyHint: '',
      configured: true
    },
    database: {
      host: 'localhost',
      port: 5432,
      databaseName: 'rag_projects_db',
      username: 'postgres',
      sslMode: 'disable',
      schemaName: 'public',
      passwordSet: false,
      passwordHint: '',
      configured: true
    },
    docling: {
      mode: 'serve',
      baseUrl: 'http://localhost:5001',
      timeoutSeconds: 300,
      apiKeySet: false,
      apiKeyHint: '',
      configured: true
    }
  }
})

/* Helper functions to operate local mock state */
export function signOut() {
  mockStore.currentUser = null
}

export function addProject(name: string, description: string): Project {
  const newProject: Project = {
    id: `proj-${Date.now()}`,
    name,
    description,
    knowledgeFilesCount: 0,
    personaName: 'General Assistant',
    lastUpdated: new Date().toISOString().split('T')[0],
    files: [],
    persona: {
      name: 'General Assistant',
      role: 'Knowledge Assistant',
      instructions: 'You are a helpful assistant. Use the uploaded files to answer questions accurately.',
      tone: 'Neutral',
      answerLanguage: 'English',
      strictKnowledge: true
    }
  }
  mockStore.projects.unshift(newProject)
  return newProject
}

export function getProjectById(id: string): Project | undefined {
  return mockStore.projects.find((p) => p.id === id)
}

export function addFileToProject(projectId: string, file: { name: string; type: KnowledgeFile['type']; size: string }): KnowledgeFile | undefined {
  const proj = getProjectById(projectId)
  if (!proj) return undefined
  const newFile: KnowledgeFile = {
    id: `file-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
    name: file.name,
    type: file.type,
    size: file.size,
    addedDate: new Date().toISOString().split('T')[0],
    status: 'Untrained',
    chunksCount: 0
  }
  proj.files.unshift(newFile)
  proj.knowledgeFilesCount = proj.files.length
  proj.lastUpdated = new Date().toISOString().split('T')[0]
  return newFile
}

export async function trainProjectKnowledge(
  projectId: string,
  onStep?: (progress: number, stepText: string) => void
): Promise<void> {

}

export function removeFileFromProject(projectId: string, fileId: string) {
  const proj = getProjectById(projectId)
  if (!proj) return
  proj.files = proj.files.filter((f) => f.id !== fileId)
  proj.knowledgeFilesCount = proj.files.length
  proj.lastUpdated = new Date().toISOString().split('T')[0]
}

export function updateProjectPersona(projectId: string, persona: PersonaConfig) {
  const proj = getProjectById(projectId)
  if (!proj) return
  proj.persona = { ...persona }
  proj.personaName = persona.name
  proj.lastUpdated = new Date().toISOString().split('T')[0]
}

export function sendPlaygroundMessage(projectId: string, content: string) {
  if (!mockStore.playground.messages[projectId]) {
    mockStore.playground.messages[projectId] = []
  }

  const userMsg: ChatMessage = {
    id: `msg-${Date.now()}`,
    role: 'user',
    content,
    timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  mockStore.playground.messages[projectId].push(userMsg)

  const proj = getProjectById(projectId)
  const sourceName = proj?.files[0]?.name || 'Standard_Documentation.pdf'

  // Generate realistic assistant response in local mock state
  const assistantMsg: ChatMessage = {
    id: `msg-${Date.now() + 1}`,
    role: 'assistant',
    content: `Based on ${sourceName}, the document specifies the operational guidelines and required standards relevant to your request.`,
    timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    sources: [sourceName]
  }
  mockStore.playground.messages[projectId].push(assistantMsg)

  mockStore.playground.lastRetrievedChunks[projectId] = [
    {
      id: `chunk-${Date.now()}`,
      fileName: sourceName,
      location: 'Section 1.2',
      excerpt: 'Standard operational criteria dictate compliance with all outlined procedures.'
    }
  ]
}

export function clearPlaygroundMessages(projectId: string) {
  mockStore.playground.messages[projectId] = []
  mockStore.playground.lastRetrievedChunks[projectId] = []
}
