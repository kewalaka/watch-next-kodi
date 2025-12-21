export interface List {
    id: number;
    group_name: string;
    list_name: string;
    content_type: string;
    kodi_host: string;
}

export interface Item {
    id: number;
    list_id: number;
    kodi_id: number;
    media_type: 'movie' | 'episode' | 'show' | 'season';
    title: string;
    year: number;
    poster_path: string;
    runtime: number;
    episode_count: number;
    season: number;
    rating: number;
    sort_order: number;
}

export interface MediaItem {
    id: number;
    label: string;
    title: string;
    year?: number;
    thumbnail?: string;
    plot?: string;
    runtime?: number;
    rating?: number;
    showtitle?: string;
    season?: number;
    episode?: number;
    episode_count?: number;
}

const API_BASE = '/api';

export async function getLists(): Promise<List[]> {
    const res = await fetch(`${API_BASE}/lists`);
    return res.json();
}

export async function getItems(listId: number): Promise<Item[]> {
    const res = await fetch(`${API_BASE}/lists/${listId}/items`);
    return res.json();
}

export async function addItem(listId: number, item: Partial<Item>): Promise<Item> {
    const res = await fetch(`${API_BASE}/lists/${listId}/items`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(item),
    });
    return res.json();
}

export async function deleteItem(itemId: number): Promise<void> {
    await fetch(`${API_BASE}/items/${itemId}`, { method: 'DELETE' });
}

export async function reorderItem(itemId: number, sortOrder: number): Promise<void> {
    await fetch(`${API_BASE}/items/${itemId}/reorder`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sort_order: sortOrder }),
    });
}

export async function searchMedia(query: string, listId: number, contentType: string): Promise<MediaItem[]> {
    const params = new URLSearchParams({ q: query, list_id: listId.toString(), content_type: contentType });
    const res = await fetch(`${API_BASE}/search?${params}`);
    return res.json();
}

export async function syncLibrary(listId: number, contentType: string): Promise<{ count: number }> {
    const params = new URLSearchParams({ list_id: listId.toString(), content_type: contentType });
    const res = await fetch(`${API_BASE}/sync?${params}`);
    if (!res.ok) {
        const text = await res.text();
        throw new Error(`Sync failed (status ${res.status}): ${text || 'Unknown error'}`);
    }
    return res.json();
}

export async function getSeasons(showId: number, listId: number): Promise<MediaItem[]> {
    const params = new URLSearchParams({ tvshowid: showId.toString(), list_id: listId.toString() });
    const res = await fetch(`${API_BASE}/tv/seasons?${params}`);
    return res.json();
}

export async function getEpisodes(showId: number, season: number, listId: number): Promise<MediaItem[]> {
    const params = new URLSearchParams({ tvshowid: showId.toString(), season: season.toString(), list_id: listId.toString() });
    const res = await fetch(`${API_BASE}/tv/episodes?${params}`);
    return res.json();
}

export interface Config {
    subtitle: string;
    footer: string;
}

export async function getConfig(): Promise<Config> {
    const res = await fetch(`${API_BASE}/config`);
    return res.json();
}
