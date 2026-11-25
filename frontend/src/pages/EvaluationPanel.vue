<script setup lang="ts">
import { onMounted, ref } from 'vue'
import BaseButton from '../components/BaseButton.vue'
import FormField from '../components/FormField.vue'
import { renderMarkdown } from '../utils/markdown'
import {
  getRetrievalConfig,
  updateRetrievalConfig,
  listEvalSets,
  createEvalSet,
  listEvalCases,
  addEvalCase,
  runEvaluation,
  type RetrievalConfig,
  type EvalSet,
  type EvalCase,
  type EvalReport
} from '../services/projects'

const props = defineProps<{ projectId: string; readonly?: boolean }>()

const config = ref<RetrievalConfig>({
  topK: 8,
  minScore: 0,
  hybrid: false,
  rerank: false,
  guardrails: false,
  hybridWeight: 0.25,
  rerankWeight: 0.25
})
const sets = ref<EvalSet[]>([])
const selectedSetId = ref('')
const cases = ref<EvalCase[]>([])
const report = ref<EvalReport | null>(null)
const setName = ref('Knowledge check')
const question = ref('')
const expectedAnswer = ref('')
const expectedSource = ref('')
const error = ref('')
const busy = ref(false)

onMounted(async () => {
  await reload()
})

async function reload() {
  error.value = ''
  try {
    config.value = await getRetrievalConfig(props.projectId)
    sets.value = await listEvalSets(props.projectId)
    if (sets.value.length > 0 && !selectedSetId.value) {
      selectedSetId.value = sets.value[0].id
      await loadCases()
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function loadCases() {
  if (!selectedSetId.value) return
  cases.value = await listEvalCases(selectedSetId.value)
  report.value = null
}

async function saveConfig() {
  if (props.readonly) return
  error.value = ''
  try {
    await updateRetrievalConfig(props.projectId, config.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function handleCreateSet() {
  if (props.readonly) return
  error.value = ''
  try {
    const set = await createEvalSet(props.projectId, setName.value.trim() || 'Knowledge check')
    await reload()
    selectedSetId.value = set.id
    await loadCases()
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function handleAddCase() {
  if (props.readonly || !selectedSetId.value || !question.value.trim()) return
  error.value = ''
  try {
    await addEvalCase(selectedSetId.value, question.value.trim(), expectedAnswer.value.trim(), expectedSource.value.trim())
    question.value = ''
    expectedAnswer.value = ''
    expectedSource.value = ''
    await loadCases()
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function handleRun() {
  if (props.readonly || !selectedSetId.value) return
  busy.value = true
  error.value = ''
  try {
    report.value = await runEvaluation(selectedSetId.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="evaluation-panel">
    <div v-if="error" class="eval-error">{{ error }}</div>

    <section class="eval-card">
      <header class="card-head">
        <h3 class="card-title">Retrieval settings</h3>
        <p class="card-hint">Atur bagaimana konteks diambil sebelum jawaban dibuat.</p>
      </header>

      <div class="config-grid">
        <FormField label="Top K" id="eval-topk" hint="Jumlah potongan konteks yang diambil.">
          <input id="eval-topk" v-model.number="config.topK" type="number" min="1" max="20" :disabled="readonly" />
        </FormField>
        <FormField label="Minimum score" id="eval-minscore" hint="Ambang skor kemiripan.">
          <input id="eval-minscore" v-model.number="config.minScore" type="number" min="0" max="1" step="0.05" :disabled="readonly" />
        </FormField>
        <FormField label="Hybrid weight" id="eval-hybrid-weight" hint="Bobot pencarian hibrida.">
          <input id="eval-hybrid-weight" v-model.number="config.hybridWeight" type="number" min="0" max="1" step="0.05" :disabled="readonly" />
        </FormField>
        <FormField label="Rerank weight" id="eval-rerank-weight" hint="Bobot peninjauan ulang.">
          <input id="eval-rerank-weight" v-model.number="config.rerankWeight" type="number" min="0" max="1" step="0.05" :disabled="readonly" />
        </FormField>
      </div>

      <div class="toggle-box">
        <span class="toggle-title">Opsi pencarian</span>
        <div class="config-toggles">
          <label><input v-model="config.hybrid" type="checkbox" :disabled="readonly" /> Hybrid search</label>
          <label><input v-model="config.rerank" type="checkbox" :disabled="readonly" /> Rerank results</label>
          <label><input v-model="config.guardrails" type="checkbox" :disabled="readonly" /> Guardrails</label>
        </div>
      </div>

      <div class="card-actions">
        <BaseButton v-if="!readonly" variant="outline" size="sm" type="button" @click="saveConfig">
          Save retrieval settings
        </BaseButton>
      </div>
    </section>

    <section class="eval-card">
      <header class="card-head">
        <h3 class="card-title">Evaluation sets</h3>
        <p class="card-hint">Kumpulkan pertanyaan uji untuk mengukur kualitas jawaban.</p>
      </header>

      <div class="set-create-row">
        <FormField label="Set name" id="eval-set-name" hint="Nama set baru.">
          <input id="eval-set-name" v-model="setName" type="text" :disabled="readonly" />
        </FormField>
        <BaseButton v-if="!readonly" variant="outline" size="sm" type="button" @click="handleCreateSet">
          Create set
        </BaseButton>
      </div>

      <label class="stacked-label">
        Active set
        <select v-model="selectedSetId" :disabled="readonly" @change="loadCases">
          <option value="">Pilih set…</option>
          <option v-for="set in sets" :key="set.id" :value="set.id">{{ set.name }}</option>
        </select>
      </label>

      <p v-if="sets.length === 0" class="empty-note">Belum ada evaluation set. Buat set terlebih dahulu.</p>
    </section>

    <section v-if="selectedSetId" class="eval-card">
      <header class="card-head">
        <h3 class="card-title">Test cases</h3>
        <p class="card-hint">Tambahkan pertanyaan dan jawaban yang diharapkan.</p>
      </header>

      <div class="case-form-grid">
        <FormField label="Question" id="eval-question">
          <input id="eval-question" v-model="question" type="text" placeholder="Question to ask the project" :disabled="readonly" />
        </FormField>
        <FormField label="Expected answer (optional)" id="eval-expected-answer">
          <input id="eval-expected-answer" v-model="expectedAnswer" type="text" :disabled="readonly" />
        </FormField>
        <FormField label="Expected source file (optional)" id="eval-expected-source">
          <input id="eval-expected-source" v-model="expectedSource" type="text" placeholder="menu.pdf" :disabled="readonly" />
        </FormField>
      </div>

      <div class="card-actions">
        <BaseButton v-if="!readonly" variant="outline" size="sm" type="button" @click="handleAddCase">
          Add case
        </BaseButton>
        <BaseButton
          v-if="!readonly"
          variant="primary"
          size="sm"
          type="button"
          :disabled="busy || cases.length === 0"
          @click="handleRun"
        >
          {{ busy ? 'Running…' : 'Run evaluation' }}
        </BaseButton>
      </div>

      <p v-if="cases.length === 0" class="empty-note">Belum ada test case pada set ini.</p>
      <div v-else class="case-list">
        <div v-for="item in cases" :key="item.id" class="case-item">
          <span class="case-question">{{ item.question }}</span>
          <span v-if="item.expectedAnswer" class="case-expected">{{ item.expectedAnswer }}</span>
        </div>
      </div>
    </section>

    <section v-if="report" class="eval-card">
      <header class="card-head">
        <h3 class="card-title">Result</h3>
        <p class="card-hint">
          {{ report.answerHit }}/{{ report.total }} jawaban cocok ·
          {{ report.sourceHit }}/{{ report.total }} sumber cocok ·
          {{ report.answered }} jawaban dikembalikan
        </p>
      </header>

      <div class="result-list">
        <article v-for="(result, index) in report.results" :key="index" class="result-item">
          <header class="result-head">
            <div class="result-question">{{ index + 1 }}. {{ result.question }}</div>
            <div class="result-flags">
              <span :class="result.answerMatched ? 'flag-ok' : 'flag-bad'">
                {{ result.answerMatched ? 'answer OK' : 'answer unchecked' }}
              </span>
              <span :class="result.sourceMatched ? 'flag-ok' : 'flag-bad'">
                {{ result.sourceMatched ? 'source OK' : 'source miss' }}
              </span>
              <span v-if="result.noEvidence" class="flag-bad">no evidence</span>
            </div>
          </header>
          <div class="result-answer markdown-body" v-html="renderMarkdown(result.answer)" />
        </article>
      </div>
    </section>
  </div>
</template>

<style scoped>
.evaluation-panel {
  display: flex;
  flex-direction: column;
  gap: var(--space-24);
  width: 100%;
  max-width: 960px;
  margin: 0 auto;
}

.eval-card {
  padding: var(--space-20);
  background-color: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  display: flex;
  flex-direction: column;
  gap: var(--space-16);
}

.card-head {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-12);
  border-bottom: 1px solid var(--color-border);
}

.card-title {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  margin: 0;
}

.card-hint,
.empty-note {
  color: var(--color-text-muted);
  font-size: var(--font-size-sm);
  margin: 0;
}

.eval-error {
  padding: var(--space-8) var(--space-12);
  border: 1px solid var(--color-danger);
  color: var(--color-danger);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
}

.config-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-16);
}

.case-form-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-12);
}

.set-create-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: var(--space-12);
  align-items: end;
}

.toggle-box {
  padding: var(--space-12);
  background-color: var(--color-surface-subtle);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  display: flex;
  flex-direction: column;
  gap: var(--space-8);
}

.toggle-title {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
}

.config-toggles {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-16);
}

.config-toggles label {
  display: flex;
  align-items: center;
  gap: var(--space-8);
  font-size: var(--font-size-sm);
}

.stacked-label {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  font-size: var(--font-size-sm);
  max-width: 420px;
}

.card-actions {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: var(--space-8);
  padding-top: var(--space-4);
}

.case-list,
.result-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-8);
}

.case-item,
.result-item {
  padding: var(--space-12) var(--space-16);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  display: flex;
  flex-direction: column;
  gap: var(--space-8);
}

.case-item {
  flex-direction: row;
  justify-content: space-between;
  gap: var(--space-16);
}

.case-question {
  font-weight: var(--font-weight-medium);
}

.case-expected {
  color: var(--color-text-muted);
}

.result-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-8);
}

.result-question {
  font-weight: var(--font-weight-medium);
}

.result-flags {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-8);
  font-size: var(--font-size-sm);
}

.flag-ok {
  color: var(--color-success);
}

.flag-bad {
  color: var(--color-danger);
}

.result-answer {
  padding: var(--space-12);
  background-color: var(--color-surface-subtle);
  border-radius: var(--radius-sm);
  color: var(--color-text);
}

@media (max-width: 900px) {
  .config-grid,
  .case-form-grid {
    grid-template-columns: 1fr;
  }

  .set-create-row {
    grid-template-columns: 1fr;
  }
}
</style>
