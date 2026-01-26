<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { fetchObjects, fetchObjectTypes } from './api';
    import type { ADObject } from './types';
    import { extractName, extractType, getErrorMessage } from './utils';

    interface Props {
        selectedId?: string;
        onselect?: (event: CustomEvent<ADObject>) => void;
    }

    let { selectedId, onselect }: Props = $props();

    let objects: ADObject[] = $state([]);
    let objectTypes: string[] = $state([]);
    let total = $state(0);
    let loading = $state(true);
    let error: string | null = $state(null);

    let search = $state('');
    let selectedType = $state('');
    let limit = $state(50);
    let offset = $state(0);

    let searchTimeout: ReturnType<typeof setTimeout> | undefined;

    onMount(async () => {
        try {
            objectTypes = await fetchObjectTypes();
        } catch (e) {
            console.error('Failed to load object types:', e);
        }
        await loadObjects();
    });

    onDestroy(() => {
        if (searchTimeout) {
            clearTimeout(searchTimeout);
        }
    });

    async function loadObjects() {
        loading = true;
        error = null;
        try {
            const result = await fetchObjects({ type: selectedType, search, limit, offset });
            objects = result.objects;
            total = result.total;
        } catch (e) {
            error = getErrorMessage(e);
            objects = [];
        } finally {
            loading = false;
        }
    }

    function handleSearchInput() {
        if (searchTimeout) {
            clearTimeout(searchTimeout);
        }
        searchTimeout = setTimeout(() => {
            offset = 0;
            loadObjects();
        }, 300);
    }

    function handleTypeChange() {
        offset = 0;
        loadObjects();
    }

    function selectObject(obj: ADObject) {
        onselect?.(new CustomEvent('select', { detail: obj }));
    }

    function nextPage() {
        if (offset + limit < total) {
            offset += limit;
            loadObjects();
        }
    }

    function prevPage() {
        if (offset > 0) {
            offset = Math.max(0, offset - limit);
            loadObjects();
        }
    }
</script>

<div class="object-list">
    <div class="filters">
        <input
            type="text"
            placeholder="Search..."
            bind:value={search}
            oninput={handleSearchInput}
            class="search-input"
        />
        <select bind:value={selectedType} onchange={handleTypeChange} class="type-select">
            <option value="">All</option>
            {#each objectTypes as type}
                <option value={type}>{type}</option>
            {/each}
        </select>
    </div>

    <div class="list-content">
        {#if loading}
            <div class="status">Loading...</div>
        {:else if error}
            <div class="status error">{error}</div>
        {:else if objects.length === 0}
            <div class="status">No objects found</div>
        {:else}
            <ul class="objects">
                {#each objects as obj}
                    <li>
                        <button
                            class="object-item"
                            class:selected={selectedId === obj.id}
                            onclick={() => selectObject(obj)}
                        >
                            <span class="type-badge" title={obj.type}>{extractType(obj.type)}</span>
                            <span class="object-name" title={obj.dn}>{extractName(obj.dn)}</span>
                        </button>
                    </li>
                {/each}
            </ul>
        {/if}
    </div>

    {#if !loading && objects.length > 0}
        <div class="pagination">
            <button onclick={prevPage} disabled={offset === 0}>←</button>
            <span class="page-info">{offset + 1}-{Math.min(offset + limit, total)} / {total}</span>
            <button onclick={nextPage} disabled={offset + limit >= total}>→</button>
        </div>
    {/if}
</div>

<style>
    .object-list {
        display: flex;
        flex-direction: column;
        height: 100%;
        overflow: hidden;
    }

    .filters {
        display: flex;
        gap: 0.75rem;
        padding: 1rem 1.25rem;
        background: var(--bg-elevated);
    }

    .search-input {
        flex: 1;
        padding: 0.6rem 0.875rem;
        border: 1px solid var(--border-subtle);
        border-radius: 6px;
        background: var(--bg-base);
        color: var(--text-primary);
        font-size: 0.875rem;
    }

    .search-input:focus {
        outline: none;
        border-color: var(--accent-primary);
        box-shadow: 0 0 0 3px var(--accent-primary-dim);
    }

    .search-input::placeholder {
        color: var(--text-muted);
    }

    .type-select {
        padding: 0.6rem 0.75rem;
        border: 1px solid var(--border-subtle);
        border-radius: 6px;
        background: var(--bg-base);
        color: var(--text-primary);
        font-size: 0.875rem;
        min-width: 90px;
    }

    .type-select:focus {
        outline: none;
        border-color: var(--accent-primary);
    }

    .list-content {
        flex: 1;
        overflow-y: auto;
        padding: 0.5rem 0;
    }

    .status {
        padding: 3rem 1.5rem;
        text-align: center;
        color: var(--text-muted);
        font-size: 0.9rem;
    }

    .status.error {
        color: var(--diff-remove);
    }

    .objects {
        list-style: none;
        margin: 0;
        padding: 0 0.75rem;
    }

    .objects li {
        margin-bottom: 0.25rem;
    }

    .object-item {
        display: flex;
        align-items: center;
        gap: 0.75rem;
        width: 100%;
        padding: 0.75rem 1rem;
        background: transparent;
        border: none;
        border-radius: 6px;
        cursor: pointer;
        text-align: left;
        color: var(--text-secondary);
        transition: all 0.15s ease;
    }

    .object-item:hover {
        background: var(--bg-hover);
        color: var(--text-primary);
    }

    .object-item.selected {
        background: var(--accent-primary-dim);
        color: var(--text-primary);
    }

    .type-badge {
        flex-shrink: 0;
        padding: 0.2rem 0.5rem;
        background: var(--bg-hover);
        color: var(--text-muted);
        border-radius: 4px;
        font-size: 0.65rem;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.03em;
    }

    .object-item.selected .type-badge {
        background: var(--accent-primary);
        color: white;
    }

    .object-name {
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-size: 0.875rem;
        font-family: 'JetBrains Mono', 'Consolas', monospace;
    }

    .pagination {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.75rem 1.25rem;
        background: var(--bg-elevated);
    }

    .pagination button {
        padding: 0.5rem 0.875rem;
        border: none;
        border-radius: 6px;
        background: var(--bg-hover);
        color: var(--text-secondary);
        font-size: 0.8rem;
    }

    .pagination button:hover:not(:disabled) {
        background: var(--border-medium);
        color: var(--text-primary);
    }

    .pagination button:disabled {
        opacity: 0.3;
        cursor: not-allowed;
    }

    .page-info {
        color: var(--text-muted);
        font-size: 0.8rem;
        font-family: 'JetBrains Mono', monospace;
    }
</style>
