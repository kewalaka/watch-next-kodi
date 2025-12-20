import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { searchMedia, addItem, getSeasons, MediaItem, Item } from '../lib/api';
import { Search, Loader2, Plus, X, Star, ChevronRight, ArrowLeft } from 'lucide-react';

interface AddItemModalProps {
    isOpen: boolean;
    onClose: () => void;
    listId: number;
    type: string; // 'movies' or 'tv'
}

export function AddItemModal({ isOpen, onClose, listId, type }: AddItemModalProps) {
    const [query, setQuery] = useState('');
    const [selectedShow, setSelectedShow] = useState<MediaItem | null>(null);
    const queryClient = useQueryClient();

    const { data: results, isLoading } = useQuery({
        queryKey: ['search', listId, type, query],
        queryFn: () => searchMedia(query, listId, type),
        enabled: query.length > 2 && !selectedShow,
    });

    const { data: seasons, isLoading: isLoadingSeasons } = useQuery({
        queryKey: ['seasons', selectedShow?.id],
        queryFn: () => getSeasons(selectedShow!.id, listId),
        enabled: !!selectedShow,
    });

    const addMutation = useMutation({
        mutationFn: (mediaItem: MediaItem) => {
            const isSeason = mediaItem.title.toLowerCase().includes('season');
            const itemPayload: Partial<Item> = {
                title: selectedShow ? selectedShow.title : (mediaItem.label || mediaItem.title),
                kodi_id: selectedShow ? selectedShow.id : mediaItem.id,
                media_type: isSeason ? 'season' : (type === 'tv' ? 'show' : 'movie'),
                year: (selectedShow?.year || mediaItem.year) || 0,
                poster_path: (selectedShow?.thumbnail || mediaItem.thumbnail) || '',
                season: isSeason ? mediaItem.season : 0,
                runtime: mediaItem.runtime || 0,
                episode_count: mediaItem.episode_count || 0,
                rating: (selectedShow?.rating || mediaItem.rating) || 0,
                sort_order: 0,
            };
            return addItem(listId, itemPayload);
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['items', listId] });
            onClose();
            handleClose();
        },
    });

    const handleClose = () => {
        setQuery('');
        setSelectedShow(null);
    };

    const formatRuntime = (seconds: number) => {
        if (!seconds) return '0m';
        const h = Math.floor(seconds / 3600);
        const m = Math.floor((seconds % 3600) / 60);
        return h > 0 ? `${h}h ${m}m` : `${m}m`;
    };

    const getImageURL = (path: string) => {
        if (!path) return '';
        if (path.startsWith('/api/posters/')) return path;
        // Kodi image proxy
        return decodeURIComponent(path.replace('image://', '').replace(/\/$/, ''));
    };

    if (!isOpen) return null;

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
            <div className="w-full max-w-2xl bg-[#1e1e24] border border-border rounded-xl shadow-2xl overflow-hidden flex flex-col max-h-[80vh]">
                <div className="p-4 border-b border-white/10 flex items-center gap-3">
                    {selectedShow ? (
                        <button onClick={() => setSelectedShow(null)} className="p-1 hover:bg-white/10 rounded">
                            <ArrowLeft className="w-5 h-5 text-textMuted" />
                        </button>
                    ) : (
                        <Search className="w-5 h-5 text-textMuted" />
                    )}

                    <input
                        autoFocus
                        type="text"
                        placeholder={selectedShow ? `Select Season for ${selectedShow.title}` : `Search ${type === 'movies' ? 'movies' : 'TV shows'}...`}
                        className="flex-1 bg-transparent outline-none text-lg placeholder-textMuted text-white"
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                        disabled={!!selectedShow}
                    />
                    <button onClick={() => { onClose(); handleClose(); }}><X className="w-5 h-5 text-textMuted hover:text-white" /></button>
                </div>

                <div className="flex-1 overflow-y-auto p-2 space-y-1">
                    {isLoading || isLoadingSeasons ? (
                        <div className="flex items-center justify-center py-12 text-textMuted">
                            <Loader2 className="w-6 h-6 animate-spin mr-2" /> {selectedShow ? 'Fetching Seasons...' : 'Searching...'}
                        </div>
                    ) : null}

                    {!selectedShow && results?.map((item) => (
                        <div key={item.id} className="flex items-center gap-4 p-3 hover:bg-white/5 rounded-lg group transition-colors cursor-pointer" onClick={() => type === 'tv' ? setSelectedShow(item) : addMutation.mutate(item)}>
                            <div className="w-12 h-16 bg-black/40 rounded flex-shrink-0 overflow-hidden">
                                {item.thumbnail && <img src={getImageURL(item.thumbnail)} className="w-full h-full object-cover" />}
                            </div>
                            <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-2">
                                    <h4 className="font-medium truncate text-white">{item.title}</h4>
                                    {item.rating && <span className="flex items-center gap-0.5 text-xs text-amber-400 font-bold">< Star className="w-3 h-3 fill-current" />{item.rating.toFixed(1)}</span>}
                                </div>
                                <p className="text-sm text-textMuted">
                                    {item.year} • {type === 'tv' ? `Series (${item.episode_count || '?'} Episodes)` : `Movie (${formatRuntime(item.runtime || 0)})`}
                                </p>
                            </div>
                            {type === 'tv' ? <ChevronRight className="w-5 h-5 text-textMuted" /> : <Plus className="w-5 h-5 text-textMuted" />}
                        </div>
                    ))}

                    {selectedShow && (
                        <div className="px-3 py-4 border-b border-white/5 bg-white/5">
                            <h2 className="text-xl font-bold text-white mb-1">{selectedShow.title}</h2>
                            <div className="flex items-center justify-between">
                                <h3 className="text-xs font-bold uppercase tracking-wider text-textMuted">Available Seasons</h3>
                                <span className="text-xs text-primary font-bold">{seasons?.length || 0} Seasons</span>
                            </div>
                        </div>
                    )}

                    {selectedShow && seasons?.map((season) => (
                        <div key={season.id} className="flex items-center justify-between p-4 hover:bg-white/5 rounded-lg group transition-colors">
                            <div>
                                <h4 className="font-medium text-white">{season.title || season.label}</h4>
                                <p className="text-sm text-textMuted">{season.episode_count} episodes</p>
                            </div>
                            <button
                                onClick={() => addMutation.mutate(season)}
                                className="p-2 rounded-full bg-white/10 hover:bg-primary text-white transition-all shadow-xl"
                            >
                                <Plus className="w-5 h-5" />
                            </button>
                        </div>
                    ))}
                </div>
            </div>
        </div>
    );
}
