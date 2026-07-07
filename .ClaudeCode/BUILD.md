# ビルド・開発・リリース手順

開発者向けの手順（セットアップ・ビルド・配布・リリース）をまとめる。README.mdはエンドユーザー向けの
内容に絞っているため、これらはここに集約している。

## セットアップ

```bat
go mod tidy
```

## 開発

```bat
wails dev
```

## ビルド

```bat
wails build
```

生成された `build\bin\link-launcher.exe` を実行する。

### 配布時の注意（HTMLは同梱されない）

本アプリはフロントエンド（`frontend/link-launcher.html`）をexeにembedせず、実行時に
`exe と同じディレクトリの frontend\link-launcher.html` をディスクから読み込む設計になっている
（理由は [`DESIGN.md`](./DESIGN.md) 参照）。そのため配布時は exe 単体ではなく、
`frontend\link-launcher.html` を同梱する必要がある。

- **`wails build --nsis` でインストーラを作る場合**: `build/windows/installer/project.nsi` で
  `frontend\link-launcher.html` をインストール先へ配置するようにしてあるため、追加作業は不要。
- **exe単体（ポータブル版）を配布する場合**: `build\bin\link-launcher.exe` と同じ階層に
  `frontend\link-launcher.html` を手動でコピーしてから配布する。

## リリース

`v*.*.*` 形式のタグ（例: `v1.0.0`）をpushすると、GitHub Actions
（`.github/workflows/release.yml`）が自動的に以下を行いGitHub Releaseを作成する。

- `wails build --nsis` でexe本体とNSISインストーラをビルド
- `frontend/link-launcher.html` を同梱したポータブル版ZIP（`link-launcher-<tag>-windows-portable.zip`）を作成
- インストーラ（`link-launcher-<tag>-windows-installer.exe`）とあわせてReleaseの資産としてアップロード

```bash
git tag v1.0.0
git push origin v1.0.0
```

補足: このリポジトリで作業するAIエージェント（Claude Code等）が使っているセッション環境では、
ブランチへのpushは通常どおり行えるが、**タグのpushだけがgit連携プロキシに403で拒否される**ことがある
（意図的な制限と思われる）。その場合はユーザー自身の環境から上記のコマンドを実行してタグをpushするか、
GitHubのWeb UIの Releases → Draft a new release からタグを作成・公開する。
