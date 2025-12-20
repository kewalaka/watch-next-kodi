import { useState } from 'react';
import { useSortable } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { useQuery } from '@tanstack/react-query';
import { Item, getEpisodes } from '../lib/api';
import { GripVertical, Trash2, Tv, Film, Star, Clock, Loader2, ChevronDown, ChevronUp, RefreshCw } from 'lucide-react';

interface SortableItemProps {
    item: Item;
    onDelete: (id: number) => void;
}

export function SortableItem({ item, onDelete }: SortableItemProps) {
    const [isExpanded, setIsExpanded] = useState(false);
    const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: item.id });

    // Lazy load episode info for seasons
    const { data: episodes, isLoading: isLoadingMeta, refetch, isFetching } = useQuery({
        queryKey: ['episodes-meta', item.kodi_id, item.season],
        queryFn: () => getEpisodes(item.kodi_id, item.season, item.list_id),
        enabled: item.media_type === 'season',
        staleTime: 1000 * 60 * 60, // 1 hour
    });

    const style = {
        transform: CSS.Transform.toString(transform),
        transition,
        zIndex: isDragging ? 50 : 0,
        opacity: isDragging ? 0.3 : 1
    };

    const formatRuntime = (seconds: number) => {
        const h = Math.floor(seconds / 3600);
        const m = Math.floor((seconds % 3600) / 60);
        return h > 0 ? `${h}h ${m}m` : `${m}m`;
    };

    const totalRuntime = episodes?.reduce((acc, ep) => acc + (ep.runtime || 0), 0) || 0;
    const epCount = episodes?.length || item.episode_count || 0;

    const getImageURL = (path: string) => {
        if (!path) return '';
        if (path.startsWith('/api/posters/')) return path;
        return decodeURIComponent(path.replace('image://', '').replace(/\/$/, ''));
    };

    return (
        <div
            ref={setNodeRef}
            style={style}
            className={`group bg-surface rounded-xl border border-border mb-3 shadow-md transition-all hover:border-primary/30 ${isDragging ? 'shadow-2xl' : ''}`}
        >
            <div className="flex items-center gap-4 p-4">
                <button {...attributes} {...listeners} className="cursor-grab text-textMuted hover:text-white p-1">
                    <GripVertical className="w-5 h-5" />
                </button>

                <div className="w-16 h-24 bg-black/40 rounded-lg overflow-hidden flex-shrink-0 border border-white/5">
                    {item.poster_path && <img src={getImageURL(item.poster_path)} className="w-full h-full object-cover" />}
                </div>

                <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between mb-1">
                        <div className="flex items-center gap-2">
                            {item.media_type === 'movie' ? <Film className="w-3.5 h-3.5 text-accent" /> : <Tv className="w-3.5 h-3.5 text-primary" />}
                            <span className="text-[10px] uppercase tracking-wider font-bold text-textMuted">
                                {item.media_type === 'season' ? `Season ${item.season}` : item.media_type}
                            </span>
                        </div>
                        {item.rating > 0 && (
                            <div className="flex items-center gap-1 text-xs text-amber-400 font-bold bg-amber-400/10 px-2 py-0.5 rounded-full border border-amber-400/20">
                                <Star className="w-3 h-3 fill-current" />
                                {item.rating.toFixed(1)}
                            </div>
                        )}
                    </div>
                    <h3 className="text-lg font-semibold text-white truncate mb-1">{item.title}</h3>
                    <div className="flex items-center gap-4 text-sm text-textMuted">
                        <span>{item.year}</span>
                        {item.media_type === 'season' && (
                            <div className="flex items-center gap-1">
                                <button
                                    onClick={() => setIsExpanded(!isExpanded)}
                                    className="flex items-center gap-1.5 px-2 py-0.5 bg-primary/10 text-primary rounded text-xs font-semibold hover:bg-primary/20 transition-colors"
                                >
                                    {isLoadingMeta ? <Loader2 className="w-3 h-3 animate-spin" /> : `${epCount} Episodes`}
                                    {isExpanded ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                                </button>
                                <button
                                    onClick={(e) => { e.stopPropagation(); refetch(); }}
                                    className={`p-1 text-textMuted hover:text-white transition-all ${isFetching ? 'animate-spin' : ''}`}
                                    title="Refresh episode list"
                                >
                                    <RefreshCw className="w-3 h-3" />
                                </button>
                            </div>
                        )}
                        {!isExpanded && totalRuntime > 0 && (
                            <span className="flex items-center gap-1.5 text-xs">
                                <Clock className="w-3 h-3" /> {formatRuntime(totalRuntime)}
                            </span>
                        )}
                        {item.media_type === 'movie' && item.runtime > 0 && (
                            <span className="flex items-center gap-1.5 text-xs">
                                <Clock className="w-3 h-3" /> {formatRuntime(item.runtime)}
                            </span>
                        )}
                    </div>
                </div>

                <button onClick={() => onDelete(item.id)} className="p-2.5 rounded-lg text-textMuted hover:text-red-400 hover:bg-red-400/10 transition-colors opacity-100 md:opacity-0 md:group-hover:opacity-100">
                    <Trash2 className="w-5 h-5" />
                </button>
            </div>

            {isExpanded && episodes && episodes.length > 0 && (
                <div className="px-14 pb-4 animate-in slide-in-from-top-2">
                    <div className="space-y-2 border-l-2 border-white/5 pl-4 py-2">
                        <div className="flex items-center gap-2 mb-3 text-xs text-textMuted font-bold uppercase tracking-wider">
                            <Clock className="w-3 h-3" /> Total Runtime: {formatRuntime(totalRuntime)}
                        </div>
                        {episodes.map(ep => (
                            <div key={ep.id} className="flex justify-between items-center text-sm group/ep">
                                <span className="text-textMuted font-medium italic">E{ep.episode}: <span className="text-white not-italic">{ep.title}</span></span>
                                <span className="text-xs text-textMuted flex items-center gap-1"><Clock className="w-3 h-3" />{formatRuntime(ep.runtime || 0)}</span>
                            </div>
                        ))}
                    </div>
                </div>
            )}
        </div>
    );
}
