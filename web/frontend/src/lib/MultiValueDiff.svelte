<script lang="ts">
    import { computeArrayDiff, formatDNValue, isDN } from './utils';
    import type { ArrayDiffResult } from './utils';

    interface Props {
        oldValue: unknown;
        newValue: unknown;
    }

    let { oldValue, newValue }: Props = $props();

    let diff: ArrayDiffResult = $derived(computeArrayDiff(oldValue, newValue));

    let showFullValues = $state(false);

    let hasDNValues = $derived.by(() => {
        const allValues = [...diff.added, ...diff.removed, ...diff.unchanged];
        return allValues.some(isDN);
    });

    function formatItem(value: string): string {
        if (showFullValues || !isDN(value)) {
            return value;
        }
        return formatDNValue(value);
    }

    let totalCount = $derived(diff.added.length + diff.removed.length + diff.unchanged.length);
</script>

<div class="multi-value-diff">
    <div class="diff-header">
        <div class="diff-summary">
            {#if diff.added.length > 0}
                <span class="badge added">+{diff.added.length} added</span>
            {/if}
            {#if diff.removed.length > 0}
                <span class="badge removed">-{diff.removed.length} removed</span>
            {/if}
            {#if diff.unchanged.length > 0}
                <span class="badge unchanged">{diff.unchanged.length} unchanged</span>
            {/if}
        </div>
        {#if hasDNValues}
            <button class="toggle-btn" onclick={() => showFullValues = !showFullValues}>
                {showFullValues ? 'Show names' : 'Show full DNs'}
            </button>
        {/if}
    </div>

    {#if totalCount === 0}
        <div class="empty-state">(empty)</div>
    {:else}
        <div class="diff-content">
            {#if diff.added.length > 0}
                <div class="diff-section">
                    <div class="section-label added">Added</div>
                    <ul class="value-list">
                        {#each diff.added as item}
                            <li class="value-item added" title={item}>
                                <span class="status-icon">+</span>
                                <span class="value-text">{formatItem(item)}</span>
                            </li>
                        {/each}
                    </ul>
                </div>
            {/if}

            {#if diff.removed.length > 0}
                <div class="diff-section">
                    <div class="section-label removed">Removed</div>
                    <ul class="value-list">
                        {#each diff.removed as item}
                            <li class="value-item removed" title={item}>
                                <span class="status-icon">−</span>
                                <span class="value-text">{formatItem(item)}</span>
                            </li>
                        {/each}
                    </ul>
                </div>
            {/if}

            {#if diff.unchanged.length > 0}
                <details class="diff-section unchanged-section">
                    <summary class="section-label unchanged">
                        {diff.unchanged.length} unchanged
                    </summary>
                    <ul class="value-list">
                        {#each diff.unchanged as item}
                            <li class="value-item unchanged" title={item}>
                                <span class="status-icon">&nbsp;</span>
                                <span class="value-text">{formatItem(item)}</span>
                            </li>
                        {/each}
                    </ul>
                </details>
            {/if}
        </div>
    {/if}
</div>

<style>
    .multi-value-diff {
        width: 100%;
        background: var(--bg-base);
        border-radius: 6px;
        overflow: hidden;
    }

    .diff-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.75rem 1rem;
        background: var(--bg-surface);
        border-bottom: 1px solid var(--border-subtle);
    }

    .diff-summary {
        display: flex;
        gap: 0.5rem;
        flex-wrap: wrap;
    }

    .badge {
        padding: 0.2rem 0.5rem;
        border-radius: 4px;
        font-size: 0.75rem;
        font-weight: 600;
        font-family: 'JetBrains Mono', monospace;
    }

    .badge.added {
        background: var(--diff-add-bg);
        color: var(--diff-add);
    }

    .badge.removed {
        background: var(--diff-remove-bg);
        color: var(--diff-remove);
    }

    .badge.unchanged {
        background: var(--bg-hover);
        color: var(--text-muted);
    }

    .toggle-btn {
        padding: 0.35rem 0.75rem;
        background: var(--bg-hover);
        border: none;
        border-radius: 4px;
        color: var(--text-secondary);
        font-size: 0.75rem;
        cursor: pointer;
    }

    .toggle-btn:hover {
        background: var(--border-medium);
        color: var(--text-primary);
    }

    .empty-state {
        padding: 1.5rem;
        text-align: center;
        color: var(--text-muted);
        font-size: 0.9rem;
    }

    .diff-content {
        padding: 0.5rem 0;
    }

    .diff-section {
        padding: 0.5rem 1rem;
    }

    .section-label {
        font-size: 0.7rem;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        margin-bottom: 0.5rem;
        padding: 0.25rem 0.5rem;
        border-radius: 4px;
        display: inline-block;
    }

    .section-label.added {
        background: var(--diff-add-bg);
        color: var(--diff-add);
    }

    .section-label.removed {
        background: var(--diff-remove-bg);
        color: var(--diff-remove);
    }

    .section-label.unchanged {
        background: var(--bg-hover);
        color: var(--text-muted);
        cursor: pointer;
    }

    .unchanged-section {
        border-top: 1px solid var(--border-subtle);
        margin-top: 0.5rem;
        padding-top: 0.75rem;
    }

    .unchanged-section summary {
        list-style: none;
        user-select: none;
    }

    .unchanged-section summary::-webkit-details-marker {
        display: none;
    }

    .unchanged-section summary::before {
        content: '▶ ';
        font-size: 0.6rem;
        margin-right: 0.25rem;
    }

    .unchanged-section[open] summary::before {
        content: '▼ ';
    }

    .value-list {
        list-style: none;
        margin: 0;
        padding: 0;
    }

    .value-item {
        display: flex;
        align-items: flex-start;
        gap: 0.5rem;
        padding: 0.35rem 0.5rem;
        border-radius: 4px;
        font-family: 'JetBrains Mono', monospace;
        font-size: 0.8rem;
        line-height: 1.4;
    }

    .value-item.added {
        background: var(--diff-add-bg);
    }

    .value-item.removed {
        background: var(--diff-remove-bg);
    }

    .value-item.unchanged {
        background: transparent;
    }

    .value-item.unchanged:hover {
        background: var(--bg-hover);
    }

    .status-icon {
        flex-shrink: 0;
        width: 1rem;
        text-align: center;
        font-weight: 700;
    }

    .value-item.added .status-icon {
        color: var(--diff-add);
    }

    .value-item.removed .status-icon {
        color: var(--diff-remove);
    }

    .value-item.unchanged .status-icon {
        color: var(--text-muted);
    }

    .value-text {
        flex: 1;
        word-break: break-all;
    }

    .value-item.added .value-text {
        color: var(--diff-add);
    }

    .value-item.removed .value-text {
        color: var(--diff-remove);
    }

    .value-item.unchanged .value-text {
        color: var(--text-secondary);
    }
</style>
