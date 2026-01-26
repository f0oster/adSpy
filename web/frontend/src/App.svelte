<script lang="ts">
    import ObjectList from './lib/ObjectList.svelte';
    import ObjectTimeline from './lib/ObjectTimeline.svelte';
    import AttributeDiff from './lib/AttributeDiff.svelte';
    import type { ADObject, TimelineEntry } from './lib/types';
    import { extractType } from './lib/utils';

    let selectedObject: ADObject | null = $state(null);
    let expandedVersion: { objectId: string; usn: number } | null = $state(null);
    let sidebarCollapsed = $state(false);

    function handleObjectSelect(event: CustomEvent<ADObject>) {
        selectedObject = event.detail;
        expandedVersion = null;
    }

    function handleVersionExpand(event: CustomEvent<{ objectId: string; usn: number }>) {
        expandedVersion = event.detail;
    }

    function toggleSidebar() {
        sidebarCollapsed = !sidebarCollapsed;
    }
</script>

<div class="app">
    <aside class="sidebar" class:collapsed={sidebarCollapsed}>
        <div class="sidebar-header">
            <h1 class="logo">adSpy</h1>
            <button class="collapse-btn" onclick={toggleSidebar} title={sidebarCollapsed ? 'Expand' : 'Collapse'}>
                {sidebarCollapsed ? '→' : '←'}
            </button>
        </div>
        {#if !sidebarCollapsed}
            <ObjectList onselect={handleObjectSelect} selectedId={selectedObject?.id} />
        {/if}
    </aside>

    <main class="main-content">
        {#if !selectedObject}
            <div class="welcome">
                <div class="welcome-content">
                    <h2>Active Directory Change Timeline</h2>
                    <p>Select an object from the sidebar to view its change history.</p>
                </div>
            </div>
        {:else}
            <header class="object-header">
                <div class="object-info">
                    <span class="object-type-badge" title={selectedObject.type}>{extractType(selectedObject.type)}</span>
                    <h2 class="object-dn">{selectedObject.dn}</h2>
                </div>
            </header>

            <div class="timeline-area">
                <ObjectTimeline
                    object={selectedObject}
                    onexpand={handleVersionExpand}
                >
                    {#snippet changes(entry: TimelineEntry)}
                        {#if expandedVersion && expandedVersion.usn === entry.usn_changed}
                            <AttributeDiff
                                objectId={expandedVersion.objectId}
                                usn={expandedVersion.usn}
                            />
                        {/if}
                    {/snippet}
                </ObjectTimeline>
            </div>
        {/if}
    </main>
</div>

<style>
    :global(*) {
        box-sizing: border-box;
    }

    .app {
        display: flex;
        height: 100vh;
        overflow: hidden;
        background: var(--bg-base);
    }

    /* Sidebar */
    .sidebar {
        width: 360px;
        min-width: 360px;
        background: var(--bg-surface);
        display: flex;
        flex-direction: column;
        transition: width 0.2s ease, min-width 0.2s ease;
        box-shadow: var(--shadow-soft);
        z-index: 10;
    }

    .sidebar.collapsed {
        width: 56px;
        min-width: 56px;
    }

    .sidebar-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 1rem 1.25rem;
        background: var(--bg-elevated);
    }

    .logo {
        margin: 0;
        font-size: 1.2rem;
        font-weight: 600;
        color: var(--accent-primary);
        letter-spacing: -0.02em;
    }

    .collapsed .logo {
        display: none;
    }

    .collapsed .sidebar-header {
        justify-content: center;
        padding: 1rem;
    }

    .collapse-btn {
        background: var(--bg-hover);
        border: none;
        color: var(--text-muted);
        font-size: 0.9rem;
        padding: 0.5rem 0.75rem;
        border-radius: 6px;
    }

    .collapse-btn:hover {
        background: var(--border-medium);
        color: var(--text-secondary);
    }

    /* Main Content */
    .main-content {
        flex: 1;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        background: var(--bg-base);
        padding: 1.5rem 2rem;
    }

    .welcome {
        flex: 1;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .welcome-content {
        text-align: center;
        color: var(--text-muted);
    }

    .welcome-content h2 {
        margin: 0 0 0.75rem;
        font-size: 1.75rem;
        font-weight: 500;
        color: var(--text-secondary);
    }

    .welcome-content p {
        margin: 0;
        font-size: 1rem;
    }

    /* Object Header */
    .object-header {
        padding: 1.25rem 0;
        margin-bottom: 1.5rem;
        border-bottom: 1px solid var(--border-subtle);
    }

    .object-info {
        display: flex;
        align-items: center;
        gap: 1rem;
    }

    .object-type-badge {
        padding: 0.35rem 0.75rem;
        background: var(--accent-primary-dim);
        color: var(--accent-primary);
        border-radius: 6px;
        font-size: 0.7rem;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.05em;
    }

    .object-dn {
        margin: 0;
        font-size: 1rem;
        font-weight: 500;
        font-family: 'JetBrains Mono', 'Consolas', monospace;
        color: var(--text-primary);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    /* Timeline Area */
    .timeline-area {
        flex: 1;
        overflow: hidden;
        display: flex;
        flex-direction: column;
        width: 100%;
    }
</style>
