# dal

LaTeX の数式を pure Go で組むライブラリです。TeX と同じ「ボックスとグルー」の組版規則（TeXbook 付録 G）を、OpenType MATH テーブルのパラメータで動かします。

- **画像**（`Render`）: 埋め込みの Latin Modern Math で描きます。アンチエイリアスがかかり、背景は透過、前景色は指定できます。ベースラインと寸法も返します。
- **2 次元テキスト**（`RenderText`）: 端末向けの出力です。分数は上下に組み、∑ の範囲は上下に置き、行列は罫線の部品でかっこを組みます。
- **1 行テキスト**（`RenderLine`）: `x²`, `aᵢ`, `(a + b)/c`, `√(x + 1)` のような 1 行にします。代替テキストや検索用の文字列に使えます。
- **構文木**（`Parse`）: 検証だけしたいときや、独自の出力を作りたいときに使います。

依存は標準ライブラリと `golang.org/x/image` だけです。cgo・外部プロセス・ネットワーク・ファイル I/O は使いません。並行して呼んでも安全で、同じ入力・オプション・`Version` からは同じ出力を返します。

## 使い方

```go
res, err := dal.Render(ctx, `\sum_{i=1}^n \frac{1}{i^2}`, dal.Options{
	Display:    true,
	SizePx:     24,
	Foreground: color.White,
})
// res.Image（*image.RGBA）、res.Baseline（上端からベースラインまでの px）、
// res.OriginX（左端から式の原点までの px。インクが左にはみ出すとき以外は PaddingPx）

txt, err := dal.RenderText(ctx, `\frac{a+b}{c}`, dal.TextOptions{MaxCols: 80})
//  a+b
// ─────
//   c

s, err := dal.RenderLine(`x^2 + y_i`, dal.LineOptions{}) // "x² + yᵢ"
```

失敗したときは、画像 → 2 次元テキスト → 1 行 → ソースのまま、の順に切り替えることを想定しています。2 次元テキストと 1 行出力では、`\cancel{x}` を `cancel(x)` のように命令名を付けて書きます（`\bcancel`、`\xcancel`、`\sout` も同様）。

### 幅と折り返し

- `Options.MaxWidth`（画像）と `TextOptions.MaxCols`（2 次元テキスト）は幅の上限です。超えると `ErrTooLarge` になります。
- `Wrap` を指定すると、インラインの式（`Display` が false）が幅を超えたときに、TeX と同じ位置で行を分けて縦に積みます。分ける位置は、関係記号と二項演算子の直後、および `\allowbreak` です。`\nobreak` の直前では分けません。
  - `Baseline` は 1 行めのベースラインです。
  - 分けられない 1 つの断片が幅を超えるときは、`ErrTooLarge` のままです。
  - ディスプレイの式は、KaTeX と同じく分けません。
- ディスプレイの式で幅が分かっているときは、`\tag` をその右端に置きます。式全体が `align` などの 1 つの配列で、式全体の `\tag` が無ければ、行ごとのタグも右端にそろえます。折り返すときは、タグの幅を空けて行を分けます。幅が無ければ、タグは式の後ろに 1em 空けて置きます。

2 次元テキストは、罫線（`─ │`）や `√ ∑ →`、ギリシャ文字など East Asian Ambiguous の文字を 1 桁として組みます。端末で Ambiguous を全角で表示する設定にしていると、位置がずれます。

| エラー | 意味 |
|---|---|
| `*SyntaxError` | 括弧の不一致や引数の不足など。`Offset`（バイト）と `Line`・`Col`（1 始まり）で位置を示します |
| `*UnsupportedError`（`errors.Is(err, ErrUnsupported)`） | 未対応のコマンドや環境（`Command`）。出力ごとに描けないもの（2 次元テキストで否定形の文字が無い `\not`、`TextFace` なしの `\text{日本語}` や数式フォントに字形が無い記号）も含みます |
| `ErrTooLarge` | `Limits` の上限、`MaxWidth` / `MaxCols`、画像のピクセル数（`Limits.MaxPixels`）を超えたとき。絶対値が 1000 を超える寸法（`\kern1000000pt` など）、1024 を超える `SizePx` / `PaddingPx`、内部の上限（マクロ展開後のトークン数、2 次元テキストのセル数、描画量、伸ばす記号の部品数）も含みます |

`Render` は描画中に画像 1 ピクセルあたり約 9 バイトを使います。既定の `Limits.MaxPixels`（`DefaultMaxPixels` = 16M ピクセル）では、1 回の呼び出しで最大約 150MB です。並行して呼ぶときなどにメモリを抑えたい場合は、`Limits.MaxPixels` を小さくしてください。`ctx` は描画中も、塗ったピクセルの量に応じて確認します。キャンセルされた呼び出しは画像を返さず、`ctx.Err()` を返します。

`\text{}` に日本語などを含めるときは、`Options.TextFace` にそのフォントの `font.Face` を作る関数を渡してください。数式フォントにある英字や数字はそのまま数式フォントで描き、数式フォントに無い文字の並びだけを `TextFace` で描きます。`\text` の外（数式モード）に書いた、数式フォントに無い文字（`定`、`½`、`\digamma` など）も、`TextFace` があればそれで 1 文字ずつ立体で描きます。区切りとして伸ばすことはしません。`TextFace` が無いとき、または `TextFace` にもその文字が無いときは、豆腐（代替の四角）を描かずに `ErrUnsupported` を返します。

`\color` などの色名は CSS の色名（148 色）です。KaTeX と同じく `green` は #008000 で、0.3 より前の xcolor の値（#00FF00）とは違います。`#RGB` と `#RRGGBB` も使えます。

`\triangleleft`（◃）のように数式フォントに字形が無い記号は、同じ形の文字（◁）で代わりに描きます。

## 対応している構文

[KaTeX の対応表](https://katex.org/docs/support_table.html) を互換の目標にしています。対応表の入力例 995 件（`testdata/katex_coverage.txt`）を 3 種類の出力に通した結果を、`testdata/katex_coverage.golden` に記録しています（`TestKaTeXCoverage`）。そのうち 967 件は、3 種類の出力のすべてで描けます。残りの 28 件は、Latin Modern Math に字形が無い記号（`\bigstar`、`\subseteqq`、`\lmoustache` など）で、画像では `TextFace` が要ります。この件数は、エラーを返さずに出力できた入力の数です。KaTeX や TeX の描画と見た目や寸法が一致するかは測っていません。組版の正しさは、golden の目視確認と、寸法の不変条件のテスト（区切りが要求の高さまで伸びるか、など）で確かめています。

- **上付き・下付き、プライム**: スクリプトスタイルでは、フォントのスクリプト用の字形（`ssty`）を使います。
- **分数**: `\frac` 系、`\binom` 系、`\over` `\atop` `\choose` `\brace` `\brack` `\above`、`\genfrac`、`\cfrac`
- **根号**: `\sqrt`、`\sqrt[n]`
- **記号**:
  - ギリシャ文字と記号、AMS の関係記号・二項演算子・矢印
  - KaTeX の別名（`\R` `\N` `\lt` `\rarr` `\Alpha` など）
  - コロンの組み合わせ（`\coloneqq` `\dblcolon` など）
- **演算子**: 大型演算子（`\limits` `\nolimits`）、関数名、`\operatorname`、`\varliminf` 系、`\plim`
- **区切り**: `\left` `\middle` `\right`、`\big` 系、braket（`\bra` `\ket` `\braket` `\set` と大文字形）
- **アクセント**:
  - 上: `\hat` `\widehat` `\vec` `\overrightarrow` `\overgroup` `\Overrightarrow` `\overleftharpoon` `\overlinesegment` `\ddddot` など
  - 下: `\utilde` `\underrightarrow` `\undergroup` `\underbar` など
  - 線: `\overline` `\underline`
- **書体**:
  - `\mathrm` `\mathbf` `\mathbb` `\mathcal` `\mathfrak` `\boldsymbol` `\pmb` など
  - 古い形（`\rm` `\bf` など）
  - 文字の大きさ（`\tiny` 〜 `\Huge`）
- **テキスト**:
  - `\text` 系と `\emph`
  - 中で使えるもの: `$…$` と `\(…\)`、テキストのアクセント、`\textdollar` などの記号、`\textcircled`、`\sout`、`\verb`
- **空白と配置**:
  - 空白: `\, \: \; \! \quad \qquad ~ \hspace \kern`（単位は KaTeX と同じ全部）
  - 重なりとずらし: `\smash` `\llap` `\rlap` `\clap` `\mathllap` 系、`\raisebox` `\vcenter`
  - その他: `\phantom` `\hphantom` `\vphantom` `\mathstrut`、`\rule`、`\reflectbox`
- **環境**:
  - 行列: `matrix` 系（`matrix*` 系を含む）、`smallmatrix`
  - 場合分け: `cases` `dcases` `rcases` `drcases`
  - 揃え: `aligned` `align` `alignat` `alignedat` `gather` `gathered` `split` `equation`
  - 配列: `array` `darray`（`l c r | :`、`\hline` `\hdashline`）
  - その他: `CD`、`\substack`、配列の外の `\\` と `\newline`
- **配置の注記**: `\tag`、`\overbrace` `\underbrace` `\overbracket` `\underbracket`、`\overset` `\underset` `\stackrel`、`\xrightarrow` 系
- **色と枠**: `\color` `\textcolor`、`\colorbox` `\fcolorbox`、`\boxed`、`\cancel` 系、`\not`、`\angl` `\phase`
- **マクロ**:
  - 定義: `\newcommand` 系（引数の既定値を含む）、`\def`（区切り付きのパラメータを含む）、`\gdef` `\edef` `\xdef` `\let` `\futurelet` `\global`
  - 展開の制御: `\expandafter` `\noexpand`
  - 有効範囲: 定義はグループの中でだけ有効です。`\gdef` と `\global` はグローバルに定義します。
- **その他**: `\char`、`\mathchoice`、`\TeX` `\LaTeX` `\KaTeX`、`\href` `\url`（リンク先は捨てて文字だけ描きます）、`\htmlClass` 系（中身だけ描きます）

意図的に KaTeX と変えている箇所には、コードに `NOTE(katex-compat)` を付けています。

### 対応していないもの

| 対象 | 理由 |
|---|---|
| mhchem（`\ce` `\pu`） | 化学式の別の文法で、KaTeX でも拡張機能です |
| `\includegraphics` | ファイルやネットワークから画像を読むことになり、I/O を使わない方針に反します |
| MathJax 固有の `\label` `\ref` `\eqref` `\sideset` `\prescript` `\leftroot` | KaTeX も対応していません |
| 数式モードのテキストアクセント（`\'{a}` など） | KaTeX でもエラーです。`\text{\'a}` は使えます |
| 2 次元テキストの色、大きさ、反転、重なり | 文字の格子では表せないので、中身だけを出します |

## 確認用のコマンド

```sh
printf '%s\n' '\int_0^\infty e^{-x^2} dx' | go run ./cmd/dalpng --display > out.png
printf '%s\n' '\frac{a}{b}' | go run ./cmd/dalpng --text
printf '%s\n' '\frac{a}{b}' | go run ./cmd/dalpng --line
printf '%s\n' 'a + b + c = d + e' | go run ./cmd/dalpng --text --cols 10 --wrap
```

## ライセンス

コードは MIT License です（[LICENSE](LICENSE)）。埋め込みフォントの Latin Modern Math は GUST Font License です（[LICENSE-FONT](LICENSE-FONT) と `internal/mathfont/` を参照してください）。
