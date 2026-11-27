import type { Metadata } from "next";
import { headers } from "next/headers";
import type { ReactNode } from "react";
import { CopyBlock } from "@/components/CopyBlock";
import { Card } from "@/components/ui";

export const metadata: Metadata = { title: "AI と接続" };

/**
 * Who a tool works for: public data is open to anyone, private data only to
 * its owner, and author tools need the author role.
 */
type Scope = "public" | "private" | "author";

const SCOPE_BADGES: Record<Scope, string> = {
  public: "badge badge-public",
  private: "badge badge-private",
  author: "badge badge-author",
};

const TOOLS: { name: string; does: string; scopes: Scope[] }[] = [
  { name: "list_problems", does: "公開されている問題、または自分の問題を一覧する", scopes: ["public"] },
  { name: "get_problem", does: "問題文・制約・サンプルを読む", scopes: ["public", "private"] },
  { name: "submit_code", does: "Python のコードを提出し、判定が終わるまで数秒待つ", scopes: ["public"] },
  { name: "get_submission", does: "提出の判定結果とケースごとの結果を見る", scopes: ["public", "private"] },
  { name: "create_problem", does: "問題を作る（最初は private）", scopes: ["author"] },
  { name: "update_problem", does: "問題文や制限を直す・公開する", scopes: ["private"] },
  { name: "list_test_cases", does: "テストケースを一覧する", scopes: ["private"] },
  { name: "add_test_case", does: "テストケースを追加する", scopes: ["private"] },
];

/** The AI tools covered below, in the order of their sections. */
const CLIENTS: { id: string; name: string; where: string }[] = [
  { id: "claude", name: "claude.ai・Claude Desktop", where: "設定のコネクタ" },
  { id: "claude-code", name: "Claude Code", where: "コマンド" },
  { id: "chatgpt", name: "ChatGPT", where: "設定のアプリ（開発者モード）" },
  { id: "codex", name: "Codex CLI", where: "コマンド" },
  { id: "cursor", name: "Cursor", where: "mcp.json" },
  { id: "vscode", name: "VS Code（GitHub Copilot）", where: "コマンドパレットまたは mcp.json" },
  { id: "gemini", name: "Gemini CLI", where: "コマンド" },
];

const TROUBLES: { symptom: string; fix: ReactNode }[] = [
  {
    symptom: "ログインの画面が開かない",
    fix: (
      <>
        ツールによっては、登録のあとに認証の操作が要ります。Codex CLI は <code>codex mcp login tag-judge</code>、Gemini CLI は{" "}
        <code>/mcp auth tag-judge</code> を実行してください。
      </>
    ),
  },
  { symptom: "ハンドル登録の画面が出た", fix: "TagJudge を初めて使うときは、ハンドルを決めてから許可の画面に進みます。" },
  { symptom: "問題を作れない", fix: "問題を作れるのは author 権限のアカウントだけです。閲覧と提出はどのアカウントでもできます。" },
  { symptom: "「接続できません」と表示された", fix: "許可の画面を開いたまま 10 分ほど経つと無効になります。AI ツールから接続し直してください。" },
  { symptom: "しばらくして使えなくなった", fix: "30 日間使わないと接続が切れます。AI ツールから接続し直してください。" },
  {
    symptom: "結果が「判定中」のまま返ってきた",
    fix: (
      <>
        AI が待つのは数秒までです。「提出 ID ○○ の結果を見て」と頼むと <code>get_submission</code> で確認します。
      </>
    ),
  },
];

/**
 * How a user connects an AI tool to TagJudge over MCP. Opening the MCP URL in
 * a browser also lands here (see next.config.ts). The URL is this site's
 * /mcp: the Next.js rewrite forwards it, and the OAuth pages, to the Go API.
 */
export default async function ConnectGuidePage() {
  const mcpUrl = `${await siteOrigin()}/mcp`;
  const cursorConfig = JSON.stringify({ mcpServers: { "tag-judge": { url: mcpUrl } } }, null, 2);
  const vscodeConfig = JSON.stringify({ servers: { "tag-judge": { type: "http", url: mcpUrl } } }, null, 2);

  return (
    <div className="page">
      <div className="page-head">
        <h1>AI と接続</h1>
        <p className="page-sub">
          TagJudge は MCP（Model Context Protocol）に対応した AI ツールから使えます。接続すると、会話の中で問題を探したり、コードを提出して結果を確かめたりできます。
        </p>
      </div>

      <ol className="guide-overview" aria-label="接続の流れ">
        <li>
          <span className="guide-num">1</span>
          <strong>URL をコピー</strong>
          <span>下の MCP サーバーの URL をコピーします。</span>
        </li>
        <li>
          <span className="guide-num">2</span>
          <strong>AI ツールに登録</strong>
          <span>使っているツールの手順に沿って、URL を MCP サーバーとして登録します。</span>
        </li>
        <li>
          <span className="guide-num">3</span>
          <strong>許可を押す</strong>
          <span>開いた TagJudge のページでログインし、許可します。</span>
        </li>
      </ol>

      <Card title="接続先の URL">
        <div className="guide-box">
          <CopyBlock label="MCP サーバーの URL" text={mcpUrl} />
        </div>
        <p className="callout callout-info">
          <strong>ブラウザで開いてもツールは使えません。</strong>
          この URL は AI ツールに登録するためのものです。トークンやパスワードを貼り付ける必要はなく、ログインは OAuth で行われます。
        </p>
      </Card>

      <Card title="対応している AI ツール" note="リモートの MCP サーバー（Streamable HTTP）と OAuth に対応したツールなら、ここにないものでも接続できます。">
        <table className="table guide-table">
          <thead>
            <tr>
              <th scope="col">ツール</th>
              <th scope="col">登録する場所</th>
            </tr>
          </thead>
          <tbody>
            {CLIENTS.map((client) => (
              <tr key={client.id}>
                <td data-label="ツール">
                  <a href={`#${client.id}`}>{client.name}</a>
                </td>
                <td data-label="登録する場所">{client.where}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <p className="callout callout-note">
          画面の名前や手順は、各ツールの更新で変わることがあります。見つからないときは、ツールのヘルプで「MCP」や「コネクタ」を探してください。
        </p>
      </Card>

      <div id="claude" className="guide-anchor">
        <Card title="claude.ai・Claude Desktop">
          <ol className="guide-steps">
            <li>
              Claude の<strong>設定</strong> → <strong>コネクタ</strong> → <strong>カスタムコネクタを追加</strong>を開きます。
            </li>
            <li>
              名前に <code>TagJudge</code>、URL に上の URL を入力して追加します。
            </li>
            <li>
              <strong>接続</strong>を押すと TagJudge のページが開くので、ログインして<strong>許可</strong>を押します。
            </li>
            <li>新しい会話で、ツールの一覧から TagJudge を有効にします。</li>
          </ol>
          <p className="callout callout-note">
            カスタムコネクタを使えるかどうかは、Claude のプランや組織の設定によって異なります。組織のアカウントでは、管理者による追加が必要な場合があります。
          </p>
        </Card>
      </div>

      <div id="claude-code" className="guide-anchor">
        <Card title="Claude Code">
          <ol className="guide-steps">
            <li>
              ターミナルで次のコマンドを実行します。どのディレクトリからでも使うには <code>--scope user</code> を付けます。
              <div className="guide-box">
                <CopyBlock label="コマンド" text={`claude mcp add --transport http tag-judge ${mcpUrl}`} />
              </div>
            </li>
            <li>
              Claude Code で <code>/mcp</code> と入力し、<code>tag-judge</code> を選んで <strong>Authenticate</strong> を押します。
            </li>
            <li>
              ブラウザで TagJudge のページが開くので、ログインして<strong>許可</strong>を押します。
            </li>
          </ol>
        </Card>
      </div>

      <div id="chatgpt" className="guide-anchor">
        <Card title="ChatGPT">
          <ol className="guide-steps">
            <li>
              <strong>設定</strong> → <strong>アプリ</strong> → <strong>詳細設定</strong>で<strong>開発者モード</strong>をオンにします。
            </li>
            <li>
              <strong>アプリを作成</strong>を押し、名前に <code>TagJudge</code>、MCP サーバーの URL に上の URL を入力します。認証は{" "}
              <strong>OAuth</strong> を選びます。
            </li>
            <li>
              作成すると TagJudge のページが開くので、ログインして<strong>許可</strong>を押します。
            </li>
            <li>会話の入力欄の「+」から TagJudge を有効にします。</li>
          </ol>
          <p className="callout callout-note">
            開発者モードを使えるかどうかは、ChatGPT のプランやワークスペースの設定によって異なります。ワークスペースでは、管理者が開発者モードを許可している必要があります。
          </p>
        </Card>
      </div>

      <div id="codex" className="guide-anchor">
        <Card title="Codex CLI">
          <ol className="guide-steps">
            <li>
              ターミナルで次のコマンドを実行して登録します。
              <div className="guide-box">
                <CopyBlock label="コマンド" text={`codex mcp add tag-judge --url ${mcpUrl}`} />
              </div>
            </li>
            <li>
              続けてログインします。ブラウザで TagJudge のページが開くので、<strong>許可</strong>を押します。
              <div className="guide-box">
                <CopyBlock label="コマンド" text="codex mcp login tag-judge" />
              </div>
            </li>
          </ol>
        </Card>
      </div>

      <div id="cursor" className="guide-anchor">
        <Card title="Cursor">
          <ol className="guide-steps">
            <li>
              <strong>Cursor Settings</strong> → <strong>Tools &amp; MCP</strong> → <strong>New MCP Server</strong> を開くと{" "}
              <code>~/.cursor/mcp.json</code> が開きます。
            </li>
            <li>
              次の内容を書いて保存します。ほかのサーバーがすでにあれば、<code>mcpServers</code> の中に <code>tag-judge</code> を足します。
              <div className="guide-box">
                <CopyBlock label="mcp.json" text={cursorConfig} />
              </div>
            </li>
            <li>
              設定画面の <code>tag-judge</code> に出る <strong>Connect</strong>（または <strong>Needs login</strong>）を押し、開いたページで
              <strong>許可</strong>を押します。
            </li>
          </ol>
        </Card>
      </div>

      <div id="vscode" className="guide-anchor">
        <Card title="VS Code（GitHub Copilot）">
          <ol className="guide-steps">
            <li>
              コマンドパレットで <strong>MCP: Add Server</strong> を実行し、<strong>HTTP</strong> を選んで上の URL を入力します。名前は{" "}
              <code>tag-judge</code> にします。
            </li>
            <li>
              または、<code>.vscode/mcp.json</code> に次の内容を書きます。
              <div className="guide-box">
                <CopyBlock label="mcp.json" text={vscodeConfig} />
              </div>
            </li>
            <li>
              サーバーを起動すると認証を求められるので、開いた TagJudge のページで<strong>許可</strong>を押します。Copilot Chat
              をエージェントモードにすると使えます。
            </li>
          </ol>
        </Card>
      </div>

      <div id="gemini" className="guide-anchor">
        <Card title="Gemini CLI">
          <ol className="guide-steps">
            <li>
              ターミナルで次のコマンドを実行して登録します。
              <div className="guide-box">
                <CopyBlock label="コマンド" text={`gemini mcp add --transport http --scope user tag-judge ${mcpUrl}`} />
              </div>
            </li>
            <li>
              Gemini CLI を起動して <code>/mcp auth tag-judge</code> と入力します。ブラウザで TagJudge のページが開くので、
              <strong>許可</strong>を押します。
            </li>
          </ol>
        </Card>
      </div>

      <Card
        title="使えるツール"
        note={
          <>
            AI は頼まれた内容に合わせて、次のツールを自動で使い分けます。公開範囲は{" "}
            <span className={SCOPE_BADGES.public}>public</span> が誰でも、
            <span className={SCOPE_BADGES.private}>private</span> が作成者・本人だけ、
            <span className={SCOPE_BADGES.author}>author</span> が author 権限のアカウントだけです。
          </>
        }
      >
        <table className="table guide-table">
          <thead>
            <tr>
              <th scope="col">ツール</th>
              <th scope="col">できること</th>
              <th scope="col">公開範囲</th>
            </tr>
          </thead>
          <tbody>
            {TOOLS.map((tool) => (
              <tr key={tool.name}>
                <td data-label="ツール">
                  <code>{tool.name}</code>
                </td>
                <td data-label="できること">{tool.does}</td>
                <td data-label="公開範囲">
                  <span className="guide-scopes">
                    {tool.scopes.map((scope) => (
                      <span key={scope} className={SCOPE_BADGES[scope]}>
                        {scope}
                      </span>
                    ))}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <p className="callout callout-caution">
          <strong>AI はあなたのアカウントとして操作します。</strong>
          提出や問題の作成は、Web 画面で行ったときと同じ扱いです。心当たりのない許可の画面が出たら、<strong>拒否</strong>を押してください。
        </p>
      </Card>

      <Card title="うまくいかないとき">
        <table className="table guide-table">
          <thead>
            <tr>
              <th scope="col">症状</th>
              <th scope="col">対処</th>
            </tr>
          </thead>
          <tbody>
            {TROUBLES.map((trouble) => (
              <tr key={trouble.symptom}>
                <td data-label="症状" className="guide-symptom">
                  {trouble.symptom}
                </td>
                {/* One child, so the phone layout keeps code and text on one line. */}
                <td data-label="対処">
                  <span>{trouble.fix}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>

      <Card title="接続を解除する">
        <table className="table guide-table">
          <thead>
            <tr>
              <th scope="col">ツール</th>
              <th scope="col">方法</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td data-label="ツール">claude.ai・Claude Desktop</td>
              <td data-label="方法">設定のコネクタから TagJudge を削除します。</td>
            </tr>
            <tr>
              <td data-label="ツール">ChatGPT</td>
              <td data-label="方法">設定のアプリから TagJudge を削除します。</td>
            </tr>
            <tr>
              <td data-label="ツール">Cursor・VS Code</td>
              <td data-label="方法">
                <span>
                  <code>mcp.json</code> から <code>tag-judge</code> を消します。
                </span>
              </td>
            </tr>
            <tr>
              <td data-label="ツール">Claude Code・Codex CLI・Gemini CLI</td>
              <td data-label="方法">
                <span>
                  <code>claude mcp remove tag-judge</code>・<code>codex mcp remove tag-judge</code>・
                  <code>gemini mcp remove tag-judge</code> を実行します。
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </Card>
    </div>
  );
}

/** The origin the browser used, as the Next.js server sees it behind any proxy. */
async function siteOrigin(): Promise<string> {
  const requestHeaders = await headers();
  const host = requestHeaders.get("x-forwarded-host") ?? requestHeaders.get("host") ?? "localhost:3000";
  const local = host.startsWith("localhost") || host.startsWith("127.0.0.1");
  const proto = requestHeaders.get("x-forwarded-proto") ?? (local ? "http" : "https");
  return `${proto}://${host}`;
}
