# テスト用フォント

`NotoSansCJKjp-subset.otf` と `OFL.txt` は dam（`../dam/testdata/fonts/`）の複製です。`TestCopiesFromDam` で同じであることを確かめています。作り方と含まれる文字は dam の `testdata/fonts/README.md` を参照してください。

名前が `_ja` で終わる golden ケース（`testdata/*_ja.tex`）は、このフォントを `Options.TextFace` に渡して描きます。ライブラリには含まれません（Go は `testdata` をビルドしません）。
