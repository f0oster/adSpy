<script lang="ts">
    import { fetchSDDiff } from './api';
    import { getErrorMessage } from './utils';
    import type { SDDiffResponse, SIDInfo, ACEState, ChangeCounts, ACEStatusType } from './types';

    interface Props {
        oldValue?: string;
        newValue?: string;
    }

    let { oldValue = '', newValue = '' }: Props = $props();

    let diff: SDDiffResponse | null = $state(null);
    let loading = $state(true);
    let error: string | null = $state(null);
    let activeTab: 'before' | 'after' = $state('after');

    $effect(() => {
        loadDiff(oldValue, newValue);
    });

    async function loadDiff(oldVal: string, newVal: string) {
        if (!oldVal && !newVal) {
            diff = null;
            loading = false;
            return;
        }

        loading = true;
        error = null;
        try {
            diff = await fetchSDDiff(oldVal, newVal);
        } catch (e) {
            error = getErrorMessage(e);
            diff = null;
        } finally {
            loading = false;
        }
    }

    function formatSID(sidInfo: SIDInfo | undefined): string {
        if (!sidInfo) return '(none)';
        return sidInfo.resolved_name || sidInfo.raw || '(unknown)';
    }

    function formatRights(maskFlags: string[] | undefined): string {
        if (!maskFlags || maskFlags.length === 0) return '-';
        return maskFlags.map(f => f.replace('RIGHT_', '').replace('DS_', '')).join(', ');
    }

    function formatAceType(typeName: string | undefined): string {
        if (!typeName) return '-';
        return typeName
            .replace(/^ACCESS_|_ACE$/g, '')
            .replace('SYSTEM_', 'SYS_')
            .replace('DENIED', 'DENY');
    }

    function formatControlFlags(flags: number | undefined): string {
        if (flags === undefined) return '(none)';
        const flagNames: [number, string][] = [
            [0x0004, 'PRESENT'],
            [0x0008, 'DEFAULTED'],
            [0x0100, 'TRUSTED'],
            [0x0400, 'AUTO_INHERIT_REQ'],
            [0x1000, 'AUTO_INHERITED'],
            [0x4000, 'PROTECTED'],
        ];
        const matched = flagNames
            .filter(([bit]) => (flags & bit) !== 0)
            .map(([, name]) => name);
        // TODO: Validate control flags are being parsed correctly
        const hex = '0x' + flags.toString(16).padStart(4, '0');
        return matched.length > 0 ? `${matched.join(', ')} [${hex}]` : `[${hex}]`;
    }

    function getStatusClass(status: ACEStatusType): string {
        return `status-${status}`;
    }

    function countChanges(aces: ACEState[] | undefined): ChangeCounts {
        if (!aces) return { added: 0, removed: 0, moved: 0, unchanged: 0 };
        return aces.reduce<ChangeCounts>((acc, a) => {
            acc[a.status]++;
            return acc;
        }, { added: 0, removed: 0, moved: 0, unchanged: 0 });
    }

    let oldCounts = $derived.by(() => {
        const aces = diff?.dacl_diff?.old_aces;
        return aces ? countChanges(aces) : null;
    });
    let newCounts = $derived.by(() => {
        const aces = diff?.dacl_diff?.new_aces;
        return aces ? countChanges(aces) : null;
    });
</script>

{#snippet aceTable(aces: ACEState[], view: 'before' | 'after')}
    <table class="ace-table">
        <thead>
            <tr>
                <th class="col-pos">#</th>
                <th class="col-status"></th>
                <th class="col-type">Type</th>
                <th class="col-principal">Principal</th>
                <th class="col-rights">Rights</th>
                <th class="col-guid">Object GUID</th>
            </tr>
        </thead>
        <tbody>
            {#each aces as aceState}
                <tr class={getStatusClass(aceState.status)}>
                    <td class="col-pos">{aceState.position}</td>
                    <td class="col-status">
                        {#if view === 'before'}
                            {#if aceState.status === 'removed'}
                                <span class="status-icon removed" title="Removed">−</span>
                            {:else if aceState.status === 'moved'}
                                <span class="status-icon moved" title="Moved to position {aceState.moved_to}">→{aceState.moved_to}</span>
                            {:else}
                                <span class="status-icon unchanged">&nbsp;</span>
                            {/if}
                        {:else}
                            {#if aceState.status === 'added'}
                                <span class="status-icon added" title="Added">+</span>
                            {:else if aceState.status === 'moved'}
                                <span class="status-icon moved" title="Moved from position {aceState.moved_from}">{aceState.moved_from}→</span>
                            {:else}
                                <span class="status-icon unchanged">&nbsp;</span>
                            {/if}
                        {/if}
                    </td>
                    <td class="col-type" title={aceState.ace?.type_name}>{formatAceType(aceState.ace?.type_name)}</td>
                    <td class="col-principal" title={aceState.ace?.sid?.raw}>{formatSID(aceState.ace?.sid)}</td>
                    <td class="col-rights" title={aceState.ace?.mask_flags?.join(', ')}>{formatRights(aceState.ace?.mask_flags)}</td>
                    <td class="col-guid" title={aceState.ace?.object_type_guid}>{aceState.ace?.object_type_guid || '-'}</td>
                </tr>
            {/each}
        </tbody>
    </table>
{/snippet}

<div class="sd-diff">
    {#if loading}
        <div class="status-msg">Loading...</div>
    {:else if error}
        <div class="status-msg error">{error}</div>
    {:else if !diff}
        <div class="status-msg">No security descriptor data</div>
    {:else if !diff.has_changes}
        <div class="status-msg">No changes</div>
    {:else}
        {#if diff.owner_changed || diff.group_changed || diff.control_flags_changed}
            <div class="meta-changes">
                {#if diff.owner_changed}
                    <span class="meta-item">
                        <span class="label">Owner:</span>
                        <span class="old">{formatSID(diff.old_owner)}</span>
                        <span class="arrow">→</span>
                        <span class="new">{formatSID(diff.new_owner)}</span>
                    </span>
                {/if}
                {#if diff.group_changed}
                    <span class="meta-item">
                        <span class="label">Group:</span>
                        <span class="old">{formatSID(diff.old_group)}</span>
                        <span class="arrow">→</span>
                        <span class="new">{formatSID(diff.new_group)}</span>
                    </span>
                {/if}
                {#if diff.control_flags_changed}
                    <span class="meta-item">
                        <span class="label">Flags:</span>
                        <span class="old">{formatControlFlags(diff.old_control_flags)}</span>
                        <span class="arrow">→</span>
                        <span class="new">{formatControlFlags(diff.new_control_flags)}</span>
                    </span>
                {/if}
            </div>
        {/if}

        {#if diff.dacl_diff}
            <div class="dacl-section">
                <div class="section-header">
                    <h4>DACL</h4>
                    <div class="tabs">
                        <button
                            class="tab"
                            class:active={activeTab === 'after'}
                            onclick={() => activeTab = 'after'}
                        >
                            After
                            {#if newCounts}
                                <span class="badge-group">
                                    {#if newCounts.added > 0}<span class="badge added">+{newCounts.added}</span>{/if}
                                    {#if newCounts.moved > 0}<span class="badge moved">↔{newCounts.moved}</span>{/if}
                                </span>
                            {/if}
                        </button>
                        <button
                            class="tab"
                            class:active={activeTab === 'before'}
                            onclick={() => activeTab = 'before'}
                        >
                            Before
                            {#if oldCounts}
                                <span class="badge-group">
                                    {#if oldCounts.removed > 0}<span class="badge removed">-{oldCounts.removed}</span>{/if}
                                    {#if oldCounts.moved > 0}<span class="badge moved">↔{oldCounts.moved}</span>{/if}
                                </span>
                            {/if}
                        </button>
                    </div>
                </div>

                <div class="ace-list">
                    {#if activeTab === 'before' && diff.dacl_diff.old_aces}
                        {@render aceTable(diff.dacl_diff.old_aces, 'before')}
                    {:else if activeTab === 'after' && diff.dacl_diff.new_aces}
                        {@render aceTable(diff.dacl_diff.new_aces, 'after')}
                    {:else}
                        <div class="empty">No ACEs</div>
                    {/if}
                </div>
            </div>
        {/if}
    {/if}
</div>

<style>
    .sd-diff {
        padding: 1.5rem;
        width: 100%;
    }

    .status-msg {
        color: var(--text-muted);
        padding: 2rem;
        text-align: center;
        font-size: 0.95rem;
    }

    .status-msg.error {
        color: var(--diff-remove);
    }

    /* Owner/Group meta changes */
    .meta-changes {
        display: flex;
        flex-wrap: wrap;
        gap: 2rem;
        padding: 1.25rem 1.5rem;
        background: var(--bg-surface);
        border-radius: 8px;
        margin-bottom: 1.5rem;
        font-family: 'JetBrains Mono', monospace;
        font-size: 0.875rem;
    }

    .meta-item {
        display: flex;
        align-items: center;
        gap: 0.75rem;
    }

    .meta-item .label {
        color: var(--text-muted);
        font-weight: 500;
    }

    .meta-item .old {
        color: var(--diff-remove);
    }

    .meta-item .new {
        color: var(--diff-add);
    }

    .meta-item .arrow {
        color: var(--text-muted);
    }

    /* DACL Section */
    .dacl-section {
        background: var(--bg-surface);
        border-radius: 8px;
        overflow: hidden;
        width: 100%;
    }

    .section-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 1rem 1.5rem;
        background: var(--bg-elevated);
    }

    .section-header h4 {
        margin: 0;
        font-size: 0.75rem;
        font-weight: 600;
        color: var(--text-secondary);
        text-transform: uppercase;
        letter-spacing: 0.08em;
    }

    /* Tabs */
    .tabs {
        display: flex;
        gap: 0.5rem;
    }

    .tab {
        display: flex;
        align-items: center;
        gap: 0.625rem;
        padding: 0.5rem 1rem;
        background: var(--bg-hover);
        border: none;
        border-radius: 6px;
        color: var(--text-secondary);
        font-size: 0.85rem;
        font-weight: 500;
        cursor: pointer;
        transition: all 0.15s ease;
    }

    .tab:hover {
        background: var(--border-medium);
        color: var(--text-primary);
    }

    .tab.active {
        background: var(--accent-primary-dim);
        color: var(--accent-primary);
    }

    .badge-group {
        display: flex;
        gap: 0.375rem;
    }

    .badge {
        padding: 0.15rem 0.45rem;
        border-radius: 4px;
        font-size: 0.7rem;
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

    .badge.moved {
        background: var(--diff-move-bg);
        color: var(--diff-move);
    }

    /* ACE Table */
    .ace-list {
        overflow-x: auto;
        width: 100%;
    }

    .ace-table {
        width: 100%;
        border-collapse: separate;
        border-spacing: 0;
        font-size: 0.875rem;
    }

    .ace-table th {
        padding: 0.875rem 1rem;
        text-align: left;
        font-weight: 500;
        color: var(--text-muted);
        font-size: 0.7rem;
        text-transform: uppercase;
        letter-spacing: 0.06em;
        background: var(--bg-base);
        white-space: nowrap;
        position: sticky;
        top: 0;
    }

    .ace-table td {
        padding: 0.875rem 1rem;
        color: var(--text-primary);
        vertical-align: middle;
    }

    /* Zebra striping */
    .ace-table tbody tr:nth-child(odd) {
        background: var(--bg-elevated);
    }

    .ace-table tbody tr:nth-child(even) {
        background: var(--bg-surface);
    }

    /* Column styles - using td. prefix for higher specificity */
    td.col-pos {
        width: 50px;
        color: var(--text-muted);
        text-align: right;
        padding-right: 1.25rem;
        font-family: 'JetBrains Mono', monospace;
        font-size: 0.8rem;
    }

    td.col-status {
        width: 70px;
        text-align: center;
    }

    td.col-type {
        width: 160px;
        white-space: nowrap;
        color: var(--text-secondary);
        font-size: 0.8rem;
    }

    td.col-principal {
        min-width: 220px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-family: 'JetBrains Mono', monospace;
    }

    td.col-rights {
        min-width: 200px;
        color: var(--accent-primary);
        font-size: 0.8rem;
        font-weight: 500;
    }

    td.col-guid {
        color: var(--text-muted);
        font-size: 0.75rem;
        font-family: 'JetBrains Mono', monospace;
        opacity: 0.7;
    }

    /* Status icons */
    .status-icon {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-width: 28px;
        padding: 0.2rem 0.4rem;
        border-radius: 4px;
        font-weight: 600;
        font-size: 0.75rem;
        font-family: 'JetBrains Mono', monospace;
    }

    .status-icon.added {
        color: var(--diff-add);
        background: var(--diff-add-bg);
    }

    .status-icon.removed {
        color: var(--diff-remove);
        background: var(--diff-remove-bg);
    }

    .status-icon.moved {
        color: var(--diff-move);
        background: var(--diff-move-bg);
    }

    .status-icon.unchanged {
        color: transparent;
    }

    /* Row highlighting by status - using tbody for higher specificity */
    .ace-table tbody tr.status-added {
        background: var(--diff-add-bg);
    }

    .ace-table tbody tr.status-removed {
        background: var(--diff-remove-bg);
    }

    .ace-table tbody tr.status-moved {
        background: var(--diff-move-bg);
    }

    .ace-table tbody tr:not(.status-added):not(.status-removed):not(.status-moved):hover {
        background: var(--bg-hover);
    }

    .empty {
        padding: 3rem;
        text-align: center;
        color: var(--text-muted);
        font-size: 0.95rem;
    }
</style>
