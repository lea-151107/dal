# コーディング規約

このリポジトリ（pure Go の数式組版ライブラリ `dal`）のコードはすべて、この規約に従って書く。
dam（`github.com/lea/mermaid`）の規約を土台にしている。背景と全体設計は [plans/math-renderer.md](plans/math-renderer.md) を参照。

---

## 1. 制約（必ず守る）

| 項目 | ルール |
|---|---|
| 依存 | 標準ライブラリと `golang.org/x/image` だけを使う。x/image が内部で使う `golang.org/x/text`・`golang.org/x/sys/cpu`（いずれも pure Go）は間接依存として許容する。新しい依存を追加する場合は事前に合意を取る。`//go:build ignore` の生成プログラム（例 `internal/textgrid/gen_width.go`）に限り、x/text を直接 import してよい |
| cgo | 禁止。`CGO_ENABLED=0 go build ./...` が常に通ること |
| 外部プロセス・ネットワーク | `os/exec` と `net` 以下のパッケージは import しない |
| `unsafe` | 自分たちのコードでは import しない（標準ライブラリ内部での利用は対象外） |
| ファイル I/O | ライブラリ本体（`cmd/` 以外）では `os` を使ってファイルを読み書きしない。数式フォント（Latin Modern Math）は `internal/mathfont` で `embed` する。別のフォントは `Options.Font` で受け取る |
| グローバル状態 | 実行中に書き換えるパッケージ変数を持たない（読み取り専用の参照テーブルと、`regexp.MustCompile` で作る正規表現は可）。`init()` で副作用を起こさない（キャッシュなども呼び出しごとに作るか、`Options` 経由で渡す） |
| Go のバージョン | `go.mod` の `go` ディレクティブに書いたバージョンを最低要件とする |

これらは `deps_test.go` が自動で確認する（依存パッケージの一覧に禁止パッケージがないか）。

## 2. 整形と静的解析

- `gofmt -s` と `goimports` を通す。import は「標準ライブラリ」「外部モジュール」「このモジュール内」の 3 グループに分ける。
- `go vet ./...` で警告がないこと。
- `staticcheck` は推奨するが任意。`go.mod` には追加せず、`go run honnef.co/go/tools/cmd/staticcheck@<version>` のようにツールとして実行する。
- 1 行の長さに上限は設けないが、120 桁を超えそうなら改行を検討する。

## 3. パッケージ構成と依存の向き

```
dal (ルート: 公開 API、エラーの変換、構文木の型エイリアス)
  ├─ internal/parse      字句解析 → マクロ展開 → 構文解析（AST を作る）
  ├─ internal/box        TeX の組版（AST → ボックス → scene）
  │    ├─ internal/mathfont  埋め込みフォントと MATH テーブルのパーサ
  │    └─ internal/scene     描画命令（px）
  ├─ internal/raster     scene → *image.RGBA
  ├─ internal/textgrid   AST → 文字のグリッド（2 次元テキスト）
  └─ internal/linear     AST → 1 行テキスト
最下層: internal/ast（構文木）, internal/symbols（読み取り専用の表）, internal/diag（内部エラー）, internal/guard（上限と ctx）
```

- 公開 API はルートパッケージ `dal` だけに置く。それ以外はすべて `internal/` に置く。構文木は `internal/ast` に定義し、ルートで型エイリアスとして公開する。
- 依存は上から下への一方向だけにする。下位のパッケージが上位を import してはいけない（例: `raster` は `box` を知らない。`box`・`textgrid`・`linear` は互いに import しない）。
- 3 種類の出力（画像・2 次元テキスト・1 行）はすべて同じ AST から作る。出力ごとに構文を解釈し直さない。
- SVG・MathML などの出力を足すときは、`scene`（SVG）か AST（MathML）の上に新しいパッケージを作る。
- 依存の向きは `deps_test.go` の `TestLayering` が `layers` の表で確認する。パッケージを追加・変更したら、この図と表を同時に更新する。

## 4. 命名

- Go の慣習に従う: MixedCaps を使い、頭字語は大文字でそろえる（`ID`, `URL`, `RGBA`, `SVG`）。
- パッケージ名は短い小文字の単数形にする。`util`, `common`, `helpers`, `misc` は禁止。
- ファイル名は小文字にする。区切りが必要なら `_` を使う（Go の慣習どおり `_test.go`, `_amd64.go` と衝突しないように注意）。
- 単位を持つ値は、名前で単位を示す: `widthPx`, `fontSizePt`, `durationDays`。
- レシーバ名は型名の頭文字 1〜2 文字にし、同じ型の中で統一する。

## 5. エラー処理

- 番兵エラーは `ErrXxx` と命名し、ルートパッケージで公開する（`ErrUnsupported`, `ErrTooLarge`）。内部パッケージで新たに番兵を作る場合も、公開 API からは必ずルートの番兵に変換して返す。
- 構文エラーは必ず `*dal.SyntaxError` で返す。位置は **バイトオフセット**（`Offset`、0 始まり）と、そこから計算した `Line`・`Col`（1 始まり、`Col` はバイト単位）で示す。マクロ展開で生じたトークンの位置は、マクロを呼んだ位置とする。
- 未対応のコマンド・環境は `*dal.UnsupportedError`（`Command` と `Offset`）で返し、`errors.Is(err, ErrUnsupported)` が成り立つようにする。出力ごとに描けないもの（`RenderText` の `\cancel` など）も同じ型で返す。内部では `internal/diag` の型を使い、ルートの `convertErr` で変換する。
- ラップには `fmt.Errorf("...: %w", err)` を使い、呼び出し側が `errors.Is` / `errors.As` で判別できるようにする。
- エラーメッセージは小文字で始め、末尾に句点やピリオドを付けない。ルートが返すメッセージには `dal: ` を前置する。
- `context` のキャンセルは `ctx.Err()` をそのまま（ラップしてもよいが `%w` で）返す。
- ライブラリのコードでは `panic` しない。想定外の panic に備え、`Parse` / `Render` / `RenderText` / `RenderLine` の境界で `recover` してエラーに変換する（その際スタックはメッセージに含めない）。
- エラーを握りつぶさない。意図的に無視する場合は `_ =` で受けて、理由をコメントに書く。

## 6. 決定性（同じ入力から同じ画像を出す）

gvmd は「入力の sha256 + `Version`」をキャッシュのキーにするので、出力が揺れるとキャッシュが壊れる。

- map を反復した順序を出力に影響させない。順序が必要なら、キーをソートするか、挿入順を持つスライスと併用する。
- 結果の順序が出力に影響するソートは `sort.SliceStable` / `slices.SortStableFunc` を使い、比較関数には必ず最後のタイブレーク（例: 元の出現順）を入れる。要素が比較キーで完全に区別できる場合や、順序が結果に影響しない場合は `slices.SortFunc` でよい。
- `math/rand`、`time.Now`、環境変数、ロケール、goroutine の完了順に依存しない。並列化する場合も、結果は決まった順序で結合する。
- レイアウト座標は raster に渡す前に 1/64 px（`fixed.Int26_6` 相当）に丸める。
- arm64 や `GOAMD64=v3` では、コンパイラが `a*b + c` を FMA に融合し、結果の最下位ビットが変わる。1/64 px に丸めてもこれは吸収しきれない（丸めの境界をまたぐ値がある）。そのため次を守る。
  - 曲線を `vector.Rasterizer` の `QuadTo` / `CubeTo` に渡さない。`internal/curve` で整数のまま折れ線にし、`LineTo` だけを使う（vector の曲線の分割は float32 で、融合の影響を受ける）。
  - グリフも同じ理由で `QuadTo` / `CubeTo` を使わない。数式フォントの輪郭は `internal/raster` で `internal/curve` を使って折れ線にする。`TextFace` が返す Face の決定性は、その Face の実装による。
  - `internal/box` と `internal/raster` では、定数でない浮動小数点の積を `float64(a*b)` のように明示的に型変換して、融合を止める。Go の仕様では、積を変数に入れてから足す場合（`t := a*b; r := t + c`）も融合してよいので、足し算の直前だけでは足りない。2 のべき乗を掛ける積（`x/2`、`x/64`）は正確なので、融合されても結果は変わらない。
  - 丸めや判定に使う値に、アーキテクチャ別のアセンブリを持つ標準関数（`math.Hypot` など）を使わない。amd64 ではアセンブリ、arm64 では融合される純 Go 実装になり、下の確認（v3）では違いが見えない。`math.Sqrt` のように、どこでも正しく丸められる関数を使う。
  - 確認: `GOAMD64=v3 go test .` と `go test -tags noasm .` が、どちらも golden と一致すること（v3 は積和の融合の一部、noasm は vector の純 Go 実装を、amd64 の上で再現する）。v3 が融合するのは足し算だけで、arm64 は引き算（`a - x*y`、`x*y - a`）も融合するので、`GOARCH=arm64 go build -a -gcflags='github.com/lea/dal/...=-S' ./... 2>&1 | grep -E '\sF(N)?M(ADD|SUB)D\s'` に出る行が、2 のべき乗を掛ける積だけであることも確かめる。
- 出力に影響する変更を加えたら `Version` を上げる。golden の更新が必要になった変更は、出力に影響する変更とみなす。`TestMain`（`goldenversion_test.go`）が `testdata/golden.version` に記録した Version と golden 全体のハッシュを確認し、Version を上げずに golden が変わると `-update` でも失敗する。

## 7. リソース保護

- 入力サイズに比例して回るループ（組版するノード、配列のセル、グリフの組み立て、描画命令）では `guard.Tick()` で `ctx.Err()` を確認する（16 回に 1 回）。`Parse` と `RenderLine` は ctx を取らないが、入力長・ノード数・深さ・展開回数の上限で時間を抑える。
- 入力長・ノード数・入れ子の深さ・マクロ展開回数・画像のピクセル数は `Limits`（既定値は `DefaultMaxXxx`）で、展開後のトークン数・グリフの組み立ての部品数・文字サイズは名前付きの定数（`maxExpandedTokens`, `maxAssemblyParts`, `maxSizePx`）で上限を設ける。画像のピクセル数はメモリの上限も兼ねる（1 ピクセルあたり約 9 バイト）ので、埋め込む側が変えられるように `Limits` に置く。
- 1 回の処理の重さが要素ごとに大きく違うループ（raster の塗りつぶし）では、要素の数ではなく処理量（ピクセル数）に応じて `ctx` を確認する。`Tick` の 16 回に 1 回では、要素が少なく 1 つずつが重い入力を止められない。最後にもう一度 `ctx` を確認し、キャンセル後に結果を返さない。
- 上限を超えたら `ErrTooLarge` を返す。上限で処理を黙って打ち切り、不完全な画像を返すことはしない。
- 再帰の深さはパーサで `MaxDepth` に制限する。組版・テキスト出力の再帰は AST の深さを超えないので、パーサの制限がそのまま効く。
- `MaxWidth` / `MaxCols` に収まらないときは縮小や折り返しをせず `ErrTooLarge` を返す（読めない画像を返さない）。

## 8. 数値と単位

- 組版（`box`）は `float64` で計算し、単位は **数式フォントのフォント単位**（本文サイズで 1 em = `UnitsPerEm`）、y は上向きとする。スクリプトのサイズは掛け率（`scale`）で表す。
- px への変換は `box.ToScene` で一度だけ行う（`SizePx / UnitsPerEm`）。`scene` 以降は px、y は下向き。
- 組版のパラメータは MATH テーブルの `MathConstants` から取る。フォントにない値（`\nulldelimiterspace` など）は em 単位の名前付き定数にして `box/box.go` の冒頭にまとめる。
- それ以外でも、組版の結果を左右する値は名前付き定数にする。1 か所でしか使わない幾何の微調整はその場のリテラルでよいが、同じ値が 2 か所以上に出てくるなら名前を付ける。
- 2 次元テキストの単位は桁と行（整数）。全角（East Asian Wide / Fullwidth）は 2 桁とする。
- 浮動小数点の比較に `==` を使わない。許容誤差の定数を使う。

## 9. コメントとドキュメント

- 公開識別子には godoc コメントを必ず付ける（英語）。識別子名で始める: `// Render draws ...`。
- 内部のコメントは日本語でも英語でもよい。**何をするかではなく、なぜそうするか** を書く。
- KaTeX / TeX と挙動を意図的に変えている箇所には `// NOTE(katex-compat):` を付けて理由を書く。
- 未対応の構文には `// TODO(<パッケージや機能>):` を付ける。TODO を書く代わりにその構文を黙って無視することはしない（`ErrUnsupported` か `SyntaxError` を返す）。

## 10. テスト

- テーブル駆動にし、`t.Run(name, ...)` でサブテストに名前を付ける。
- 入力の数式は `testdata/<名前>.tex` に置き、golden は `testdata/golden/<同名>.png`（画像）、`.txt`（2 次元テキスト。`> ` の行がベースライン）、`_line.txt`（1 行テキスト。Unicode と ASCII の 2 行）に置く。名前が `inline_` で始まるものはインライン、それ以外はディスプレイで組む。`go test . -update` で golden を再生成する。
- golden を更新したら、差分を目視で確認してからコミットする。
- 新しい構文に対応するときは、最低限 **golden ケース 1 つ** と **構文エラーか未対応のケース 1 つ**（オフセットの確認を含む）を追加する。
- `FuzzParse` / `FuzzRender` / `FuzzRenderText` / `FuzzRenderLine` / `FuzzMathTable`（壊したフォント）で、panic しないこと・上限内の時間で返ることを確認する。Fuzz からは `recover` のない非公開関数を呼ぶ。
- 決定性テスト（同じ入力を並行して描画し、ピクセルと文字列が一致すること）と幅のテスト（`Dx() <= MaxWidth`、各行の桁数 `<= MaxCols`）は全 golden ケースに適用する。
- テストでは、システムにインストールされたフォントに依存しない。使うフォントは次の 3 つだけにする。
  - 埋め込みの Latin Modern Math
  - x/image の Go フォント
  - `testdata/fonts/` の日本語のサブセット（dam と同じファイル。`TestCopiesFromDam` で一致を確かめる）
  - 名前が `_ja` で終わる golden ケースは、この日本語フォントを TextFace に使う。
- `TestKaTeXCoverage` は、KaTeX の対応表の入力（`testdata/katex_coverage.txt`）を 3 種類の出力に通し、結果の表を `testdata/katex_coverage.golden` と照らし合わせる。
  - 対応を増やしたときも減らしたときも、表の差分に出る。
  - 表は描画結果ではないので、Version の検査の対象外。
- `TestSymbolTablesRender` は、記号表の全エントリを画像に描く。数式フォントに字形が無い記号は、`ImageSubstitutes` に代わりの文字を足すか、テストの `needsTextFace` に載せる。
- 新しい処理は、入力の大きさに対して線形に近い時間で終わるようにする。上限内の入力で二乗になる処理（折り返しで行頭から組み直すなど）は避ける。
- レビューで見つかった不具合には、`regress_test.go` に回帰テストを追加する。

## 11. コミット

- 1 コミットに 1 つの論理的な変更を入れる。リファクタリングと機能追加は分ける。
- メッセージは `<パッケージ>: <要約>` の形式で書く（例: `box: support \cancel`, `textgrid: fix tall braces`）。要約は英語で、命令形・小文字で始める。
- コミット前に `gofmt`, `go vet ./...`, `go test ./...` が通ること。
