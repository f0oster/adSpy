<script lang="ts">
    import { fetchVersionChanges } from './api';
    import SecurityDescriptorDiff from './SecurityDescriptorDiff.svelte';
    import MultiValueDiff from './MultiValueDiff.svelte';
    import type { AttributeChange } from './types';
    import { formatValue, isSecurityDescriptor, getBase64Value, shouldShowAsMultiValued, getErrorMessage } from './utils';

    interface Props {
        objectId: string;
        usn: number;
    }

    let { objectId, usn }: Props = $props();

    let changes: AttributeChange[] = $state([]);
    let loading = $state(true);
    let error: string | null = $state(null);

    $effect(() => {
        if (objectId && usn) {
            loadChanges(objectId, usn);
        }
    });

    async function loadChanges(objId: string, usnVal: number) {
        loading = true;
        error = null;
        try {
            changes = await fetchVersionChanges(objId, usnVal);
        } catch (e) {
            error = getErrorMessage(e);
            changes = [];
        } finally {
            loading = false;
        }
    }
</script>

<div class="attribute-diff">
    {#if loading}
        <div class="loading">Loading changes...</div>
    {:else if error}
        <div class="error">{error}</div>
    {:else if changes.length === 0}
        <div class="empty">No attribute changes recorded</div>
    {:else}
        <table class="changes-table">
            <thead>
                <tr>
                    <th>Attribute</th>
                    <th>Old Value</th>
                    <th></th>
                    <th>New Value</th>
                </tr>
            </thead>
            <tbody>
                {#each changes as change}
                    <tr class="change-row">
                        <td class="attr-name">{change.attribute}</td>
                        {#if isSecurityDescriptor(change.attribute)}
                            <td colspan="3" class="special-cell">
                                <SecurityDescriptorDiff
                                    oldValue={getBase64Value(change.old_value)}
                                    newValue={getBase64Value(change.new_value)}
                                />
                            </td>
                        {:else if shouldShowAsMultiValued(change)}
                            <td colspan="3" class="special-cell">
                                <MultiValueDiff
                                    oldValue={change.old_value}
                                    newValue={change.new_value}
                                />
                            </td>
                        {:else}
                            <td class="old-value">
                                <pre>{formatValue(change.old_value)}</pre>
                            </td>
                            <td class="arrow">→</td>
                            <td class="new-value">
                                <pre>{formatValue(change.new_value)}</pre>
                            </td>
                        {/if}
                    </tr>
                {/each}
            </tbody>
        </table>
    {/if}
</div>

<style>
    .attribute-diff {
        width: 100%;
        padding: 1.5rem;
    }

    .loading, .error, .empty {
        padding: 2rem;
        text-align: center;
        color: var(--text-muted);
        font-size: 0.95rem;
    }

    .error {
        color: var(--diff-remove);
    }

    .changes-table {
        width: 100%;
        border-collapse: separate;
        border-spacing: 0;
        font-size: 0.9rem;
    }

    .changes-table th {
        text-align: left;
        padding: 0.875rem 1rem;
        background: var(--bg-surface);
        color: var(--text-muted);
        font-weight: 500;
        text-transform: uppercase;
        font-size: 0.7rem;
        letter-spacing: 0.08em;
    }

    .changes-table th:first-child {
        border-radius: 6px 0 0 0;
    }

    .changes-table th:last-child {
        border-radius: 0 6px 0 0;
    }

    .changes-table td {
        padding: 1rem;
        vertical-align: top;
        background: var(--bg-elevated);
    }

    .changes-table tbody tr {
        transition: background 0.15s ease;
    }

    .changes-table tbody tr:nth-child(even) td {
        background: var(--bg-surface);
    }

    .attr-name {
        font-family: 'JetBrains Mono', monospace;
        color: var(--accent-primary);
        white-space: nowrap;
        font-weight: 500;
        font-size: 0.85rem;
    }

    .old-value, .new-value {
        width: 40%;
    }

    .old-value pre {
        color: var(--diff-remove);
        margin: 0;
        white-space: pre-wrap;
        word-break: break-all;
        font-family: 'JetBrains Mono', monospace;
        font-size: 0.85rem;
        line-height: 1.5;
    }

    .new-value pre {
        color: var(--diff-add);
        margin: 0;
        white-space: pre-wrap;
        word-break: break-all;
        font-family: 'JetBrains Mono', monospace;
        font-size: 0.85rem;
        line-height: 1.5;
    }

    .arrow {
        color: var(--text-muted);
        text-align: center;
        width: 40px;
        font-size: 1rem;
    }

    .special-cell {
        background: var(--bg-base);
        padding: 0;
    }

    .change-row:hover td {
        background: var(--bg-hover);
    }
</style>
