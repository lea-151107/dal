# 数式組版モジュール（pure Go）実装計画

## Context

- 要件: `/home/lea/tmp/math-module-requirements.md`。gvmd が使っている go-latex では上付き・下付き、`\left`/`\right`、行列が描けないため、TeX と同じボックスとグルーの組版エンジンを自作する。結果は画像、2 次元テキスト、1 行テキストの 3 種類で出す。
- 置き場所は `/home/lea/dal`（今は空）。モジュールは `github.com/lea/dal`、パッケージは `dal`。
- 設計と規約は dam（`~/dam`）にそろえる。
  - 依存: 標準ライブラリと `golang.org/x/image v0.47.0` だけ。go 1.26.0。
  - cgo、`os/exec`、`net`、`unsafe` は使わない。`os` は `cmd/` でだけ使う。
  - 実行中に書き換えるグローバル変数を持たない。
  - 公開 API の境界で `recover` する。
  - 座標は 1/64 px に丸める。
- 範囲: **第 1 段階と第 2 段階の全部**（ユーザーが選択）。9 章の 4（フォントの差し替え）は安いので入れる。SVG・MathML・事前定義マクロ（9 章の 5〜7）は入れないが、層を分けて後から足せるようにしておく。
- フォント: **Latin Modern Math**（GUST Font License）。CTAN の `lm-math.zip` から `latinmodern-math.otf` と `GUST-FONT-LICENSE.txt` を取ってきて同梱する。ネットワークを使うのはこの取得の 1 回だけ（実装時に許可を求める）。

## 公開 API（ルートパッケージ `dal`）

要件 4 章の案どおりにする。追加・確定する点は次のとおり。

```go
func Parse(src string, opt ParseOptions) (*Expr, error)
func Render(ctx context.Context, src string, opt Options) (*Result, error)
func RenderText(ctx context.Context, src string, opt TextOptions) (*TextResult, error)
func RenderLine(src string, opt LineOptions) (string, error)
const Version = "0.1.0"

type ParseOptions struct{ Limits Limits }
type Options struct {
    Display bool; SizePx float64 /* 0→20 */; Foreground, Background color.Color
    PaddingPx, MaxWidth int
    TextFace func(sizePx float64) (font.Face, error)
    Font []byte // 0 長なら同梱の LM Math。MATH テーブルが無ければエラー
    Limits Limits
}
type Result struct{ Image *image.RGBA; Baseline int; Width, Height, Depth float64 }
type TextOptions struct{ Display bool; MaxCols int; Limits Limits }
type TextResult struct{ Lines []string; Baseline int }
type LineStyle int // LineUnicode（既定）/ LineASCII
type LineOptions struct{ Style LineStyle; Limits Limits }
type Limits struct{ MaxInputBytes /*16KiB*/, MaxNodes /*5000*/, MaxDepth /*64*/, MaxExpansions /*1000*/ int }

var ErrTooLarge, ErrUnsupported error
type UnsupportedError struct{ Command string; Offset int } // Is(ErrUnsupported) == true
type SyntaxError struct{ Offset, Line, Col int; Msg string }  // Line・Col は 1 始まり。Col はバイト単位
```

- 構文木は `internal/ast` に置き、ルートでは型エイリアス（`type Expr = ast.Expr`、`type Node = ast.Node` と各ノード型）として公開する。こうするとルートと internal の間で import が循環しない。
- 各ノードには `Offset` を持たせる。マクロは Parse の中で展開するので、構文木にはマクロの定義が残らない。

## ディレクトリ構成

```
dal.go options.go errors.go version.go ast.go(エイリアス)
internal/
  guard/      ctx の確認（16 回に 1 回）とノード数・深さ・展開回数の上限（dam の guard を移植）
  ast/        構文木（原子の種類 Ord/Op/Bin/Rel/Open/Close/Punct/Inner を含む）
  parse/      字句解析 → マクロ展開（\newcommand, \def）→ 構文解析。SyntaxError と UnsupportedError を作る
  symbols/    コマンドの表（記号 → Unicode と原子の種類、関数名、かっこ、アクセント、書体の対応表）。
              読み取り専用の表で、KaTeX の対応関数一覧を目標にする
  mathfont/   latinmodern-math.otf を embed し、ライセンスも同梱。ほかに次の 2 つ:
              ot.go: sfnt にない MATH テーブルの自作パーサ（境界チェックを付ける）
              font.go: sfnt.Font に MATH を足した呼び出しごとの Font 型
  box/        TeX の組版エンジン（AST → ボックス）。Box/HList/VList/Kern/Glue/Glyph/Rule、スタイル D/T/S/SS と cramped、
              付録 G の規則、間隔の表
  scene/      描画命令（Glyph{gid, x, y, size, color}、TextRun、Rect、Path）。単位は px の float64。将来 SVG を出すときもこの層から作る
  raster/     scene → *image.RGBA。グリフの輪郭は sfnt の LoadGlyph で取り、x/image/vector で描く。
              座標は 1/64 px に丸め、ピクセル数に上限を設ける
  textgrid/   2 次元テキストの組版（AST → 文字のグリッド）と、東アジアの文字幅の表（dam の gen_width.go 方式で生成）
  linear/     1 行テキスト（AST → string）。Unicode の上付き・下付き、数学用英数字の対応表
cmd/dalpng/   標準入力 → PNG（--display --size --fg --bg --font --textfont --text --cols --line --ascii --output）
docs/plans/math-renderer.md, docs/CODING_GUIDELINES.md, README.md, LICENSE-FONT (GUST)
testdata/<name>.tex, testdata/golden/<name>.png|.txt|_line.txt
```

依存の向きは一方通行にする。

- ルート → parse、box、textgrid、linear
- box → mathfont、scene
- ルート → raster → scene、mathfont
- ast と symbols はどこからでも使える最下層に置く

## 主要設計

1. **パイプライン**: 処理の流れは「ソース → トークン → 展開 → AST → (a)、(b)、(c)」。
   - (a) `box` で組む → `scene` → `raster` で画像にする
   - (b) `textgrid` で文字のグリッドに組む
   - (c) `linear` で 1 行にする

   3 種類とも同じ AST を使う。どの出力でも、未対応のコマンドは Parse の時点で `UnsupportedError` になる。
2. **フォント**: グローバルなキャッシュは持たない。
   - 呼び出しのたびに `sfnt.Parse(bytes)` する。sfnt の Parse はテーブルの目次を読むだけなので軽い。
   - MATH テーブルは、最初にヘッダとオフセットだけを検証する。Coverage の検索（形式 1・2、二分探索）は、必要になったときにバイト列から直接読む。
   - `sfnt.Buffer` は呼び出しごとに作るので、並行して呼んでも安全。
   - 読み取るテーブル:
     - `MathConstants`（56 個の値）
     - `MathItalicsCorrectionInfo`、`MathTopAccentAttachment`、`ExtendedShapeCoverage`、`MathKernInfo`
     - `MathVariants`（縦と横の Coverage、`MathGlyphConstruction`、`GlyphAssembly`、`minConnectorOverlap`）
   - 文字からグリフへの変換は sfnt の cmap を使う。
   - `ssty`（GSUB）は読まない。`// NOTE(katex-compat):` として、上付きは縮小で代用すると書いておく。
3. **組版（box）**: TeXbook 付録 G に従い、パラメータは MathConstants から取る。
   - 原子の間隔は 8×8 の表で決める（スクリプトスタイルでは thin 以外を落とす）。
   - 規則と担当する構文:
     - 分数は規則 15（`\binom` もここ）
     - 上付き・下付きは規則 17 と 18（MathKern を使ったカットインを含む）
     - 根号は規則 11（サイズ違い → 部品の組み立て。`\sqrt[n]` の位置は RadicalDegree* で決める）
     - アクセントは規則 12（TopAccentAttachment を使う。`\widehat` は横のサイズ違い）
     - 範囲付きの大型演算子は規則 13（Display のときは DisplayOperatorMinHeight 以上のサイズ違いを選ぶ）
     - `\left`/`\right`/`\middle` は規則 19（delimiterfactor 901 と shortfall 5pt）
     - `\big` 系は KaTeX と同じ倍率にする
     - 行列や array は、列を揃える指定を使い、列の間隔と行の間隔を決める
   - 書体は Unicode の数学用英数字（U+1D400 以降）に写す。立体の文字は ASCII のまま描く。
   - `\text` は、数式フォントにある文字は数式フォントで描く。数式フォントに無い文字の並びだけを、`TextFace` の face で幅を測って TextRun として描く。空白・結合文字・ZWJ は直前の並びに付ける。`TextFace` が nil で数式フォントに無い文字があれば、`ErrUnsupported` を返す。こうすれば利用者は次の手段に切り替えられる。
4. **描画（raster）**:
   - SizePx は 1 em の大きさ。座標は font units × SizePx / unitsPerEm で px にする。
   - 画像の大きさは ceil(W)+2·pad × ceil(H+D)+2·pad とし、Baseline は pad + round(H) にする。
   - 前景色は Foreground か `\color` の色、背景は Background か透過。
   - MaxWidth を超えたら縮小せずに `ErrTooLarge` を返す。ピクセル数の上限は 16M にする。
5. **2 次元テキスト（textgrid）**: ボックス `{W, Asc, Desc, cells}` を整数の桁で組む。
   - 分数は `─` の横線をベースラインの行に置く。
   - 根号は `√` と上線の `‾` で描く。
   - かっこは、高さ 1 なら普通の文字にし、2 以上なら `⎛⎜⎝` `⎡⎢⎣` `⎧⎨⎩` `│` で組む。
   - 上付き・下付きは、全部の文字を Unicode の上付き・下付き文字にできるときだけそれを使い、できなければ上下の行に置く。
   - アクセントは上の行に置き、結合文字は使わない。
   - ∑ は `∑` 1 文字にする。Display なら範囲を上下に、インラインなら上付き・下付きの位置に置く。
   - 各行の幅は MaxCols 以下にし、超えたら `ErrTooLarge` を返す。
   - 表せない構文は `ErrUnsupported` にする（`\cancel` など。`\color` は無視し、NOTE を付ける）。
6. **1 行テキスト（linear）**: 要件 6 章のとおりにする。
   - 上付き・下付きは、全部の文字を変換できるときだけ Unicode の上付き・下付き文字にする。
   - 分数と根号はかっこを補って線形化する（`(a+b)/(c+d)`、`√(x+1)`）。
   - 行列は `[[a, b]; [c, d]]` の形にする。
   - エラーにするのは未知のコマンドだけ。
7. **第 2 段階**:
   - `aligned`/`align`/`gather`/`array`（`lcr|` の指定）。`\tag` は式の右に quad を空けて置く。
   - `\overbrace`/`\underbrace` は MathVariants の横方向の組み立てで作る。
   - `\overset`/`\underset`/`\stackrel` に対応する。
   - `\xrightarrow` は横方向のサイズ違いを使い、足りなければ矢じりと Rule をつないで作る。
   - `\color`/`\textcolor` は名前と `#RRGGBB` の色を受け付ける。
   - `\newcommand`/`\def` は引数なしの形と `#1..#9` を受け付ける（区切り付きのパラメータは未対応にし、NOTE を付ける）。展開の回数と、展開後のトークン数に上限を設ける。
   - `\displaystyle` などのスタイル指定、`\phantom`、`\boxed`、`\cancel` に対応する。
8. **エラーとリソースの保護**:
   - 内部のエラーは境界の `convertErr` で公開用のエラーに変換する。
   - 入力長・ノード数・入れ子の深さ（構文解析と組版の両方の再帰で数える）・展開回数・ピクセル数に上限を設け、超えたら `ErrTooLarge` を返す。
   - Line と Col はオフセットから計算する。

## 実装順

0. 計画を `docs/plans/math-renderer.md` に保存する。dam の規約をもとに `docs/CODING_GUIDELINES.md` を書く（`NOTE(katex-compat)`、オフセット形式のエラー、`.tex` の golden）。`go.mod` を作る。
1. フォントを取得して同梱し、`mathfont`（MATH パーサとそのテスト。定数の値を既知の値と照合する）を作る。
2. `ast`、`symbols`、`parse`（第 1 段階の構文、エラーのオフセット）と `Parse` の API を作る。
3. `linear` と `RenderLine` を作る（早めに AST の形を検証するため）。
4. `box`、`scene`、`raster` と `Render`（第 1 段階）を作る。
5. `textgrid` と `RenderText`（第 1 段階）を作る。
6. 第 2 段階の構文を 3 種類の出力すべてに入れる。
7. テストをそろえる: golden、決定性、幅、エラー、Fuzz、依存、ctx。
8. `cmd/dalpng` と README を作り、性能を測る。

## 検証

- `CGO_ENABLED=0 go build ./...`、`go vet ./...`、`go test ./...` を通す。
- golden: `go test ./... -update` で作り直し、PNG は Read で目視して確認する。対象は `testdata/*.tex` から作る `.png`、`.txt`、`_line.txt`。
- `TestDeterministic`: 4 並列で何度も描き、ピクセルの sha256 と文字列が一致することを確かめる。
- `TestMaxWidth` と `TestMaxCols` を用意する。
- `TestSyntaxErrors`: Offset、Line、Col を確かめる。
- `TestUnsupported`: Command の値と `errors.Is` が成り立つことを確かめる。
- `TestLimits`、`TestContext` を用意する。
- `TestDependencies`: dam の deps_test.go を移植する。
- Fuzz: `FuzzParse`、`FuzzRender`、`FuzzRenderText`、`FuzzRenderLine`、`FuzzMathTable`（壊れたフォントのバイト列）。panic せず、2 秒以内に返ることを確かめる。
- ベンチマーク: `\sum_{i=1}^n \frac{1}{i^2}` で、Render が数 ms、RenderText が 1 ms 未満に収まることを確かめる。
- CLI: `echo '\frac{a}{b}' | go run ./cmd/dalpng > /tmp/x.png` を実行して確認する。

## 実装状況（2026-11-04）

第 1 段階と第 2 段階を実装した。計画からの変更点と、分かっている制限は次のとおり。

**計画からの変更点**
- 原子の種類の定数は、構文木の型 `Op` と名前がぶつかるため `ClassOrd` … `ClassInner` にした。
- `Group` に `Class` を持たせ、`\mathrel{}` などに対応した。
- 2 次元テキストのインライン分数は、TeX と同じく分子・分母の中に空白を入れない（`a+b`）。
- 2 次元テキストの上線は `─`、`\bar` / `\overline` は上の行の `_`、`\underline` は下の行の `‾` で描く。
- `\text` の日本語は、`TextFace` が無ければ `ErrUnsupported` にした。

**分かっている制限**（どれも `NOTE(katex-compat)` か `TODO` をコードに付けた）

> 2026-11-05 追記: 次の制限は `math-completion.md` のフェーズ 1 と 2（v0.3〜v0.4）で解消した。
> - `ssty`（スクリプト用の字形とプライム）
> - MathKern（MathKernInfo を持つフォントのとき）
> - `\tag` の右端揃え（幅が分かるとき）
> - 定義の有効範囲、区切り付きのパラメータと既定値
> - `\hphantom` と `\vphantom`
> - 2 次元テキストの `\cancel`
> - `\over` 系
>
> 2 次元テキストで色を捨てる点は、設計上そのまま残す。
- `ssty`（GSUB）は読まない。プライムは本文サイズのまま、底の直後に置く。
- MathKern（スクリプトの切り欠き）は使わない。
- `\newcommand` / `\def` の定義はグループを越えて有効になる。区切り付きのパラメータと、省略可能な引数は未対応。
- `\tag` は式の右に 1em 空けて置く（右端揃えではない）。
- `\hphantom` / `\vphantom` は `\phantom` と同じ扱い。
- 2 次元テキストの `\cancel` と `\color` は描けない（`\cancel` は `ErrUnsupported`、`\color` は無視する）。
- `\over` / `\choose` / `\atop` は未対応（`\text` の中の数式と array の `p{}` は 0.1.2 で対応）。

**性能**（`\sum_{i=1}^n \frac{1}{i^2}`、amd64）: Render は約 47µs、RenderText は約 3.8µs、RenderLine は約 2µs。

## 敵対的レビュー後の修正（2026-11-04、Version 0.1.1）

サブエージェントによる敵対的レビューの指摘のうち、反論しても残ったものを直した。回帰テストは `review_test.go` にある。

- **リソースの上限を組版の前で効かせる。** 次の 3 つは、どれも上限を超えると `ErrTooLarge` を返す。
  - マクロ展開は、組み立てる前に出力のトークン数を計算する。
  - array では、短い行を最大の列数まで埋めるセルと、`\hline` と `|` もノード数に数える。
  - 2 次元テキストのグリッドは、1 回の呼び出しで確保するセル数の合計を 4M に制限する。
- **悪意あるフォントへの対策**: 1 回の呼び出しで使う組み立て部品の合計を 20000 に制限し、MATH テーブルのサイズ違いと部品の数にも上限を設けた。
- **ラスタの仕事量**: ラスタライズで触れる面積の合計を、ピクセル数の上限の 4 倍に制限した。
- `\middle` に上付き・下付きを付けても panic しないようにした。
- 1 行と 2 次元テキストで、上付き・下付きや根号で終わる底に、さらに上付き・下付きを付けるときはかっこを補う（例: `(x²)³`、`(√x)²`）。
- `\not` の後ろの記号に否定形の文字が無い場合の扱い:
  - 画像では斜線を重ねる。
  - 1 行では U+0338 を付ける。
  - 2 次元テキストでは `ErrUnsupported` にする。
- C1 制御文字と双方向制御文字は `SyntaxError` にした。
- `Result.OriginX` を追加した。画像の左端から式の原点までの px。
- 2 次元テキストで、高さのある `\sqrt` の指数を根号の左上に置くようにした。

## 2 回目の敵対的レビュー後の修正（2026-11-04、Version 0.1.2）

回帰テストは `review2_test.go` にある。

- `\color{#RRGGBB}` / `\textcolor{#RGB}` を使えるようにした。lexer は単独の `#` を文字として通し、parser は色の引数以外では SyntaxError にする。
- **2 次元テキスト**:
  - かっこを補う対象を広げた（Font、Color、Style の本体と、Group の最後の要素）。
  - 区切り記号の無い分数や行列に上付き・下付きを付けるときは、高さに合わせて伸ばしたかっこで包む。
- **1 行テキスト**:
  - 分数のすぐ隣に別の項があるときは、かっこで包む（`(a/b)(c/d)`、`(1/2)x`）。符号の後ろや空白の隣は包まない。
  - 分子・分母が分数か空白を含むときも、かっこで包む（`a/(b/c)`）。
- **`\text` の中身**:
  - `$…$`、`\textbf` などの入れ子、テキスト用の記号（`\ldots` など）、アクセント（Latin 系の合成済み文字への変換表）に対応した。
  - `\tag` も同じ処理を通す。
- **array の列指定**: `p{…}` / `m{…}` / `b{…}` は左揃えの列にし、`@{…}` / `!{…}` は捨てる（NOTE(katex-compat)）。
- `\not` の後ろに記号が無ければ SyntaxError にした。
- **2 次元テキストの結合文字**: 合成できず黙って消えてしまうので、`ErrUnsupported` にした。
- **Options の不正な値**（SizePx が NaN・Inf・負、PaddingPx が負、MaxWidth が負）はエラーにした。PaddingPx が 1024 を超えるときは ErrTooLarge を返す。
- **`\operatorname`**: 中身が文字だけでなければ、数式として立体で組む（`arg\,max`、`f_1`）。中にある空白は、2 次元テキストと 1 行でも 1 桁として出す。
- **描画**: 同じ色の連続はマスクにまとめて 1 回で合成する。半透明の前景色でも、重なった部分が濃くならない。
- **画像の大きさ**: 寸法を 1/64 px に丸めてから、floor と ceil をかける。
- **文書化**: Ambiguous 幅の文字を 1 桁として扱うことを、TextOptions の doc と README に書いた。設計上、直さない。

## 利用者の指摘による修正（Version 0.1.3）

- **TextFace の振り分け**: これまでは TextFace を渡すと `\text` の中身がすべてその face で描かれ、`\text{otherwise}` も見た目が変わっていた。数式フォントにある文字は数式フォントで描き、無い文字の並びだけを TextFace で描くようにした。空白・結合文字・ZWJ は直前の並びに付ける。TextFace は、そうした並びがあるときだけ呼ばれる。
- **staticcheck ST1018**: テストとソースの文字列リテラルにある書式文字（U+202E、U+2066、U+200F、U+200D、U+00A0）を `\u` のエスケープで書いた。

## gvmd からの細かい指摘による修正（Version 0.1.4）

- **TextFace に無い結合文字**: 空白・結合文字・ZWJ を直前の TextFace の並びに付けるのは、TextFace にその字形があるときだけにした（`font.Face.GlyphAdvance` の ok で確かめる）。無ければ数式フォントで描くので、`\text{日本語 ̃é}` の U+0303 が豆腐にならない。
- **大きすぎる寸法**: `\kern` などの寸法の数値の絶対値が 1000 を超えるときは、SyntaxError ではなく ErrTooLarge を返す。数値として読めないときは、これまでどおり SyntaxError を返す。
- **`\left … \right` の中の `\\` と `&`**: `missing \right` ではなく、その位置で「`\\` cannot appear between \left and \right」と報告する。
- **1 行出力の `\cancel`**: 中身だけを出していたため取り消しの意味が消えていた。`cancel(x)`・`bcancel(…)`・`xcancel(…)` と書き、隣の項とは空白で離す。
- **2 次元テキストと 1 行出力の `\mathbf`**: ASCII のまま（設計上の判断。`internal/symbols/alphabet.go` の NOTE を参照）。直さない。
