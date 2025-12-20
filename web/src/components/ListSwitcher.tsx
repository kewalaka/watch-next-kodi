import { clsx } from 'clsx';

interface ListSwitcherProps {
    groups: string[];
    activeGroup: string | null;
    onSelect: (group: string) => void;
}

export function ListSwitcher({ groups, activeGroup, onSelect }: ListSwitcherProps) {
    return (
        <div className="flex space-x-2 bg-surface p-1 rounded-lg border border-border">
            {groups.map((group) => (
                <button
                    key={group}
                    onClick={() => onSelect(group)}
                    className={clsx(
                        "px-4 py-2 rounded-md text-sm font-medium transition-all duration-200",
                        activeGroup === group
                            ? "bg-primary text-white shadow-lg shadow-primary/20"
                            : "text-textMuted hover:text-white hover:bg-surfaceHover"
                    )}
                >
                    {group}
                </button>
            ))}
        </div>
    );
}
