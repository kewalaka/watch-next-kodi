import { useState, useEffect, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getLists, getConfig } from './lib/api'
import { ListSwitcher } from './components/ListSwitcher'
import { WatchList } from './components/WatchList'
import { Loader2, Tv } from 'lucide-react'

function App() {
    const [activeGroup, setActiveGroup] = useState<string | null>(null);
    const [activeListName, setActiveListName] = useState<string | null>(null);

    const { data: lists, isLoading } = useQuery({
        queryKey: ['lists'],
        queryFn: getLists,
    });

    const { data: config } = useQuery({
        queryKey: ['config'],
        queryFn: getConfig,
    });

    // Extract unique groups
    const groups = useMemo(() => Array.from(new Set(lists?.map(l => l.group_name) || [])), [lists]);

    // Auto-select first group
    useEffect(() => {
        if (groups.length > 0 && activeGroup === null) {
            setActiveGroup(groups[0]);
        }
    }, [groups, activeGroup]);

    // Get lists for active group
    const currentGroupLists = useMemo(() => lists?.filter(l => l.group_name === activeGroup) || [], [lists, activeGroup]);

    // Auto-select first list type when group changes
    useEffect(() => {
        if (currentGroupLists.length > 0) {
            // If activeListName is not in current group, reset to first available
            const exists = currentGroupLists.find(l => l.list_name === activeListName);
            if (!exists) {
                setActiveListName(currentGroupLists[0].list_name);
            }
        }
    }, [currentGroupLists, activeListName]);

    // Find active list based on Group + ListName
    const activeList = lists?.find(l => l.group_name === activeGroup && l.list_name === activeListName);

    return (
        <div className="min-h-screen flex flex-col items-center p-4 md:p-8 bg-background text-text font-inter selection:bg-primary/30">
            <header className="w-full max-w-3xl mb-10 flex flex-col md:flex-row justify-between items-center gap-4">
                <div className="flex items-center gap-3">
                    <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-primary to-accent flex items-center justify-center text-white shadow-lg shadow-primary/20">
                        <Tv className="w-7 h-7" />
                    </div>
                    <div>
                        <h1 className="text-3xl font-bold tracking-tight">What's Next</h1>
                        <p className="text-textMuted text-sm italic">{config?.subtitle}</p>
                    </div>
                </div>

                {isLoading ? (
                    <Loader2 className="animate-spin text-textMuted" />
                ) : groups.length > 0 ? (
                    <ListSwitcher
                        groups={groups}
                        activeGroup={activeGroup}
                        onSelect={setActiveGroup}
                    />
                ) : null}
            </header>

            <main className="w-full max-w-3xl animate-fade-in flex-1">
                <div className="flex gap-6 mb-6 border-b border-white/10 overflow-x-auto">
                    {currentGroupLists.map((list) => (
                        <button
                            key={list.list_name}
                            onClick={() => setActiveListName(list.list_name)}
                            className={`pb-3 px-1 text-sm font-medium transition-colors border-b-2 whitespace-nowrap capitalize ${activeListName === list.list_name ? 'border-primary text-primary' : 'border-transparent text-textMuted hover:text-white'}`}
                        >
                            {list.list_name}
                        </button>
                    ))}
                </div>

                {activeList ? (
                    <WatchList 
                        key={activeList.id} 
                        listId={activeList.id} 
                        name={activeList.list_name} 
                        contentType={activeList.content_type || 'movie'} 
                    />
                ) : (
                    <div className="bg-surface rounded-xl border border-border p-12 text-center shadow-xl">
                        <p className="text-textMuted">
                            {isLoading ? "Loading lists..." : `No list found for ${activeGroup}.`}
                        </p>
                    </div>
                )}
            </main>

            <footer className="mt-20 py-6 text-textMuted text-xs w-full text-center border-t border-white/5">
                {config?.footer}
            </footer>
        </div>
    )
}

export default App
