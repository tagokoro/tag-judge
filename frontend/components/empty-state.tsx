export function EmptyState({ title }: { title: string }) {
  return (
    <div className="rounded-md border border-dashed border-line bg-white px-4 py-8 text-center text-sm text-ink/60">
      {title}
    </div>
  );
}
