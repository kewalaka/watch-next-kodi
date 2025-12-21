import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getItems, deleteItem, reorderItem, syncLibrary } from '../lib/api';
import { SortableContext, verticalListSortingStrategy, arrayMove } from '@dnd-kit/sortable';
import {
    DndContext,
    closestCenter,
    DragEndEvent,
    PointerSensor,
    TouchSensor,
    useSensor,
    useSensors,
} from '@dnd-kit/core';
import { SortableItem } from './SortableItem';
import { AddItemModal } from './AddItemModal';
import { Plus, Loader2, RefreshCw } from 'lucide-react';

interface WatchListProps {
    listId: number;
    name: string; // List name/type e.g. 'movies', 'tv', 'weekend'
    contentType: string; // 'movie' or 'tv'
}

export function WatchList({ listId, name, contentType }: WatchListProps) {
    const [isModalOpen, setIsModalOpen] = useState(false);
    const [isSyncing, setIsSyncing] = useState(false);
    const queryClient = useQueryClient();

    const sensors = useSensors(
        useSensor(PointerSensor, {
            activationConstraint: {
                distance: 8,
            },
        }),
        useSensor(TouchSensor, {
            activationConstraint: {
                delay: 250,
                tolerance: 5,
            },
        })
    );

    const { data: items, isLoading } = useQuery({
        queryKey: ['items', listId],
        queryFn: () => getItems(listId),
    });

    const deleteMutation = useMutation({
        mutationFn: deleteItem,
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['items', listId] }),
    });

    const reorderMutation = useMutation({
        mutationFn: ({ id, sortOrder }: { id: number; sortOrder: number }) => reorderItem(id, sortOrder),
    });

    const handleSync = async () => {
        setIsSyncing(true);
        try {
            await syncLibrary(listId, contentType === 'tv' ? 'tv' : 'movies');
            queryClient.invalidateQueries({ queryKey: ['items', listId] });
        } catch (e) {
            console.error('Sync failed:', e);
        } finally {
            setIsSyncing(false);
        }
    };

    const onDragEnd = (event: DragEndEvent) => {
        const { active, over } = event;
        if (over && active.id !== over.id) {
            const oldIndex = items!.findIndex((i) => i.id === active.id);
            const newIndex = items!.findIndex((i) => i.id === over.id);
            const newItems = arrayMove(items!, oldIndex, newIndex);

            // Update UI optimistically
            queryClient.setQueryData(['items', listId], newItems);

            // Persist changes
            newItems.forEach((item, index) => {
                reorderMutation.mutate({ id: item.id, sortOrder: index });
            });
        }
    };

    const safeItems = items || [];

    return (
        <div className="w-full">
            <div className="flex justify-between items-center mb-6">
                <h2 className="text-xl font-semibold flex items-center gap-2 capitalize">
                    {name} List
                    <span className="bg-primary/10 text-primary text-xs px-2 py-0.5 rounded-full font-bold">
                        {safeItems.length}
                    </span>
                </h2>
                <div className="flex gap-2">
                    <button
                        onClick={handleSync}
                        disabled={isSyncing}
                        className="flex items-center gap-2 px-4 py-2 bg-white/5 hover:bg-white/10 text-white rounded-lg font-medium transition-all text-sm border border-white/10 disabled:opacity-50"
                    >
                        <RefreshCw className={`w-4 h-4 ${isSyncing ? 'animate-spin' : ''}`} />
                        {isSyncing ? 'Syncing...' : 'Sync'}
                    </button>
                    <button
                        onClick={() => setIsModalOpen(true)}
                        className="flex items-center gap-2 px-4 py-2 bg-primary hover:bg-primary/90 text-white rounded-lg font-medium shadow-lg shadow-primary/20 transition text-sm"
                    >
                        <Plus className="w-4 h-4" />
                        Add {contentType === 'movie' ? 'Movie' : 'Show'}
                    </button>
                </div>
            </div>

            {isLoading ? (
                <div className="flex justify-center py-20">
                    <Loader2 className="animate-spin text-textMuted w-10 h-10" />
                </div>
            ) : safeItems.length === 0 ? (
                <div className="bg-surface rounded-xl border border-border p-12 text-center shadow-xl">
                    <p className="text-textMuted">Your list is empty. Time to add something!</p>
                </div>
            ) : (
                <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
                    <SortableContext items={safeItems.map(i => i.id)} strategy={verticalListSortingStrategy}>
                        {safeItems.map((item) => (
                            <SortableItem key={item.id} item={item} onDelete={(id) => deleteMutation.mutate(id)} />
                        ))}
                    </SortableContext>
                </DndContext>
            )}

            <AddItemModal
                isOpen={isModalOpen}
                onClose={() => setIsModalOpen(false)}
                listId={listId}
                contentType={contentType}
            />
        </div>
    );
}
