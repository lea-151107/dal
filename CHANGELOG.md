# 変更履歴

このファイルには、公開したバージョンごとの変更を書きます。
タグの番号は `Version`（`version.go`）と同じです。`Version` は、同じ入力とオプションからの出力（`Render`・`RenderText`・`RenderLine`）が変わるたびに上がります。

## [0.4.14] - 2026-11-08

最初の公開です。

- LaTeX の数式を pure Go で組みます。TeX のボックスとグルーの規則（TeXbook 付録 G）を、OpenType MATH テーブルのパラメータで動かします。
- 出力は 4 種類です。
  - `Render`: 画像。埋め込みの Latin Modern Math で描き、ベースラインと寸法も返します。
  - `RenderText`: 端末向けの 2 次元テキスト。
  - `RenderLine`: `x²` のような 1 行のテキスト。
  - `Parse`: 構文木。
- KaTeX の対応表の入力例 995 件のうち、967 件を 3 種類の出力のすべてで描けます。
- インラインの式は、`Wrap` を指定すると TeX と同じ位置で折り返します。
- `\text{}` の日本語などは、`Options.TextFace` に渡したフォントで描きます。
- 依存は標準ライブラリと `golang.org/x/image` だけです。cgo・外部プロセス・ネットワーク・ファイル I/O は使いません。
- ライセンスは MIT です。埋め込みフォントは GUST Font License です。

[0.4.14]: https://github.com/lea-151107/dal/releases/tag/v0.4.14
