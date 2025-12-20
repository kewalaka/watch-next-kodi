import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getLists, getConfig } from './lib/api'
import { ListSwitcher } from './components/ListSwitcher'
import { WatchList } from './components/WatchList'
import { Loader2, Tv } from 'lucide-react'

function App() {
    const [activeGroup, setActiveGroup] = useState<string | null>(null);
    const [activeType, setActiveType] = useState<string>('movies'); // 'movies' or 'tv'

    const { data: lists, isLoading } = useQuery({
        queryKey: ['lists'],
        queryFn: getLists,
    });

    const { data: config } = useQuery({
        queryKey: ['config'],
        queryFn: getConfig,
    });

    // Extract unique groups
    const groups = Array.from(new Set(lists?.map(l => l.group_name) || []));

    // Auto-select first group
    useEffect(() => {
        if (groups.length > 0 && activeGroup === null) {
            setActiveGroup(groups[0]);
        }
    }, [groups, activeGroup]);

    // Find active list based on Group + Type
    const activeList = lists?.find(l => l.group_name === activeGroup && l.type === activeType);

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
                <div className="flex gap-6 mb-6 border-b border-white/10">
                    <button
                        onClick={() => setActiveType('movies')}
                        className={`pb-3 px-1 text-sm font-medium transition-colors border-b-2 ${activeType === 'movies' ? 'border-primary text-primary' : 'border-transparent text-textMuted hover:text-white'}`}
                    >
                        Movies
                    </button>
                    <button
                        onClick={() => setActiveType('tv')}
                        className={`pb-3 px-1 text-sm font-medium transition-colors border-b-2 ${activeType === 'tv' ? 'border-primary text-primary' : 'border-transparent text-textMuted hover:text-white'}`}
                    >
                        TV Shows
                    </button>
                </div>

                {activeList ? (
                    <WatchList key={activeList.id} listId={activeList.id} type={activeList.type} />
                ) : (
                    <div className="bg-surface rounded-xl border border-border p-12 text-center shadow-xl">
                        <p className="text-textMuted">
                            {isLoading ? "Loading lists..." : `No list found for ${activeGroup} - ${activeType}.`}
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
