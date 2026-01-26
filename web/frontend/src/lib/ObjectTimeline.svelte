<script lang="ts">
    import { fetchObjectTimeline } from './api';
    import type { ADObject, TimelineEntry } from './types';
    import type { Snippet } from 'svelte';
    import { formatDate, formatFullDate, getErrorMessage } from './utils';

    interface Props {
        object: ADObject | null;
        onexpand?: (event: CustomEvent<{ objectId: string; usn: number }>) => void;
        changes?: Snippet<[TimelineEntry]>;
    }

    let { object, onexpand, changes }: Props = $props();

    let timeline: TimelineEntry[] = $state([]);
    let loading = $state(false);
    let error: string | null = $state(null);
    let expandedUSN: number | null = $state(null);

    $effect(() => {
        if (object) {
            loadTimeline(object.id);
        }
    });

    async function loadTimeline(objectId: string) {
        loading = true;
        error = null;
        expandedUSN = null;
        try {
            timeline = await fetchObjectTimeline(objectId);
        } catch (e) {
            error = getErrorMessage(e);
            timeline = [];
        } finally {
            loading = false;
        }
    }

    function toggleVersion(usn: number) {
        if (expandedUSN === usn) {
            expandedUSN = null;
        } else {
            expandedUSN = usn;
            if (object) {
                onexpand?.(new CustomEvent('expand', { detail: { objectId: object.id, usn } }));
            }
        }
    }
</script>

<div class="timeline-container">
    {#if loading}
        <div class="status">Loading timeline...</div>
    {:else if error}
        <div class="status error">{error}</div>
    {:else if timeline.length === 0}
        <div class="status">No change history found</div>
    {:else}
        <div class="timeline">
            {#each timeline as entry, index}
                <div class="version" class:expanded={expandedUSN === entry.usn_changed}>
                    <button class="version-header" onclick={() => toggleVersion(entry.usn_changed)}>
                        <div class="version-meta">
                            <span class="version-number">v{timeline.length - index}</span>
                            <span class="usn">USN {entry.usn_changed}</span>
                        </div>
                        <div class="version-info">
                            <span class="timestamp" title={formatFullDate(entry.timestamp)}>
                                {formatDate(entry.timestamp)}
                            </span>
                            {#if entry.modified_by}
                                <span class="modified-by">{entry.modified_by}</span>
                            {/if}
                        </div>
                        <span class="expand-icon">{expandedUSN === entry.usn_changed ? '▼' : '▶'}</span>
                    </button>

                    {#if expandedUSN === entry.usn_changed}
                        <div class="version-details">
                            {#if changes}
                                {@render changes(entry)}
                            {/if}
                        </div>
                    {/if}
                </div>
            {/each}
        </div>
    {/if}
</div>

<style>
    .timeline-container {
        display: flex;
        flex-direction: column;
        height: 100%;
        overflow: hidden;
    }

    .status {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
        color: var(--text-muted);
        font-size: 1rem;
    }

    .status.error {
        color: var(--diff-remove);
    }

    .timeline {
        flex: 1;
        overflow-y: auto;
        padding: 0;
    }

    .version {
        margin-bottom: 1rem;
        border-radius: 8px;
        background: var(--bg-surface);
        overflow: hidden;
        width: 100%;
        opacity: 0.7;
        transition: all 0.2s ease;
    }

    .version:hover {
        opacity: 0.9;
    }

    .version.expanded {
        opacity: 1;
        box-shadow: var(--shadow-soft);
        background: var(--bg-elevated);
    }

    .version-header {
        display: flex;
        align-items: center;
        width: 100%;
        padding: 1rem 1.25rem;
        background: none;
        border: none;
        cursor: pointer;
        color: var(--text-secondary);
        transition: all 0.15s ease;
    }

    .version-header:hover {
        background: var(--bg-hover);
    }

    .version.expanded .version-header {
        color: var(--text-primary);
        background: transparent;
    }

    .version-meta {
        display: flex;
        align-items: center;
        gap: 1rem;
        min-width: 160px;
    }

    .version-number {
        font-weight: 600;
        color: var(--accent-primary);
        font-size: 0.95rem;
    }

    .usn {
        font-family: 'JetBrains Mono', monospace;
        font-size: 0.8rem;
        color: var(--text-muted);
        padding: 0.2rem 0.5rem;
        background: var(--bg-hover);
        border-radius: 4px;
    }

    .version-info {
        flex: 1;
        display: flex;
        align-items: center;
        gap: 1.25rem;
    }

    .timestamp {
        font-size: 0.875rem;
        color: var(--text-secondary);
    }

    .modified-by {
        font-size: 0.8rem;
        color: var(--text-muted);
        background: var(--bg-hover);
        padding: 0.25rem 0.625rem;
        border-radius: 4px;
        font-family: 'JetBrains Mono', monospace;
    }

    .expand-icon {
        color: var(--text-muted);
        font-size: 0.75rem;
        margin-left: 0.75rem;
        transition: transform 0.2s ease;
    }

    .version.expanded .expand-icon {
        color: var(--accent-primary);
        transform: rotate(0deg);
    }

    .version-details {
        padding: 0;
        background: var(--bg-base);
        max-height: 70vh;
        overflow: auto;
        width: 100%;
    }
</style>
