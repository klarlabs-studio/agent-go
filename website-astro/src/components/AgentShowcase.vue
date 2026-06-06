<template>
  <div class="agent-showcase-tabs">
    <!-- Tab Buttons -->
    <div class="tab-buttons">
      <button
        v-for="(scenario, index) in scenarios"
        :key="scenario.id"
        class="tab-button"
        :class="{ active: activeTab === index }"
        @click="switchTab(index)"
      >
        <span class="tab-icon">{{ scenario.icon }}</span>
        <span class="tab-label">{{ scenario.name }}</span>
      </button>
    </div>

    <!-- Active Scenario -->
    <div class="scenario-container">
      <div class="scenario-header">
        <div class="scenario-title-row">
          <h3>{{ activeScenario.title }}</h3>
          <a :href="activeScenario.githubUrl" class="view-code-link" target="_blank" rel="noopener">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
            </svg>
            <span>View Code</span>
          </a>
        </div>
        <p>{{ activeScenario.description }}</p>
      </div>

      <div class="showcase-grid">
        <!-- Tools Panel -->
        <div class="showcase-tools">
          <h4>Available Tools</h4>
          <div class="tool-list">
            <div
              v-for="tool in activeScenario.tools"
              :key="tool.name"
              class="tool-card"
              :class="{ 'tool-active': isToolActive(tool.name) }"
            >
              <div class="tool-header">
                <code>{{ tool.name }}</code>
                <span class="tool-badge" :class="tool.badge">{{ tool.badgeText }}</span>
              </div>
              <p>{{ tool.description }}</p>
            </div>
          </div>
        </div>

        <!-- Execution Demo -->
        <div class="showcase-demo" @click="togglePause">
          <h4>Execution Trace</h4>

          <!-- Progress Bar -->
          <div class="progress-bar">
            <div
              v-for="(state, index) in stateSequence"
              :key="index"
              class="progress-step"
              :class="{
                active: index <= currentStateIndex,
                current: index === currentStateIndex
              }"
            >
              <span class="progress-dot" :class="state"></span>
              <span class="progress-label">{{ state }}</span>
            </div>
          </div>

          <!-- Execution Trace -->
          <div class="execution-trace">
            <TransitionGroup name="step">
              <div
                v-for="(step, index) in visibleSteps"
                :key="step.id"
                class="trace-step"
                :class="{ latest: index === visibleSteps.length - 1 }"
              >
                <div class="step-header">
                  <span class="step-state" :class="step.state">{{ step.state }}</span>
                  <span class="step-action">{{ step.action }}</span>
                </div>

                <div v-if="step.type === 'tool'" class="step-tool">
                  <div class="tool-io">
                    <div class="io-row">
                      <span class="io-label">Input:</span>
                      <code class="io-value">{{ formatJson(step.input) }}</code>
                    </div>
                    <div class="io-row">
                      <span class="io-label">Output:</span>
                      <code class="io-value" :class="{ success: step.success, warning: step.warning }">{{ formatJson(step.output) }}</code>
                    </div>
                  </div>
                </div>

                <div v-else-if="step.type === 'transition'" class="step-description">
                  {{ step.description }}
                </div>

                <div v-else-if="step.type === 'human'" class="step-human">
                  <span class="human-icon">👤</span>
                  {{ step.description }}
                </div>

                <div v-else-if="step.type === 'result'" class="step-result">
                  <span class="result-icon">✓</span>
                  {{ step.description }}
                </div>
              </div>
            </TransitionGroup>
          </div>

          <!-- Footer -->
          <div class="demo-footer">
            <div class="footer-left">
              <span class="pause-hint">{{ isPaused ? '▶ Click to resume' : '⏸ Click to pause' }}</span>
            </div>
            <div class="footer-right">
              <span class="step-counter">step {{ currentStepIndex + 1 }}/{{ activeScenario.steps.length }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'

const stateSequence = ['intake', 'explore', 'decide', 'act', 'validate', 'done']

const scenarios = [
  {
    id: 'fileops',
    name: 'File Operations',
    icon: '📁',
    title: 'File Operations Agent',
    description: 'Manages files in a workspace: read, write, list, and delete operations.',
    githubUrl: 'https://github.com/klarlabs-studio/agent-go/tree/main/example/fileops',
    tools: [
      { name: 'list_dir', badge: 'readonly', badgeText: 'ReadOnly', description: 'List files and directories' },
      { name: 'read_file', badge: 'readonly', badgeText: 'ReadOnly', description: 'Read contents of a file' },
      { name: 'write_file', badge: 'idempotent', badgeText: 'Idempotent', description: 'Write content to a file' },
      { name: 'delete_file', badge: 'destructive', badgeText: 'Destructive', description: 'Delete a file (requires approval)' }
    ],
    steps: [
      { id: 1, state: 'intake', type: 'transition', action: 'Goal received', description: 'Create a hello.txt file with a greeting message' },
      { id: 2, state: 'explore', type: 'tool', action: 'list_dir', input: { path: '.' }, output: { files: [], count: 0 }, success: true },
      { id: 3, state: 'decide', type: 'transition', action: 'Planning', description: 'Directory is empty. Will create hello.txt file.' },
      { id: 4, state: 'act', type: 'tool', action: 'write_file', input: { path: 'hello.txt', content: 'Hello!' }, output: { bytes: 6, created: true }, success: true },
      { id: 5, state: 'validate', type: 'tool', action: 'read_file', input: { path: 'hello.txt' }, output: { content: 'Hello!', size: 6 }, success: true },
      { id: 6, state: 'done', type: 'result', action: 'Complete', description: 'File created and verified successfully' }
    ]
  },
  {
    id: 'support',
    name: 'Customer Support',
    icon: '🎧',
    title: 'Customer Support Agent',
    description: 'Handles support tickets: lookup customers, check orders, search knowledge base, escalate issues.',
    githubUrl: 'https://github.com/klarlabs-studio/agent-go/tree/main/example/customer-support',
    tools: [
      { name: 'lookup_customer', badge: 'readonly', badgeText: 'ReadOnly', description: 'Find customer by email or ID' },
      { name: 'get_order_status', badge: 'readonly', badgeText: 'ReadOnly', description: 'Check order shipping status' },
      { name: 'search_kb', badge: 'readonly', badgeText: 'ReadOnly', description: 'Search knowledge base articles' },
      { name: 'create_ticket', badge: 'idempotent', badgeText: 'Idempotent', description: 'Create a support ticket' },
      { name: 'escalate', badge: 'destructive', badgeText: 'Human', description: 'Escalate to human agent' }
    ],
    steps: [
      { id: 1, state: 'intake', type: 'transition', action: 'Ticket received', description: '"Where is my order #38291? It\'s been 2 weeks!"' },
      { id: 2, state: 'explore', type: 'tool', action: 'lookup_customer', input: { email: 'jane@email.com' }, output: { id: 'cust_847', name: 'Jane Smith', tier: 'premium' }, success: true },
      { id: 3, state: 'explore', type: 'tool', action: 'get_order_status', input: { order_id: '38291' }, output: { status: 'delayed', carrier: 'FedEx', eta: '2 days' }, success: true, warning: true },
      { id: 4, state: 'decide', type: 'transition', action: 'Analyzing', description: 'Order delayed. Premium customer. Will offer compensation.' },
      { id: 5, state: 'act', type: 'tool', action: 'create_ticket', input: { type: 'shipping_delay', priority: 'high' }, output: { ticket_id: 'TKT-9921', status: 'open' }, success: true },
      { id: 6, state: 'validate', type: 'tool', action: 'search_kb', input: { query: 'shipping delay compensation' }, output: { article: 'POL-201', action: '10% refund' }, success: true },
      { id: 7, state: 'done', type: 'result', action: 'Resolved', description: 'Ticket created. 10% refund applied. ETA shared with customer.' }
    ]
  },
  {
    id: 'devops',
    name: 'DevOps Monitor',
    icon: '🔧',
    title: 'DevOps Monitoring Agent',
    description: 'Monitors infrastructure: check metrics, analyze logs, restart services, send alerts.',
    githubUrl: 'https://github.com/klarlabs-studio/agent-go/tree/main/example/devops-monitor',
    tools: [
      { name: 'get_metrics', badge: 'readonly', badgeText: 'ReadOnly', description: 'Fetch service health metrics' },
      { name: 'query_logs', badge: 'readonly', badgeText: 'ReadOnly', description: 'Search application logs' },
      { name: 'restart_service', badge: 'destructive', badgeText: 'Destructive', description: 'Restart a service (requires approval)' },
      { name: 'send_alert', badge: 'idempotent', badgeText: 'Idempotent', description: 'Send alert to on-call team' }
    ],
    steps: [
      { id: 1, state: 'intake', type: 'transition', action: 'Alert triggered', description: 'High error rate detected on api-gateway service' },
      { id: 2, state: 'explore', type: 'tool', action: 'get_metrics', input: { service: 'api-gateway' }, output: { cpu: '23%', memory: '67%', errors: '847/min' }, success: true, warning: true },
      { id: 3, state: 'explore', type: 'tool', action: 'query_logs', input: { service: 'api-gateway', level: 'error', limit: 10 }, output: { pattern: 'connection pool exhausted', count: 312 }, success: true },
      { id: 4, state: 'decide', type: 'transition', action: 'Diagnosing', description: 'Connection pool exhausted. Service restart recommended.' },
      { id: 5, state: 'decide', type: 'human', action: 'Approval requested', description: 'Waiting for approval to restart api-gateway...' },
      { id: 6, state: 'act', type: 'tool', action: 'restart_service', input: { service: 'api-gateway', graceful: true }, output: { status: 'restarted', downtime: '3.2s' }, success: true },
      { id: 7, state: 'validate', type: 'tool', action: 'get_metrics', input: { service: 'api-gateway' }, output: { cpu: '18%', memory: '45%', errors: '2/min' }, success: true },
      { id: 8, state: 'done', type: 'result', action: 'Resolved', description: 'Service restarted. Error rate normalized. Incident logged.' }
    ]
  }
]

const activeTab = ref(0)
const currentStepIndex = ref(0)
const isPaused = ref(false)
let interval = null

const activeScenario = computed(() => scenarios[activeTab.value])

const visibleSteps = computed(() => {
  return activeScenario.value.steps.slice(0, currentStepIndex.value + 1)
})

const currentStateIndex = computed(() => {
  const currentState = activeScenario.value.steps[currentStepIndex.value].state
  return stateSequence.indexOf(currentState)
})

const isToolActive = (toolName) => {
  const currentStep = activeScenario.value.steps[currentStepIndex.value]
  return currentStep.type === 'tool' && currentStep.action === toolName
}

const formatJson = (obj) => {
  return JSON.stringify(obj)
}

const togglePause = () => {
  isPaused.value = !isPaused.value
  if (isPaused.value) {
    clearInterval(interval)
  } else {
    startAnimation()
  }
}

const switchTab = (index) => {
  activeTab.value = index
  currentStepIndex.value = 0
  clearInterval(interval)
  isPaused.value = false
  startAnimation()
}

const startAnimation = () => {
  interval = setInterval(() => {
    if (currentStepIndex.value < activeScenario.value.steps.length - 1) {
      currentStepIndex.value++
    } else {
      setTimeout(() => {
        currentStepIndex.value = 0
      }, 2000)
    }
  }, 2000)
}

onMounted(() => {
  startAnimation()
})

onUnmounted(() => {
  if (interval) clearInterval(interval)
})
</script>

<style scoped>
.agent-showcase-tabs {
  width: 100%;
}

/* Tab Buttons */
.tab-buttons {
  display: flex;
  gap: var(--space-sm);
  margin-bottom: var(--space-xl);
  padding: var(--space-xs);
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: 12px;
}

.tab-button {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-sm);
  padding: var(--space-md) var(--space-lg);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 8px;
  cursor: pointer;
  transition: all var(--transition-fast);
  font-family: var(--font-display);
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-secondary);
}

.tab-button:hover {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}

.tab-button.active {
  background: var(--bg-primary);
  border-color: var(--accent-cyan);
  color: var(--text-primary);
  box-shadow: 0 0 0 1px rgba(0, 217, 255, 0.1);
}

.tab-icon {
  font-size: 1.1rem;
}

.tab-label {
  display: block;
}

/* Scenario Container */
.scenario-container {
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: 12px;
  overflow: hidden;
}

.scenario-header {
  padding: var(--space-lg) var(--space-xl);
  border-bottom: 1px solid var(--border-primary);
}

.scenario-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  margin-bottom: var(--space-xs);
}

.scenario-header h3 {
  font-size: 1.1rem;
  color: var(--text-primary);
  margin: 0;
}

.view-code-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: 6px;
  color: var(--text-secondary);
  font-size: 0.75rem;
  font-weight: 500;
  text-decoration: none;
  transition: all var(--transition-fast);
}

.view-code-link:hover {
  background: var(--bg-primary);
  border-color: var(--accent-cyan);
  color: var(--accent-cyan);
}

.view-code-link svg {
  width: 14px;
  height: 14px;
}

.scenario-header p {
  font-size: 0.85rem;
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.5;
}

/* Grid Layout */
.showcase-grid {
  display: grid;
  grid-template-columns: 280px 1fr;
}

.showcase-tools {
  padding: var(--space-lg);
  border-right: 1px solid var(--border-primary);
  background: var(--bg-tertiary);
}

.showcase-tools h4,
.showcase-demo h4 {
  font-size: 0.7rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--text-muted);
  margin: 0 0 var(--space-md) 0;
}

.tool-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.tool-card {
  padding: var(--space-sm) var(--space-md);
  background: var(--bg-secondary);
  border: 1px solid var(--border-secondary);
  border-radius: 6px;
  transition: all var(--transition-fast);
}

.tool-card.tool-active {
  border-color: var(--accent-cyan);
  background: var(--bg-primary);
  box-shadow: 0 0 0 1px rgba(0, 217, 255, 0.2);
}

.tool-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 2px;
}

.tool-header code {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--accent-cyan);
  background: none;
  border: none;
  padding: 0;
}

.tool-badge {
  font-size: 0.55rem;
  font-weight: 600;
  padding: 2px 5px;
  border-radius: 3px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.tool-badge.readonly { background: rgba(63, 185, 80, 0.2); color: #3fb950; }
.tool-badge.idempotent { background: rgba(0, 217, 255, 0.2); color: #00d9ff; }
.tool-badge.destructive { background: rgba(248, 81, 73, 0.2); color: #f85149; }

.tool-card p {
  font-size: 0.7rem;
  color: var(--text-muted);
  margin: 0;
  line-height: 1.3;
}

/* Demo Panel */
.showcase-demo {
  padding: var(--space-lg);
  display: flex;
  flex-direction: column;
  cursor: pointer;
}

/* Progress Bar */
.progress-bar {
  display: flex;
  justify-content: space-between;
  padding: var(--space-sm) var(--space-md);
  background: var(--bg-primary);
  border: 1px solid var(--border-secondary);
  border-radius: 8px;
  margin-bottom: var(--space-md);
  gap: var(--space-xs);
}

.progress-step {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  opacity: 0.3;
  transition: opacity var(--transition-fast);
}

.progress-step.active { opacity: 1; }
.progress-step.current .progress-dot { animation: pulse 1s infinite; }

.progress-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-muted);
}

.progress-dot.intake { background: #a371f7; }
.progress-dot.explore { background: #00d9ff; }
.progress-dot.decide { background: #ffb000; }
.progress-dot.act { background: #f85149; }
.progress-dot.validate { background: #3fb950; }
.progress-dot.done { background: #3fb950; }

.progress-label {
  font-family: var(--font-mono);
  font-size: 0.6rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.progress-step.active .progress-label { color: var(--text-secondary); }

/* Execution Trace */
.execution-trace {
  flex: 1;
  min-height: 260px;
  max-height: 300px;
  overflow-y: auto;
  padding-right: var(--space-sm);
}

.trace-step {
  padding: var(--space-sm) var(--space-md);
  margin-bottom: var(--space-xs);
  background: var(--bg-primary);
  border: 1px solid var(--border-secondary);
  border-radius: 6px;
  transition: all var(--transition-fast);
}

.trace-step.latest {
  border-color: var(--accent-cyan);
  box-shadow: 0 0 0 1px rgba(0, 217, 255, 0.1);
}

.step-header {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin-bottom: 2px;
}

.step-state {
  font-family: var(--font-mono);
  font-size: 0.6rem;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 3px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.step-state.intake { background: rgba(163, 113, 247, 0.2); color: #a371f7; }
.step-state.explore { background: rgba(0, 217, 255, 0.2); color: #00d9ff; }
.step-state.decide { background: rgba(255, 176, 0, 0.2); color: #ffb000; }
.step-state.act { background: rgba(248, 81, 73, 0.2); color: #f85149; }
.step-state.validate { background: rgba(63, 185, 80, 0.2); color: #3fb950; }
.step-state.done { background: rgba(63, 185, 80, 0.2); color: #3fb950; }

.step-action {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  font-weight: 500;
  color: var(--text-primary);
}

.step-description {
  font-size: 0.75rem;
  color: var(--text-secondary);
  line-height: 1.4;
}

.step-tool { margin-top: 2px; }

.tool-io {
  font-family: var(--font-mono);
  font-size: 0.7rem;
}

.io-row {
  display: flex;
  gap: var(--space-sm);
  padding: 2px 0;
}

.io-label {
  color: var(--text-muted);
  min-width: 45px;
}

.io-value {
  color: var(--text-secondary);
  background: var(--bg-tertiary);
  padding: 1px 4px;
  border-radius: 3px;
  word-break: break-all;
  font-size: 0.65rem;
}

.io-value.success { color: var(--accent-green); }
.io-value.warning { color: var(--accent-amber); }

.step-human {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  font-size: 0.75rem;
  color: var(--accent-amber);
  font-weight: 500;
}

.human-icon { font-size: 0.9rem; }

.step-result {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  font-size: 0.8rem;
  color: var(--accent-green);
  font-weight: 500;
}

.result-icon { font-size: 0.9rem; }

/* Footer */
.demo-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: var(--space-md);
  margin-top: var(--space-md);
  border-top: 1px solid var(--border-secondary);
  font-family: var(--font-mono);
  font-size: 0.65rem;
}

.pause-hint { color: var(--text-muted); }
.step-counter { color: var(--text-secondary); }

/* Transitions */
.step-enter-active { transition: all 0.3s ease-out; }
.step-enter-from { opacity: 0; transform: translateY(-8px); }
.step-leave-active { transition: all 0.2s ease-in; }
.step-leave-to { opacity: 0; }

@keyframes pulse {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.3); opacity: 0.8; }
}

/* Responsive */
@media (max-width: 900px) {
  .showcase-grid {
    grid-template-columns: 1fr;
  }

  .showcase-tools {
    border-right: none;
    border-bottom: 1px solid var(--border-primary);
    padding: var(--space-md);
  }

  .tool-list {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: var(--space-xs);
  }

  .execution-trace {
    min-height: 220px;
    max-height: 260px;
  }
}

@media (max-width: 640px) {
  .tab-label {
    display: none;
  }

  .tab-button {
    padding: var(--space-md);
  }

  .tab-icon {
    font-size: 1.3rem;
  }

  .progress-label {
    display: none;
  }

  .progress-bar {
    justify-content: center;
    gap: var(--space-md);
  }

  .tool-list {
    grid-template-columns: 1fr;
  }

  .scenario-header {
    padding: var(--space-md);
  }

  .showcase-demo {
    padding: var(--space-md);
  }
}
</style>
