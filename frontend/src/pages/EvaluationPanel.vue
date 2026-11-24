<script setup lang="ts">
import { onMounted, ref } from 'vue'
import BaseButton from '../components/BaseButton.vue'
import FormField from '../components/FormField.vue'
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

const props = defineProps<{ projectId: string }>()

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
  error.value = ''
  try {
    await updateRetrievalConfig(props.projectId, config.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function handleCreateSet() {
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
  if (!selectedSetId.value || !question.value.trim()) return
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
  if (!selectedSetId.value) return
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
      <h3 class="card-title">Retrieval settings</h3>
      <div class="config-grid">
        <FormField label="Top K" id="eval-topk">
          <input id="eval-topk" v-model.number="config.topK" type="number" min="1" max="20" />
        </FormField>
        <FormField label="Minimum score" id="eval-minscore">
          <input id="eval-minscore" v-model.number="config.minScore" type="number" min="0" max="1" step="0.05" />
        </FormField>
        <FormField label="Hybrid weight" id="eval-hybrid-weight">
          <input id="eval-hybrid-weight" v-model.number="config.hybridWeight" type="number" min="0" max="1" step="0.05" />
        </FormField>
        <FormField label="Rerank weight" id="eval-rerank-weight">
          <input id="eval-rerank-weight" v-model.number="config.rerankWeight" type="number" min="0" max="1" step="0.05" />
        </FormField>
      </div>
      <div class="config-toggles">
        <label><input v-model="config.hybrid" type="checkbox" /> Hybrid search</label>
        <label><input v-model="config.rerank" type="checkbox" /> Rerank results</label>
        <label><input v-model="config.guardrails" type="checkbox" /> Guardrails</label>
      </div>
      <BaseButton variant="outline" size="sm" type="button" @click="saveConfig">Save retrieval settings</BaseButton>
    </section>

    <section class="eval-card">
      <h3 class="card-title">Evaluation sets</h3>
      <div class="config-grid">
        <FormField label="Set name" id="eval-set-name">
          <input id="eval-set-name" v-model="setName" type="text" />
        </FormField>
        <div class="set-actions">
          <BaseButton variant="outline" size="sm" type="button" @click="handleCreateSet">Create set</BaseButton>
        </div>
      </div>
      <label class="stacked-label">
        Active set
        <select v-model="selectedSetId" @change="loadCases">
          <option v-for="set in sets" :key="set.id" :value="set.id">{{ set.name }}</option>
        </select>
      </label>
    </section>

    <section v-if="selectedSetId" class="eval-card">
      <h3 class="card-title">Test cases</h3>
      <div class="config-grid">
        <FormField label="Question" id="eval-question">
          <input id="eval-question" v-model="question" type="text" placeholder="Question to ask the project" />
        </FormField>
        <FormField label="Expected answer (optional)" id="eval-expected-answer">
          <input id="eval-expected-answer" v-model="expectedAnswer" type="text" />
        </FormField>
        <FormField label="Expected source file (optional)" id="eval-expected-source">
          <input id="eval-expected-source" v-model="expectedSource" type="text" placeholder="menu.pdf" />
        </FormField>
      </div>
      <div class="row-actions">
        <BaseButton variant="outline" size="sm" type="button" @click="handleAddCase">Add case</BaseButton>
        <BaseButton variant="primary" size="sm" type="button" :disabled="busy || cases.length === 0" @click="handleRun">
          {{ busy ? 'Running…' : 'Run evaluation' }}
        </BaseButton>
      </div>

      <div v-if="cases.length === 0" class="muted">No cases yet. Add a question above.</div>
      <div v-else class="case-list">
        <div v-for="item in cases" :key="item.id" class="case-item">{{ item.question }}</div>
      </div>
    </section>

    <section v-if="report" class="eval-card">
      <h3 class="card-title">Result</h3>
      <p class="muted">
        {{ report.answerHit }}/{{ report.total }} answers matched ·
        {{ report.sourceHit }}/{{ report.total }} sources matched ·
        {{ report.answered }} answers returned
      </p>
      <div class="result-list">
        <div v-for="(result, index) in report.results" :key="index" class="result-item">
          <div class="result-question">{{ result.question }}</div>
          <div class="result-flags">
            <span :class="result.answerMatched ? 'flag-ok' : 'flag-bad'">
              {{ result.answerMatched ? 'answer OK' : 'answer unchecked' }}
            </span>
            <span :class="result.sourceMatched ? 'flag-ok' : 'flag-bad'">
              {{ result.sourceMatched ? 'source OK' : 'source miss' }}
            </span>
            <span v-if="result.noEvidence" class="flag-bad">no evidence</span>
          </div>
          <div class="result-answer">{{ result.answer }}</div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.evaluation-panel {
  display: flex;
  flex-direction: column;
  gap: var(--space-16);
}

.eval-card {
  padding: var(--space-16);
  background-color: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  display: flex;
  flex-direction: column;
  gap: var(--space-12);
}

.card-title {
  font-size: var(--font-size-base);
  margin: 0;
}

.muted {
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
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: var(--space-12);
}

.config-toggles,
.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-16);
  align-items: center;
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
}

.case-list,
.result-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-8);
}

.case-item,
.result-item {
  padding: var(--space-8) var(--space-12);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
}

.result-question {
  font-weight: var(--font-weight-medium);
}

.result-flags {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-8);
  margin: var(--space-4) 0;
  font-size: var(--font-size-sm);
}

.flag-ok {
  color: var(--color-success);
}

.flag-bad {
  color: var(--color-danger);
}

.result-answer {
  color: var(--color-text-muted);
}
</style>
