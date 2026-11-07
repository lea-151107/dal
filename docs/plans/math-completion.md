# dal を「完全」にする計画

## Context

dal（v0.1.5）は、計画書 `math-renderer.md` の第 1 段階と第 2 段階を実装済みである。2026-11-05 に、次の 3 つの方法で不足を洗い出した。

- KaTeX の対応関数一覧（約 1,040 件の入力）を `Render`、`RenderText`、`RenderLine` に通した。
- 記号表の 421 件を総当たりで確かめた。
- 出力を目で見た。

その結果は次のとおり。

- **A 不具合**:
  - `\color{#1a2b3c}` のように `#` の直後が 1〜9 の色が構文エラーになる（字句解析でマクロ引数 `#1` と読まれる）。
  - 画像でだけ描けない記号がある: `\triangleleft`、`\triangleright`、`\digamma`、`\Game`、`\Finv`、`\Coloneqq`、`\pitchfork`、`\lmoustache`、`\rmoustache`。
  - `\hspace*` が構文エラーになる。
- **B 未対応の構文**: 約 300 コマンド（公式の対応表で作り直した後の集計では、996 件のうち 330 件が 3 種類の出力のどれかで通らない）。
  - 古い分数（`\over` など）
  - 環境（`alignat`、`multline`、`matrix*`、`subarray`、`CD`、破線の罫線）
  - 配置（`\smash`、`\llap`、`\raisebox`、`\colorbox` など）
  - 文字の大きさ（`\tiny` 〜 `\Huge`）
  - 伸びるアクセント（`\utilde`、`\overgroup` など）
  - 記号の別名と AMS の記号
  - テキストモードの記号
  - マクロ（`\let`、引数の既定値、区切り付きのパラメータ）
  - その他（`\verb`、`\char`、`\mathchoice`、`\href`、`\set` など）
- **C 組版の品質**:
  - 長い式を折り返さない（`MaxCols` を超えると `ErrTooLarge`）。
  - `ssty` を読まない。
  - MathKern を使わない。
  - `\tag` が右端揃えにならない。
  - マクロの定義がグループを越えて効く。
  - `\hphantom` と `\vphantom` が `\phantom` と同じ扱い。
  - 2 次元テキストで `\cancel` を描けない。
- **D 品質保証**:
  - 記号表とフォントを突き合わせるテストがない。
  - 内部パッケージに単体テストがない。
  - 日本語を含む画像の golden がない。
  - 回帰テストが `review*_test.go` に散らばっている。
  - KaTeX 一覧を使った対応状況の確認がテストになっていない。

**進め方**:
- フェーズごとに `Version` を上げ、`go test . -update` で golden を作り直す。
- 新しい golden と変わった golden の PNG は、すべて目で確かめる。
- フェーズの最後に `/adversarial-review` を通す。指摘を直し終えてから、次のフェーズに進む。

**対象外**（理由を README の「対応していないもの」に書く）:
- **KaTeX にない拡張**:
  - mhchem（`\ce`、`\pu`）: 化学式の別文法で、KaTeX でも拡張機能である。
  - MathJax 固有のもの（`\label`、`\ref`、`\eqref`、`\sideset`、`\prescript`、`\leftroot`）
- **`\includegraphics`**: 画像ファイルの読み込みはファイル I/O とネットワークを禁じる規約に反する。
- **テキストモードのアクセントを数式モードで使うこと**（`\'{a}` など）: KaTeX でもエラーになる。`\text{\'a}` は対応済み。
- **2 次元テキストの色**: `TextResult` は文字列の並びで、色を運ぶ手段がない。NOTE のまま残す。

---

## フェーズ 0: 不具合と品質保証の土台 → v0.2.0

1. **16 進の色**:
   - 字句解析では今までどおり `#1` を `tParam` にする。
   - 色の引数を読むときだけ、`tParam` を `#` と数字に戻して連結する。こうすると、マクロ本体の `\color{#1}` は今までどおり引数として置き換わる。
   - マクロ本体の中で `#1a2b3c` と書くと、TeX と同じく引数として扱われる。この点を NOTE に書く。
2. **画像でだけ描けない記号**: `mathfont` に字形がない文字の扱いを、次の順で決める。
   1. 同じ形の文字への置き換え表（`symbols.ImageSubstitutes`）。◃→◁、▹→▷、⩴→`::=`（3 字形を並べる）。
   2. `TextFace` が渡されていれば、その face で描く（`\text` と同じ TextLeaf の経路を通す）。
   3. どちらも無ければ、`UnsupportedError` を返す。メッセージで `TextFace` を渡せば描けることを示す。
   - ϝ、⅁、Ⅎ、⋔、⎰、⎱ は 2 と 3 で扱う。`\left\lmoustache` のように区切りとして伸ばす場合は、`TextFace` があっても伸ばさず、本文サイズの 1 文字で描く（NOTE を付ける）。
3. **`\hspace*`**: `\hspace` の直後の `*` を読み捨てる。
4. **記号表のテスト**（`internal/symbols` の外側にあるルートのテスト）:
   - 記号、区切り、演算子、アクセント、xarrow の各表の全エントリを `Render` に通す。
   - フォントに無いものは、2 の置き換え表か「TextFace が必要」の一覧に載っていることを確かめる。
5. **単体テスト**: テーブル駆動のテストを置く。
   - `internal/parse`: AST の要約、エラーの位置、マクロ展開
   - `internal/linear`、`internal/textgrid`: 小さな AST から文字列を作る
   - `internal/symbols`: 表どうしの整合性。名前の重複と、`Delims` と `DelimClass` の対応
6. **日本語の画像の golden**:
   - dam の `testdata/fonts/NotoSansCJKjp-subset.otf`（OFL）を、`testdata/fonts/` にライセンスと一緒に複製する。
   - 名前が `_ja` で終わる golden ケースは、このフォントを `TextFace` に使う。
   - `copies_test.go` で dam のものと一致することを確かめる。
7. **回帰テストの集約**: `review_test.go`、`review2_test.go`、`review3_test.go` を `regress_test.go` にまとめる。
8. **KaTeX 一覧との突き合わせのテスト**:
   - `testdata/katex_coverage.txt` は、KaTeX の公式の対応表の入力例から作る。1 行に 1 つの入力を置く。
   - `TestKaTeXCoverage` が 3 種類の出力に通し、結果の表を `testdata/katex_coverage.golden` と照らし合わせる。
   - この表は描画結果ではないので、`testdata/golden` の外に置き、Version の検査の対象から外す。
   - 対象外のものは `## out of scope` の節に置き、集計から外す。

## フェーズ 1: 未対応の構文（B）→ v0.3.0

範囲は KaTeX の公式の対応表（`testdata/katex_coverage.txt`。KaTeX 0.19.1 の `docs/support_table.md` から作成）で決める。フェーズ 0 のレビューで、記憶から作った最初の一覧には KaTeX にないもの（`multline`、`subarray` 環境、`\strut`、`\iddots`、`\mathsfit`、`\TextOrMath`、`\sgn`）が混じっていると分かったので、それらは範囲から外した。

フェーズ 1 の完了条件: 表のうち対象外を除く全件が、3 種類の出力のすべてで `ok` になる。ただし、次の 2 つは例外として表に残す。
- 画像で `TextFace` が要る 28 件: Latin Modern Math に字形が無く、同じ形の代わりの文字も無いもの
  - 文字と記号: `\digamma`、`\Game`、`\Finv`、`\pitchfork`、`\bigstar`、`\blacklozenge`、`\circledS`、`\minuso`、`\doublebarwedge`、`\char"263a`
  - 区切り: `\lmoustache`、`\rmoustache`、`\lBrace`、`\rBrace`
  - 矢印: `\dashleftarrow`、`\dashrightarrow`
  - AMS の関係記号: `\precapprox` 系、`\succapprox` 系、`\subseteqq` 系、`\supseteqq` 系、`\varsubsetneqq`、`\varsupsetneqq`
- 2 次元テキストの `\cancel` 系（フェーズ 2 で直す）

1. **記号の別名と記号の追加**（表に足すだけのもの）:
   - 別名:
     - 集合と文字: `\R`、`\N`、`\Z`、`\Complex`、`\cnums`、`\natnums`、`\reals`、`\Reals`、`\real`、`\image`、`\weierp`、`\alef`、`\alefsym`、`\thetasym`、`\exist`、`\isin`、`\empty`
     - 記号: `\lt`、`\gt`、`\plusmn`、`\sdot`、`\cdotp`、`\ldotp`、`\bull`、`\And`、`\Dagger`、`\sub`、`\sube`、`\supe`、`\notni`
     - トランプ: `\clubs`、`\diamonds`、`\hearts`、`\spades`
   - 区切りの別名（`Delims` にも足す）: `\lparen`、`\rparen`、`\lang`、`\rang`、`\lBrace`、`\rBrace`
   - 矢印の別名: `\rarr`、`\Rarr`、`\larr`、`\lrarr`、`\harr`、`\uarr`、`\darr` などとその大文字形、`\mapsfrom`
   - ギリシャ大文字のラテン形: `\Alpha` など
   - コロンの組み合わせ: `\coloncolon`、`\colonequals`、`\approxcolon` など。字形が無いものは、`:` と関係記号を並べて組む（`ImageSubstitutes` と同じ方法）。
   - AMS の関係記号と二項演算子: `\leqq`、`\lessapprox`、`\precneqq`、`\doublecap`、`\gtrdot` など、表で未対応のもの全部
   - 記号: `\bigstar`、`\blacktriangle` 系、`\blacklozenge`、`\maltese`、`\measuredangle`、`\sphericalangle`、`\diagup`、`\diagdown`、`\minuso`、`\circledR`、`\circledS`、`\copyright`、`\pounds`、`\yen`、`\mathsterling`、`\Bbbk`、`\daleth`、`\gimel`、`\imageof`、`\origof`、`\mathellipsis`、`\ratio`、`\triangledown`
2. **分数の一般形**:
   - `\over`、`\atop`、`\choose`、`\brace`、`\brack`、`\above{dim}` は、リストの途中に現れる中置の命令として `listUntil` で扱う。そのリストの前半を分子、後半を分母にする。1 つのリストに 2 回現れたら `SyntaxError` にする。
   - `\genfrac{左}{右}{太さ}{スタイル}{分子}{分母}` に対応する。
   - `ast.Frac` には線の太さ（`Thickness`。0 なら線なし、負なら既定の太さ）を足す。
3. **環境**:
   - `matrix*` の系列（`matrix*`、`pmatrix*`、`bmatrix*`、`Bmatrix*`、`vmatrix*`、`Vmatrix*`）: 省略可能な引数 `[l|c|r]` を受け付ける。
   - `drcases`
   - `alignat`、`alignat*`、`alignedat`: 列の組の数の引数を受け付け、組の間に隙間を空けない。
   - `\substack`: スクリプトスタイルで中央揃えの Array にし、列の間隔を 0 にする。
   - `CD`:
     - `@>a>>`、`@<<<`、`@VVV`、`@AAA`、`@=`、`@|`、`@.` を読み、格子状の Array に変換する。
     - 横の矢印のセルは XArrow にする。縦の矢印のセルは、伸ばした縦矢印の左右にラベルを置いた Group にする。
     - 新しい AST の型は作らない。
   - 破線: `\hdashline` と列指定の `:`。`ast.Row.HLineDashed` と `ast.Column.Dashed` を足す。
     - 画像では Rule を並べて破線にする。
     - 2 次元テキストでは `┄` と `┆` で描く。
   - `\newline` は、`\\` と同じく配列の外ではエラーにせず、改行として扱う。トップレベルでは、`gathered` と同じく行を中央揃えで縦に積む。
4. **配置**:
   - 新しい AST の型:
     - `Smash{Top, Bottom}`: `\smash`、`\smash[t]`、`\smash[b]`
     - `Lap{Align}`: `\llap`、`\rlap`、`\mathllap`、`\mathrlap`、`\mathclap`
     - `Raise{Mu}`: `\raisebox{dim}{text}`。`\vcenter` は、数式の軸に中心を合わせる Raise にする。
     - `ColorBox{Bg, Frame *color, Body}`: `\colorbox`、`\fcolorbox`
     - `Reflect`: `\reflectbox`、`\mathreflectbox`（左右反転。2 次元テキストと 1 行では中身をそのまま出す）
     - `Rule{W, H, Raise}`: `\rule[raise]{w}{h}`
   - 既存の型で表すもの:
     - `\sout` は `Cancel{Kind: "-"}` にする。
     - `\mathstrut` は、`(` の高さと深さを持つ幅 0 の Phantom にする。
   - `Phantom` に `H` と `V` を足し、`\hphantom`（高さと深さを 0）と `\vphantom`（幅を 0）を正しく扱う（C の項目もここで片付ける）。
   - 2 次元テキストと 1 行での扱い:
     - Smash、Lap、Raise は中身をそのまま出す。
     - ColorBox は、枠付きなら `\boxed` と同じに、枠なしならそのまま出す。
     - Rule は、2 次元テキストでは幅の分だけ `█` にし、1 行では出さない。
5. **文字の大きさ**:
   - `\tiny`、`\sixptsize`、`\scriptsize`、`\footnotesize`、`\small`、`\normalsize`、`\large`、`\Large`、`\LARGE`、`\huge`、`\Huge` を、`\displaystyle` と同じ「リストの残りに効く」命令として扱う。
   - `ast.Size{Factor}` を足す。倍率は KaTeX と同じにする。
   - box では `layouter.scale` にこの倍率を掛ける。2 次元テキストと 1 行では無視する（NOTE）。
6. **伸びるアクセントと括弧**:
   - アクセント: `\overgroup`、`\undergroup`、`\Overrightarrow`、`\overleftharpoon`、`\overrightharpoon`、`\overlinesegment`、`\underlinesegment`、`\utilde`、`\underrightarrow`、`\underleftarrow`、`\underleftrightarrow`、`\underbar`、`\ddddot`
   - 括弧: `\overbracket`、`\underbracket`（`\overbrace` の角形）
   - xarrow: `\xleftrightharpoons`、`\xtofrom`
   - `ast.Accent` に `Under` を足す。
   - 画像では、MathVariants に横方向のサイズ違いと組み立てがあればそれを使う。無ければ、字形と Rule で組む。
7. **テキストモード**:
   - `textSymbols` に足す: `\textdollar`、`\textunderscore`、`\textbraceleft`、`\textbraceright`、`\textregistered`、引用符の命令、`\lq`、`\rq`、`\textdagger`、`\textdaggerdbl`、`\textsterling`、`\textbardbl`、`\sect`、`\j`
   - `\textcircled{a}` は、丸囲みの文字に置き換える。
   - KaTeX と同じく、これらを数式モードに書いてもテキストとして描く。
   - テキストのアクセント（`\.`、`\H`、`\r`、`\v` など）: 合成済みの文字の表を広げる。無い組み合わせは結合文字を付ける（2 次元テキストでは従来どおり `ErrUnsupported`）。
   - `\emph` は `\textit` にする（入れ子では立体に戻す）。
   - `\sout` は `\text` の中でも使えるようにする。
   - `\text` の中の `\( … \)` は、数式に戻す（`$ … $` と同じ）。
8. **マクロ**:
   - `\let\a=\b`: その時点での定義を写す。組み込みの命令なら別名にする。
   - `\edef` と `\xdef`: 本体を定義の時点で展開する。
   - `\global` と `\long` は、続く定義に付く接頭辞として受け付ける（`\long` は効果なし）。
   - `\expandafter`、`\noexpand`、`\futurelet`
   - `\newcommand` の省略可能な引数（既定値）、`\def` の区切り付きのパラメータ
   - **定義の有効範囲**:
     - `\newcommand` と `\def` の定義は、そのグループの中でだけ有効にする（TeX と KaTeX に合わせる）。
     - `\gdef`、`\xdef`、`\global` の定義はグローバルにする。
     - `\begingroup … \endgroup` も 1 つのグループとして扱う。
9. **その他**:
   - `\verb!…!`: 字句解析で特別に扱い、等幅のテキストにする。
   - `\char"263A`、`\char9786`
   - `\mathchoice{D}{T}{S}{SS}`: `ast.Choice` を足す。
   - `\href{url}{body}`、`\url{url}`: body または URL を描く。リンクの情報は捨てる（NOTE）。
   - `\htmlClass`、`\htmlId`、`\htmlStyle`、`\htmlData`: 中身だけを描く。
   - `\KaTeX`、`\LaTeX`、`\TeX`: ロゴを組む。
   - `\pmb`: `\boldsymbol` で代用する（NOTE）。
   - braket: `\bra`、`\ket`、`\braket`、`\Bra`、`\Ket`、`\Braket`、`\set`、`\Set`
     - 小文字形は大きさの決まったかっこにする。
     - 大文字形は `\left … \right` にし、中の `|` を `\middle|` にする。
   - `\angl{n}` と `\phase{x}`: 角の記号を線で描く。
   - 演算子名: `\varliminf`、`\varlimsup`、`\varinjlim`、`\varprojlim`、`\plim`
   - `$` で始まる入力（`$x_1 x_2 \dotsm x_n`）: 表の例をそのまま貼った形。KaTeX でも `$` は構文エラーなので、表の対象外の節に移す。

## フェーズ 2: 組版の品質（C）→ v0.4.0

1. **折り返し**:
   - `TextOptions.Wrap` と `Options.Wrap`（bool）を足す。
   - 有効なときは、最上位のリストを TeX と同じ規則で区切る。区切る位置は、関係記号と二項演算子の直後（`\allowbreak` を含む）。`\nobreak` の位置と、`\left…\right` の中では区切らない。
   - 区切った位置で `MaxCols`、`MaxWidth` に収まるように、貪欲法で行を詰める。
   - 1 つの断片でも収まらなければ、今までどおり `ErrTooLarge` を返す。
   - ディスプレイの式（`Display`）と配列は区切らない。KaTeX もディスプレイの式は折り返さない。
   - 画像は行を縦に積み、行送りは 1.2em にする。`Baseline` は 1 行目のベースラインにする。
   - 2 次元テキストも行のブロックを縦に積み、`Baseline` は 1 行目にする。
2. **`\tag` の右寄せ**: ディスプレイのとき、幅（`MaxWidth` か `MaxCols`）が指定されていれば、タグをその右端に揃える。幅が無いときは今までどおり 1em 空けて置く。
3. **`ssty`**:
   - `mathfont` に GSUB の最小限のパーサを書く。対象は、`ssty` 機能の LookupType 1（単一の置換）と LookupType 3（代替の一覧）。
   - スクリプトスタイルの字形に、`ssty` の 1 段目と 2 段目を使う。
   - プライムは TeX と同じく上付きとして組む。`′` の `ssty` 形は位置が上がっている。こうすると、プライムを本文サイズで置くという NOTE を外せる。
   - どちらのパーサにも、テーブルの境界の確認と、ルックアップの数の上限を設ける。
4. **MathKern**:
   - `MathKernInfo` を読む。
   - 規則 17 と 18 で上付き・下付きを付けるとき、底と添字の角にあるカーンを、高さに応じて引いて足す（OpenType MATH 仕様の手順に従う）。
5. **2 次元テキストの `\cancel` と `\sout`**: 1 行出力と同じく、命令名を付けて `cancel(x)` のように書く。高さのある中身は、伸ばしたかっこで包む。`ErrUnsupported` は返さなくなる。

## フェーズ 3: 仕上げ（D）→ v0.4.x

- Fuzz の種に、フェーズ 1 と 2 の新しい構文を足す。Fuzz を各 2 分ずつ回す。
- `GOAMD64=v3` と `-tags noasm` でも golden と一致することを確かめる。
- README を更新する: 対応表、対象外の一覧と理由、`Wrap` と `TextFace` の役割。
- `math-renderer.md` の「分かっている制限」を更新する。
- ベンチマークを取り直し、性能が後退していないことを確かめる。
- KaTeX 一覧との突き合わせの結果（`ok` の件数、対象外の件数）を、この文書の末尾に記録する。

---

## 検証（各フェーズの最後に行う）

- `gofmt -s -l .`、`go vet ./...`、`CGO_ENABLED=0 go build ./...`
- `go test ./...`。golden、Version、依存の向き、dam からの複製の一致を含む。
- `GOAMD64=v3 go test .` と `go test -tags noasm .`
- `go test -run '^$' -fuzz <各Fuzz> -fuzztime 1m .`（フェーズ 3 では 2 分）
- 新しい golden と変わった golden の PNG と `.txt` を目で確かめる。
- `/adversarial-review` を通し、指摘を直してからフェーズを閉じる。結果はこの文書の「進捗」に記録する。

## 進捗

### フェーズ 0（v0.2.0、2026-11-05 完了）

- 実装した項目: 1〜8 のすべて。
- 回帰テスト: `regress_test.go` の `TestPhase0Fixes`。
- 単体テスト: `internal/parse`、`internal/linear`、`internal/textgrid`、`internal/symbols` の各 `*_test.go`。
- 既存の golden: 変わっていない。変更した経路は、以前はエラーを返していた分岐だけだった。
- 新しい golden: `text_ja`（画像、2 次元テキスト、1 行）。目で確かめた。
- 検証: gofmt、vet、`CGO_ENABLED=0` のビルド、全テスト、`GOAMD64=v3`、`-tags noasm`、Fuzz（5 種類を各 1 分）がすべて通った。
- 突き合わせ: 995 件のうち 666 件が、3 種類の出力のすべてで通る。
- **レビューで直したこと**（/adversarial-review）:
  - 突き合わせの一覧を、記憶から作ったものから KaTeX の公式の対応表に作り直した。KaTeX にないものが混じっていたためで、フェーズ 1 の範囲もこれで引き直した。
  - 数式モードで、数式フォントに無い文字を TextFace で描くようになった。この挙動の変化を、`Options.TextFace` の doc と README に書いた。
  - 突き合わせの表を Version の検査の対象から外した。

### フェーズ 1（v0.3.2、2026-11-05 完了）

- 実装した項目: 1〜9 のすべて。
- 突き合わせ: 995 件のうち 963 件が、3 種類の出力のすべてで通る。残り 32 件の内訳は次のとおりで、どちらも完了条件の例外にあたる。
  - 画像で TextFace が要る 28 件
  - 2 次元テキストの `\cancel` 系 4 件（フェーズ 2 で直す）
- 計画から変えた点:
  - 2 次元テキストの結合文字: これまで `ErrUnsupported` にしていたが、直前の文字と同じセルに入れて残すようにした（`\text{\H{a}}` → a̋）。もとの懸念は「黙って消える」ことだったので、消さずに残せばその懸念は解消する。
  - 色名: xcolor の基本色から、CSS の 148 色（dam と同じ表）にした。`green` などの値は KaTeX と同じになる。
  - フォントに無い合成済みの文字（ǎ、ȧ）: 元の字と結合文字に分けて描く。
- 新しい golden: `genfrac`、`envs_more`、`cd`、`alignat`、`layout`、`sizes`、`accents_more`、`textmode`、`misc_more`、`lines`、`macros_more`、`substack`。PNG はすべて目で確かめた。
- 既存の golden: v0.2.0 と比べて 1 つも変わっていない（ハッシュで比較した）。
- 検証: gofmt、vet、`CGO_ENABLED=0` のビルド、全テスト、`GOAMD64=v3`、`-tags noasm`、Fuzz（KaTeX の対応表の全入力を種に加え、5 種類を各 1 分）。
- **レビューで直したこと**（/adversarial-review）:
  - `\text` の中の `\verb` が黙って消えていた。
  - `\char` が特殊文字（`{`、`$`、`#`、`~`）を構文として解釈していた。
  - `\edef` の本体にある `\def` が、定義の時点で実行されていた。
  - CSS 色への変更を README に書いた。
  - 回帰テストは `TestPhase1Fixes`。
  - レビューの修正後に、`FuzzParse` と `FuzzRender` を各 40 秒回し直した。

### フェーズ 2（v0.4.1、2026-11-05 完了）

- **実装した項目**: 1〜5 のすべて。
- **突き合わせ**: 995 件のうち 967 件が、3 種類の出力のすべてで通る。残り 28 件は、画像で TextFace が要る記号。
- **計画から変えた点、補足**:
  - **`\tag` の右寄せ**:
    - 幅が分かっているとき（`MaxWidth`、`MaxCols`）に、ディスプレイの式のタグを右端に置く。式そのものは左寄せのまま。
    - 式全体が 1 つの配列（`align` など）のときは、行ごとのタグも右端にそろえる。
    - 2 次元テキストでは、幅の違う行のタグを右揃えにした。
  - **MathKern**:
    - Latin Modern Math には MathKernInfo が無いので、同梱のフォントでは見た目が変わらない。`Options.Font` で MathKern を持つフォントを渡したときに効く。
    - カーンを求める部分は `cutIns` に切り出し、単体テストした。
    - 描画を通した確認は `TestMathKern`（`mathkern_test.go`）で行う。テストの中で、同梱のフォントの MATH テーブルに MathKernInfo を書き足したフォントを組み立てる（イタリックの V の右上に +50、右下に −100）。それで描くと、`V_1` は 10px 狭く、`V^2` は 5px 広くなり、カーンの無い `x_1` は変わらないことを確かめる。
    - このテストは `cutIns` を無効にすると失敗することも確かめた。
  - **`ssty`**:
    - スクリプトスタイルの記号、演算子名、`\text` のうち数式フォントで描く文字に、スクリプト用の字形を使う。
    - プライムは通常の上付きにした。これで「プライムは本文サイズで置く」という NOTE は外れた。
    - `A'_1` のように、プライムと下付きが上下にそろうようになった。
- **golden**: v0.3.2 と比べて 31 件が変わった（スクリプトの字形、プライム、2 次元テキストの `\cancel`）。代表的な PNG を目で確かめた。
- **検証**: gofmt、vet、`CGO_ENABLED=0` のビルド、全テスト、`GOAMD64=v3`、`-tags noasm`、Fuzz（5 種類を各 1 分）。
- **レビューで直したこと**（/adversarial-review）:
  - 折り返しが、改行候補ごとに行頭から組み直していたので、二乗の時間がかかっていた（2,800 ノードで 0.86 秒）。要素の位置を 1 回だけ求める線形の方式にした。回帰テストは `TestWrapLinear`（2,400 項を 1 秒以内）。
  - スクリプトの中の `\text` にも `ssty` を使うようにした（v0.4.1）。
- **Fuzz で見つけた不具合**: `\not\notag` が panic した（何も生まない命令の結果の nil が `\not` の中身に入っていた）。フェーズ 2 より前からあった不具合で、Fuzz の種に KaTeX の対応表を足したことで見つかった。`\not` の後ろが何も生まない命令なら構文エラーにし、見つかった入力は `testdata/fuzz/FuzzRender/` に回帰の種として残した。直した後に、5 種類の Fuzz を各 1 分ずつ、ログを残して回し直し、すべて通った。
- **残る制限**: インラインの式で、折り返しと `\tag` を併用すると、タグは 1 行めの、最も長い行より右に付く。

### フェーズ 3（v0.4.1、2026-11-05 完了）

- **Fuzz の種**: フェーズ 1 で次を足した。
  - KaTeX の対応表の全入力
  - 壊れかけの入力（閉じていない `\verb` や `CD`、範囲外の `\char`、空の `\genfrac` など）
  - フェーズ 2 で見つけた `\not\notag` は、`testdata/fuzz/FuzzRender/` に残してある。
- **Fuzz**: 5 種類を各 2 分（結果は下の「最終確認」に書く）。
- **ビルドの変種**: `GOAMD64=v3` と `-tags noasm` で golden と一致する。
- **文書**:
  - README: 対応している構文、対応していないものと理由、幅と折り返し、`TextFace` の役割、CSS 色
  - `math-renderer.md` の「分かっている制限」に、解消したものを追記した。
  - `CODING_GUIDELINES.md` のテストの節: テスト用フォント、`TestKaTeXCoverage`、`TestSymbolTablesRender`、処理量を線形に保つ規則
- **性能**（`\sum_{i=1}^n \frac{1}{i^2}`、amd64、5 回の中央値）:
  - Render: 約 57µs（v0.1 時点は約 47µs）
  - RenderText: 約 4.5µs（同 3.8µs）
  - RenderLine: 約 2.6µs（同 2µs）
  - どれも 2 割前後遅くなった。`ssty` に関係しない RenderText と RenderLine も同じ割合なので、主な原因は構文解析の変更（展開器のスコープ管理、トークンの印）と見ている。いずれも計画の目標（Render は数 ms、RenderText は 1ms 未満）の範囲に収まっている。
- **KaTeX の対応表との突き合わせ**（`testdata/katex_coverage.golden`）:
  - 対象外を除く 995 件のうち、3 種類の出力のすべてで通るのは 967 件（v0.1.5 の時点は 666 件）。
  - 残る 28 件は、画像で TextFace が要る記号（フェーズ 1 の例外の一覧）。
  - 対象外: mhchem、`\includegraphics`、MathJax 固有の命令、`$` で始まる入力の 9 件。
- **レビューで直したこと**（/adversarial-review）:
  - `internal/parse/parse.go` に、もう当てはまらない TODO（`\text` の入れ子と数式モード）が残っていた。`argText` の役割を説明するコメントに置き換えた。
  - 「最終確認」の節を設けた（下）。
  - 対応件数（967 / 995、例外 28 件）は、表の集計と一致することを確かめた。

## 最終確認（2026-11-05、v0.4.1）

- **静的な検査とビルド**: `gofmt -s -l .` は差分なし。`go vet ./...`、`CGO_ENABLED=0 go build ./...` が通った。
- **テスト**: `go test -count=1 ./...` の全 8 パッケージが通った。golden、Version、依存の向き、dam からの複製、KaTeX の対応表、記号表、回帰テストを含む。
- **ビルドの変種**: `GOAMD64=v3 go test .` と `go test -tags noasm .` が、golden と一致した。
- **Fuzz**: FuzzParse、FuzzRender、FuzzRenderText、FuzzRenderLine、FuzzMathTable を各 2 分回し、すべて通った（失敗の入力なし）。
- **dam**: dal の変更（テスト用フォントと色の表の共有）の後も、dam のテストが通った。
- **フェーズごとのレビュー**: フェーズ 0〜3 の終わりに、それぞれ /adversarial-review を通し、指摘を直した。計画の手順（検証を終えてから次のフェーズに進む）からずれた点は、次の 4 つ。
  1. **フェーズ 0**: レビューと修正は、フェーズ 1 の作業より前に行った。ただし、フェーズ 0 の Fuzz（各 1 分）が終わる前に、フェーズ 1 の記号表の追加を始めていた。その Fuzz の後半は、フェーズ 1 の途中のコードに対して回っている。また、フェーズ 0 のレビューで「確かめること」に挙げた件数の数え直しは、フェーズ 3 で行った。
  2. **フェーズ 1**: レビューと修正は、フェーズ 2 の作業より前に行った。ただし、Fuzz の待ち時間に、フェーズ 2 の GSUB のパーサ（`internal/mathfont/gsub.go`。既存のコードからは呼ばれない新しいファイル）を書いていた。レビューの修正の後に、`FuzzParse` と `FuzzRender` を回し直している。
  3. **フェーズ 2**: MathKern は、フェーズを閉じた時点では `cutIns` の単体テストでしか確かめていなかった。描画を通した確認（`TestMathKern`）は、フェーズ 3 の後に、停止フックの指摘を受けて足した。
  4. **フェーズ 2 の Fuzz**: 実行中にレビューの修正を入れたため、1 回めの FuzzRender は失敗の入力を残さずに失敗した。ログを残して回し直したところ、`\not\notag` の panic を見つけた（フェーズ 2 の節に記録したとおり、直してある）。

### 停止フックの指摘への対応（2026-11-05）

1. **フェーズ 0 のレビューの「確かめること」**: 2 件とも済んでいる。
   - 突き合わせの表を公式の対応表で作り直し、それに合わせてフェーズ 1 の範囲を引き直した（フェーズ 1 の節の冒頭）。
   - 数式モードで TextFace を使うことを、`Options.TextFace` の doc と README に書いた。
   - 表の集計（967 / 995、例外 28 件）が表の中身と一致することは、フェーズ 3 のレビューで数え直して確かめた。
2. **フェーズ 3 の Fuzz**: 5 種類それぞれの終了コードを、ログ（`go test -fuzz` の出力）で確かめた。FuzzParse、FuzzRender、FuzzRenderText、FuzzRenderLine、FuzzMathTable のすべてが `exit=0` で、失敗の入力は無かった。
3. **golden の目視**:
   - フェーズ 2 で変わった PNG 29 件はすべて確かめた。うち 21 件は、変更前（v0.3.2）と変更後を上下に並べた一覧画像を作って見比べた。違いはスクリプトの字形（`ssty`）だけで、レイアウトの崩れや欠けは無かった。
   - フェーズ 1 の新しい golden 12 件とフェーズ 0 の `text_ja` は、作ったときに 1 件ずつ見ている。
   - 変わったテキストの golden（`cancel.txt`、`textmode.txt`）は中身を確かめた。
4. **MathKern の確認**: 上のフェーズ 2 の節のとおり、MathKernInfo を書き足したフォントで描いて確かめた（`TestMathKern`）。

## 最終レビュー（サブエージェントによる敵対的レビュー）後の修正（v0.4.2、2026-11-05）

深刻な指摘 3 件を直した。

1. **配列の行タグと式全体の `\tag` を両方使うと、どの幅でも `ErrTooLarge` になっていた。** また `Wrap` は、行幅からタグの幅を差し引いていなかった。次のように直した。
   - 式全体にタグがあるときは、行タグを右端に寄せない。
   - 折り返すときは、行幅からタグとその前の間隔を先に差し引く。
2. **既定以外のオプション（`MaxWidth`、`MaxCols`、`Wrap`、タグの右寄せ）の出力が、Version の検査の外にあった。** `options_golden_test.go` の `TestGoldenOptions` に 5 ケースを加え、`testdata/golden/opt_*` に置いた。golden.version のハッシュの対象に入ったので、これらの出力が変わると Version を上げない限りテストが失敗する。
3. **`\text` の中の、数式フォントにも TextFace にも無い文字が、豆腐（代替の四角）として黙って描かれていた。** TextFace の字形の有無を確かめ、無ければ `ErrUnsupported` にした。これまで豆腐の描画を前提にしていたテスト（`TestTextFace`、`TestTextFaceMissingMark`）は、日本語のテスト用フォントを使う形に直した。

回帰テストは `TestFinalReviewFixes`。

### 細かい指摘の修正（v0.4.3）

1. **マクロ定義の有効範囲**: 配列のセル、環境、`\left … \right` をグループとして扱う。展開器の `pushScope` と `popScope` を使う。
2. **組み込みの命令の扱い**: `\newcommand` で組み込みの命令や既存のマクロを指定すると構文エラーにし、`\providecommand` は既にあれば何もしない。組み込みかどうかは `internal/parse/builtins.go` の `isBuiltin` で判定する。構文解析器が扱う命令名の一覧は、`TestParserCommandsListed` でソースの `case` と突き合わせる。KaTeX の別名（`\R` など）は組み込みとみなさない（NOTE(katex-compat)。LaTeX には無く、文書が自分で定義することが多いため）。
3. **`##`**: マクロ本体の `##1` を、本体の中の定義のパラメータとして扱う。
4. **SizePx**: 1 未満は不正な値としてエラーにする（非正規化数で NaN の寸法になっていた）。
5. **URL**: `\url` と `\href` の URL は、字句解析の段階でそのまま読む。`%` と `#` は URL の文字として扱い、`\%` などは記号に戻す。

回帰テストは `TestMinorReviewFixes`。golden は変わっていない。ただし、以前と出力が変わる入力（重複した `\newcommand` や `%` を含む URL など）があるので、Version を上げた。

## 2 回目の最終レビュー（サブエージェント）後の修正（v0.4.4、2026-11-05）

v0.4.3 の修正が持ち込んだ回帰と、新たな指摘を直した。

1. **`\url{#1}` をマクロの中で使うと `#1` のまま出ていた**（v0.4.3 で URL を逐語的に読むようにしたことによる回帰）。また `\url` を再定義できなくなっていた。次のように直した。
   - マクロ本体の URL のトークンの `#n` を、展開するときに引数の文字列（`\%` などは記号に戻す）で置き換える。
   - 利用者のマクロが URL のトークンを引数として受け取ったら、TeX と同じく通常のトークンに字句解析し直す。
2. **`\text` の中の豆腐の抜け道**: CJK の直後に来た結合文字が、どちらのフォントにも無い場合も `ErrUnsupported` にした。
3. **組み込みの判定の漏れ**: 次を加え、`\def` による再定義もエラーにした（展開器が先に扱うため、定義しても黙って無視されていた）。
   - 構文解析器が別の場所で扱う命令: `\\`、`\cr`、`\newline`、`\hdashline`、`\textcircled`、`\(`、`\)`
   - 展開器が扱う命令: `\begingroup` 系、`\long` など
   - `TestParserCommandsListed` は、`env.go`、`text.go`、`parse.go` の命令名も突き合わせる。
4. **細かい指摘**:
   - 配列の外の `\\` で分けた行は、グループを作らない（KaTeX と同じ）。
   - `#` が続いた場合は、TeX の規則（`##` は `#`）で、連続の長さに応じて展開する（`###1` は `#` と引数）。
   - 行にタグを持つ環境（`align` など）が式の全体でないときは、構文エラーにする（タグを置く場所が無いため）。
5. **Version の検査の穴**:
   - **golden が取り残される問題**: Version が同じままで記録済みの golden が無くなったら失敗する。どの fixture にも対応しない golden も、`TestNoOrphanGoldens` で失敗させる。fixture の名前を変えると、この 2 つで Version を上げることが求められる（dam も同じ）。
   - **golden に表れない出力の変化**: `TestGoldenOutputs` が、KaTeX の対応表の全入力と、新しい「回帰」の節の入力について、次の 3 つを `testdata/golden/katex_outputs.txt` に記録する。どの入力の出力が変わっても、Version を上げる必要がある。
     - 1 行の出力
     - 2 次元テキストのハッシュ
     - 画像のハッシュ
