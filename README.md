# tsundoku

あとで読む URL を、タグと未読・既読で管理する CLI。データはローカルに保存します。ページの取得やブラウザの起動はしません。

## インストール

Go 1.27.1 以降と C コンパイラが必要です。このリポジトリで実行します。

```sh
CGO_ENABLED=1 go install .
```

インストール先（通常 `~/go/bin`）を `PATH` に追加してください。

## 使い方

```sh
tsundoku add 'https://example.com/article' --tag blog --tag go
tsundoku list --unread --tag go
tsundoku show 1
```

`add` は ID を返します。同じ URL を再登録するとタグを追加し、既読状態は保持します。タグは小文字になります。

`list` は新しい順に20件表示します。`--limit` で件数、`--reverse` で順序を変えられます。`--read` は既読のみ、複数の `--tag` はすべてのタグを持つ URL に絞ります。

`show` は保存した URL とタグを表示し、既読にします。既読にせず確認するには `--frozen` を付けます。

| コマンド | 操作 |
|---|---|
| `tsundoku show unread` | 未読を古い順に10件表示し、既読にする。件数は `--limit` で変更 |
| `tsundoku show unread --frozen --json` | 未読を既読にせず JSON 配列で出力 |
| `tsundoku rm 1 2` | 指定した ID を削除 |
| `tsundoku tag list` | タグと登録件数を表示 |
| `tsundoku skill install` | エージェント向けスキルを `~/.agents/skills/tsundoku/SKILL.md` に書き出す |
| `tsundoku skill uninstall` | 上記スキルファイルを削除する |
| `tsundoku list --help` | コマンドのオプションを確認 |

登録上限は10,000件、`--limit` は1〜1,000件です。URL は HTTP / HTTPS に対応し、末尾スラッシュやクエリが異なるものは別に登録します。

## 設定

設定なしでも使えます。`tsundoku init` で設定ファイルとデータベースを作成し、保存先を表示します。既存の設定は上書きしません。

| 保存内容 | 既定の場所 | 保存先の変更 |
|---|---|---|
| 設定 | `~/.config/tsundoku/config.toml` | `--config PATH` |
| データ | `~/.local/share/tsundoku/tsundoku.db` | `--db PATH` |
| スキル | `~/.agents/skills/tsundoku/SKILL.md` | `skill install` で書き出す |

`XDG_CONFIG_HOME` / `XDG_DATA_HOME` が設定されていれば、それぞれ `~/.config` / `~/.local/share` の代わりに使います。

`config.toml` の例:

```toml
[list]
limit = 20
sort = "created"
reverse = false

[tags]
sort = "name"
reverse = false

[[filter]]
name = "go"
rule = "https://example.com/*"

[[filter]]
name = "news"
regex = '^https://news\.'
```

`list.sort` は `created` / `id`、`tags.sort` は `name` / `count`。コマンドのオプションが設定より優先されます。

`[[filter]]` は URL に一致したタグを登録時に付けます。`rule` は URL 全体に一致し、`*` は任意の文字列、`?` は任意の1文字です。正規表現を使う場合は `rule` の代わりに `regex` を指定します。

設定を既存の URL に適用するには:

```sh
tsundoku tag refresh --dry-run
tsundoku tag refresh
```

通常はタグの追加だけを行います。`--prune` を付けると、ルールに一致しないタグを手動で付けたものも含めて削除します。
