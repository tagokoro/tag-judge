import { ProblemForm } from "@/components/problem-form";

export default function NewProblemPage() {
  return (
    <div className="grid gap-5">
      <div>
        <h1 className="text-2xl font-bold tracking-normal">Create Problem</h1>
        <p className="mt-1 text-sm text-ink/60">問題文、制約、公開範囲を設定します。</p>
      </div>
      <div className="panel p-4">
        <ProblemForm />
      </div>
    </div>
  );
}
