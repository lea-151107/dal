package mathfont

// The GSUB reader below handles only what math layout needs: the 'ssty'
// feature (script style alternates), with single substitutions (lookup
// type 1) and alternate substitutions (type 3), possibly behind extension
// lookups (type 7). Everything is read through data, which is bounds
// checked, and the counts are capped, because fonts may come from
// Options.Font.

// Caps on the work done for one font.
const (
	maxSstyLookups   = 64
	maxSstySubtables = 256
)

// sstySubtable is one subtable of an ssty lookup.
type sstySubtable struct {
	kind uint16 // 1 single, 3 alternate
	d    data
}

// parseSsty returns the subtables of the font's ssty feature, or nil when
// the font has none (or a GSUB table that cannot be read).
func parseSsty(file []byte) []sstySubtable {
	g, err := findTable(file, "GSUB")
	if err != nil || g.u16(0) != 1 {
		return nil
	}
	features := g.sub(int(g.u16(6)))
	lookups := g.sub(int(g.u16(8)))
	if features == nil || lookups == nil {
		return nil
	}
	var out []sstySubtable
	seen := map[int]bool{}
	nf := int(features.u16(0))
	for i := 0; i < nf && i < 1024; i++ {
		rec := 2 + 6*i
		if string(features[min(rec, len(features)):min(rec+4, len(features))]) != "ssty" {
			continue
		}
		f := features.sub(int(features.u16(rec + 4)))
		n := int(f.u16(2))
		for j := 0; j < n && len(seen) < maxSstyLookups; j++ {
			li := int(f.u16(4 + 2*j))
			if seen[li] || li >= int(lookups.u16(0)) {
				continue
			}
			seen[li] = true
			lk := lookups.sub(int(lookups.u16(2 + 2*li)))
			kind := lk.u16(0)
			ns := int(lk.u16(4))
			for k := 0; k < ns && len(out) < maxSstySubtables; k++ {
				st := lk.sub(int(lk.u16(6 + 2*k)))
				sk := kind
				if sk == 7 { // extension: the real subtable is further on
					sk = st.u16(2)
					st = st.sub(int(st.u32(4)))
				}
				if st != nil && (sk == 1 || sk == 3) {
					out = append(out, sstySubtable{kind: sk, d: st})
				}
			}
		}
	}
	return out
}

// sstyGlyph returns the script style form of g: level 1 for script style,
// 2 for scriptscript style. It returns g when the font has none.
func sstyGlyph(subs []sstySubtable, g uint16, level int) uint16 {
	for _, s := range subs {
		d := s.d
		i := coverage(d.sub(int(d.u16(2))), g)
		if i < 0 {
			continue
		}
		switch {
		case s.kind == 1 && d.u16(0) == 1:
			return uint16(int(g) + int(d.i16(4)))
		case s.kind == 1 && d.u16(0) == 2:
			if i < int(d.u16(4)) {
				return d.u16(6 + 2*i)
			}
		case s.kind == 3 && d.u16(0) == 1:
			if i >= int(d.u16(4)) {
				continue
			}
			set := d.sub(int(d.u16(6 + 2*i)))
			n := int(set.u16(0))
			if n == 0 {
				continue
			}
			return set.u16(2 + 2*(min(level, n)-1))
		}
	}
	return g
}
