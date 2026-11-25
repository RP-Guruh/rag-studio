<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { t } from '../i18n'
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
const setName = ref(t('Knowledge check'))
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
    const set = await createEvalSet(props.projectId, setName.value.trim() || t('Knowledge check'))
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

// Param-based summary; kept out of the template so "{{...}}" cannot close the mustache.
const reportSummary = (report: EvalReport) =>
  t('{{answerHit}}/{{total}} answers matched · {{sourceHit}}/{{total}} sources matched · {{answered}} answers returned', {
    answerHit: report.answerHit,
    sourceHit: report.sourceHit,
    total: report.total,
    answered: report.answered
  })
</script>

<template>
  <div class="evaluation-panel">
    <div v-if="error" class="eval-error">{{ error }}</div>

    <section class="eval-card">
      <header class="card-head">
        <h3 class="card-title">{{ t('Retrieval settings') }}</h3>
        <p class="card-hint">{{ t('Configure how context is retrieved before an answer is generated.') }}</p>
      </header>

      <div class="config-grid">
        <FormField :label="t('Top K')" id="eval-topk" :hint="t('Number of context chunks to retrieve.')">
          <input id="eval-topk" v-model.number="config.topK" type="number" min="1" max="20" :disabled="readonly" />
        </FormField>
        <FormField :label="t('Minimum score')" id="eval-minscore" :hint="t('Similarity score threshold.')">
          <input id="eval-minscore" v-model.number="config.minScore" type="number" min="0" max="1" step="0.05" :disabled="readonly" />
        </FormField>
        <FormField :label="t('Hybrid weight')" id="eval-hybrid-weight" :hint="t('Weight of hybrid search.')">
          <input id="eval-hybrid-weight" v-model.number="config.hybridWeight" type="number" min="0" max="1" step="0.05" :disabled="readonly" />
        </FormField>
        <FormField :label="t('Rerank weight')" id="eval-rerank-weight" :hint="t('Weight of reranking.')">
          <input id="eval-rerank-weight" v-model.number="config.rerankWeight" type="number" min="0" max="1" step="0.05" :disabled="readonly" />
        </FormField>
      </div>

      <div class="toggle-box">
        <span class="toggle-title">{{ t('Search options') }}</span>
        <div class="config-toggles">
          <label><input v-model="config.hybrid" type="checkbox" :disabled="readonly" /> {{ t('Hybrid search') }}</label>
          <label><input v-model="config.rerank" type="checkbox" :disabled="readonly" /> {{ t('Rerank results') }}</label>
          <label><input v-model="config.guardrails" type="checkbox" :disabled="readonly" /> {{ t('Guardrails') }}</label>
        </div>
      </div>

      <div class="card-actions">
        <BaseButton v-if="!readonly" variant="outline" size="sm" type="button" @click="saveConfig">
          {{ t('Save retrieval settings') }}
        </BaseButton>
      </div>
    </section>

    <section class="eval-card">
      <header class="card-head">
        <h3 class="card-title">{{ t('Evaluation sets') }}</h3>
        <p class="card-hint">{{ t('Collect test questions to measure answer quality.') }}</p>
      </header>

      <div class="set-create-row">
        <FormField :label="t('Set name')" id="eval-set-name" :hint="t('Name of the new set.')">
          <input id="eval-set-name" v-model="setName" type="text" :disabled="readonly" />
        </FormField>
        <BaseButton v-if="!readonly" variant="outline" size="sm" type="button" @click="handleCreateSet">
          {{ t('Create set') }}
        </BaseButton>
      </div>

      <label class="stacked-label">
        {{ t('Active set') }}
        <select v-model="selectedSetId" :disabled="readonly" @change="loadCases">
          <option value="">{{ t('Choose a set…') }}</option>
          <option v-for="set in sets" :key="set.id" :value="set.id">{{ set.name }}</option>
        </select>
      </label>

      <p v-if="sets.length === 0" class="empty-note">{{ t('No evaluation sets yet. Create one first.') }}</p>
    </section>

    <section v-if="selectedSetId" class="eval-card">
      <header class="card-head">
        <h3 class="card-title">{{ t('Test cases') }}</h3>
        <p class="card-hint">{{ t('Add questions and their expected answers.') }}</p>
      </header>

      <div class="case-form-grid">
        <FormField :label="t('Question')" id="eval-question">
          <input id="eval-question" v-model="question" type="text" :placeholder="t('Question to ask the project')" :disabled="readonly" />
        </FormField>
        <FormField :label="t('Expected answer (optional)')" id="eval-expected-answer">
          <input id="eval-expected-answer" v-model="expectedAnswer" type="text" :disabled="readonly" />
        </FormField>
        <FormField :label="t('Expected source file (optional)')" id="eval-expected-source">
          <input id="eval-expected-source" v-model="expectedSource" type="text" :placeholder="t('menu.pdf')" :disabled="readonly" />
        </FormField>
      </div>

      <div class="card-actions">
        <BaseButton v-if="!readonly" variant="outline" size="sm" type="button" @click="handleAddCase">
          {{ t('Add case') }}
        </BaseButton>
        <BaseButton
          v-if="!readonly"
          variant="primary"
          size="sm"
          type="button"
          :disabled="busy || cases.length === 0"
          @click="handleRun"
        >
          {{ busy ? t('Running…') : t('Run evaluation') }}
        </BaseButton>
      </div>

      <p v-if="cases.length === 0" class="empty-note">{{ t('No test cases in this set yet.') }}</p>
      <div v-else class="case-list">
        <div v-for="item in cases" :key="item.id" class="case-item">
          <span class="case-question">{{ item.question }}</span>
          <span v-if="item.expectedAnswer" class="case-expected">{{ item.expectedAnswer }}</span>
        </div>
      </div>
    </section>

    <section v-if="report" class="eval-card">
      <header class="card-head">
        <h3 class="card-title">{{ t('Result') }}</h3>
        <p class="card-hint">{{ reportSummary(report) }}</p>
      </header>

      <div class="result-list">
        <article v-for="(result, index) in report.results" :key="index" class="result-item">
          <header class="result-head">
            <div class="result-question">{{ index + 1 }}. {{ result.question }}</div>
            <div class="result-flags">
              <span :class="result.answerMatched ? 'flag-ok' : 'flag-bad'">
                {{ result.answerMatched ? t('answer OK') : t('answer unchecked') }}
              </span>
              <span :class="result.sourceMatched ? 'flag-ok' : 'flag-bad'">
                {{ result.sourceMatched ? t('source OK') : t('source miss') }}
              </span>
              <span v-if="result.noEvidence" class="flag-bad">{{ t('no evidence') }}</span>
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
