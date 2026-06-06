<template>
  <div class="code-tabs">
    <div class="tabs-header">
      <button
        v-for="(tab, index) in tabs"
        :key="tab.id"
        class="tab-button"
        :class="{ active: activeTab === index }"
        @click="activeTab = index"
      >
        <span class="tab-number">{{ index + 1 }}</span>
        <span class="tab-label">{{ tab.label }}</span>
      </button>
    </div>
    <div class="tab-content">
      <div class="code-header">
        <span class="code-filename">{{ tabs[activeTab].filename }}</span>
        <button class="copy-button" @click="copyCode" :class="{ copied }">
          <span v-if="copied">Copied!</span>
          <span v-else>Copy</span>
        </button>
      </div>
      <pre class="code-block"><code v-html="tabs[activeTab].code"></code></pre>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const activeTab = ref(0)
const copied = ref(false)

const tabs = [
  {
    id: 'install',
    label: 'Install',
    filename: 'terminal',
    code: `<span class="comment"># Add agent-go to your project</span>
go get go.klarlabs.de/agent`
  },
  {
    id: 'tool',
    label: 'Define Tool',
    filename: 'tools.go',
    code: `<span class="keyword">package</span> main

<span class="keyword">import</span> (
    api <span class="string">"go.klarlabs.de/agent/interfaces/api"</span>
)

<span class="comment">// Define a read-only tool</span>
<span class="keyword">var</span> readFileTool = api.NewToolBuilder(<span class="string">"read_file"</span>).
    WithDescription(<span class="string">"Reads contents of a file"</span>).
    WithAnnotations(api.Annotations{
        ReadOnly: <span class="keyword">true</span>,
    }).
    WithExecutor(<span class="keyword">func</span>(ctx context.Context, input json.RawMessage) (tool.Result, error) {
        <span class="keyword">var</span> req ReadFileInput
        <span class="keyword">if</span> err := json.Unmarshal(input, &req); err != <span class="keyword">nil</span> {
            <span class="keyword">return</span> tool.Result{}, err
        }
        data, err := os.ReadFile(req.Path)
        <span class="keyword">if</span> err != <span class="keyword">nil</span> {
            <span class="keyword">return</span> tool.Result{}, err
        }
        <span class="keyword">return</span> tool.Result{Output: json.RawMessage(data)}, <span class="keyword">nil</span>
    }).
    Build()`
  },
  {
    id: 'engine',
    label: 'Create Engine',
    filename: 'main.go',
    code: `<span class="keyword">package</span> main

<span class="keyword">import</span> (
    api <span class="string">"go.klarlabs.de/agent/interfaces/api"</span>
)

<span class="keyword">func</span> main() {
    <span class="comment">// Create tool registry</span>
    registry := api.NewToolRegistry()
    registry.Register(readFileTool)

    <span class="comment">// Configure tool eligibility per state</span>
    eligibility := api.NewToolEligibility()
    eligibility.Allow(api.StateExplore, <span class="string">"read_file"</span>)

    <span class="comment">// Build engine with policies</span>
    engine, err := api.New(
        api.WithRegistry(registry),
        api.WithPlanner(myPlanner),
        api.WithToolEligibility(eligibility),
        api.WithTransitions(api.DefaultTransitions()),
        api.WithBudgets(<span class="keyword">map</span>[<span class="keyword">string</span>]<span class="keyword">int</span>{
            <span class="string">"tool_calls"</span>: <span class="number">100</span>,
        }),
        api.WithMaxSteps(<span class="number">50</span>),
    )
    <span class="keyword">if</span> err != <span class="keyword">nil</span> {
        log.Fatal(err)
    }

    <span class="comment">// Run the agent</span>
    run, err := engine.Run(ctx, <span class="string">"Analyze the config files"</span>)
    <span class="keyword">if</span> err != <span class="keyword">nil</span> {
        log.Fatal(err)
    }

    fmt.Printf(<span class="string">"Result: %s\\n"</span>, run.Result)
}`
  },
  {
    id: 'test',
    label: 'Test',
    filename: 'main_test.go',
    code: `<span class="keyword">package</span> main

<span class="keyword">import</span> (
    <span class="string">"testing"</span>
    api <span class="string">"go.klarlabs.de/agent/interfaces/api"</span>
)

<span class="keyword">func</span> TestAgentExecution(t *testing.T) {
    <span class="comment">// Use ScriptedPlanner for deterministic tests</span>
    planner := api.NewScriptedPlanner(
        api.ScriptStep{
            ExpectState: api.StateIntake,
            Decision:    api.NewTransitionDecision(api.StateExplore, <span class="string">"begin"</span>),
        },
        api.ScriptStep{
            ExpectState: api.StateExplore,
            Decision:    api.NewCallToolDecision(<span class="string">"read_file"</span>, input, <span class="string">"read config"</span>),
        },
        api.ScriptStep{
            ExpectState: api.StateExplore,
            Decision:    api.NewFinishDecision(<span class="string">"analysis complete"</span>, result),
        },
    )

    engine, _ := api.New(
        api.WithPlanner(planner),
        <span class="comment">// ... other options</span>
    )

    run, err := engine.Run(ctx, <span class="string">"Analyze the config"</span>)
    <span class="keyword">if</span> err != <span class="keyword">nil</span> {
        t.Fatalf(<span class="string">"unexpected error: %v"</span>, err)
    }

    <span class="keyword">if</span> run.State != api.StateDone {
        t.Errorf(<span class="string">"expected done, got %s"</span>, run.State)
    }
}`
  }
]

const copyCode = async () => {
  const text = tabs[activeTab.value].code.replace(/<[^>]*>/g, '')
  try {
    await navigator.clipboard.writeText(text)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch (err) {
    console.error('Failed to copy:', err)
  }
}
</script>

<style scoped>
.code-tabs {
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: 12px;
  overflow: hidden;
}

.tabs-header {
  display: flex;
  background: var(--bg-tertiary);
  border-bottom: 1px solid var(--border-primary);
  overflow-x: auto;
}

.tab-button {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  padding: var(--space-md) var(--space-lg);
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.8rem;
  cursor: pointer;
  transition: all var(--transition-fast);
  white-space: nowrap;
}

.tab-button:hover {
  color: var(--text-primary);
  background: var(--bg-secondary);
}

.tab-button.active {
  color: var(--accent-cyan);
  border-bottom-color: var(--accent-cyan);
  background: var(--bg-secondary);
}

.tab-number {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  background: var(--bg-tertiary);
  border-radius: 4px;
  font-size: 0.7rem;
  font-weight: 600;
}

.tab-button.active .tab-number {
  background: var(--accent-cyan);
  color: var(--bg-primary);
}

.tab-content {
  position: relative;
}

.code-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-sm) var(--space-md);
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-secondary);
}

.code-filename {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-muted);
}

.copy-button {
  padding: var(--space-xs) var(--space-sm);
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: 4px;
  color: var(--text-secondary);
  font-family: var(--font-mono);
  font-size: 0.7rem;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.copy-button:hover {
  border-color: var(--accent-cyan);
  color: var(--accent-cyan);
}

.copy-button.copied {
  background: var(--accent-green);
  border-color: var(--accent-green);
  color: var(--bg-primary);
}

.code-block {
  margin: 0;
  padding: var(--space-lg);
  overflow-x: auto;
  font-family: var(--font-mono);
  font-size: 0.8rem;
  line-height: 1.7;
  color: var(--text-primary);
}

.code-block code {
  background: none;
  border: none;
  padding: 0;
}

/* Syntax highlighting */
:deep(.keyword) {
  color: #a371f7;
}

:deep(.string) {
  color: #3fb950;
}

:deep(.comment) {
  color: #484f58;
  font-style: italic;
}

:deep(.number) {
  color: #ffb000;
}

@media (max-width: 768px) {
  .tab-button {
    padding: var(--space-sm) var(--space-md);
  }

  .tab-label {
    display: none;
  }

  .tab-number {
    width: 24px;
    height: 24px;
  }
}
</style>
