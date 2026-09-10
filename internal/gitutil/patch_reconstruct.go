package gitutil

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var ErrPatchImageBudget = errors.New("postimage exceeds image retention budget")

// ReconstructPatchPostimage applies an already normalized effect exactly in
// memory. False means the strict representation has no executable payload;
// an invalid payload or context mismatch is an error, never a limitation.
// limit bounds the newly retained image, independently of the input image.
func ReconstructPatchPostimage(patch string, effect PatchEffect, pre []byte, limit int64) ([]byte, bool, error) {
	effects, err := NormalizePatchEffects(patch)
	if err != nil {
		return nil, false, fmt.Errorf("invalid patch grammar")
	}
	if effect.Ordinal < 1 || effect.Ordinal > len(effects) ||
		effects[effect.Ordinal-1] != effect {
		return nil, false, fmt.Errorf("effect is not the normalized patch projection")
	}
	fragment := patch[effect.FragmentStart:effect.FragmentEnd]
	lines := splitPatchLines(fragment)
	starts := patchRecordStarts(lines)
	if len(starts) == 2 {
		first, err := normalizeOneRecord(lines, starts[0], starts[1])
		if err != nil {
			return nil, false, fmt.Errorf("invalid type-change deletion")
		}
		second, err := normalizeOneRecord(lines, starts[1], len(lines))
		if err != nil {
			return nil, false, fmt.Errorf("invalid type-change addition")
		}
		if !first.BinaryStanza {
			removed, err := reconstructTextRecord(fragment[:lines[starts[1]].start], pre, limit)
			if err != nil || len(removed) != 0 {
				return nil, false, fmt.Errorf("type-change deletion does not match reference")
			}
		} else if deletion := fragment[:lines[starts[1]].start]; strings.Contains(deletion, "\nGIT binary patch\n") {
			removed, err := reconstructBinaryRecord(deletion, pre, limit)
			if err != nil || len(removed) != 0 {
				return nil, false, fmt.Errorf("invalid binary type-change deletion")
			}
		}
		addition := fragment[lines[starts[1]].start:]
		if second.BinaryStanza {
			if !strings.Contains(addition, "\nGIT binary patch\n") {
				return nil, false, nil
			}
			post, err := reconstructBinaryRecord(addition, nil, limit)
			return post, err == nil, err
		}
		post, err := reconstructTextRecord(addition, nil, limit)
		return post, err == nil, err
	}
	if effect.BinaryStanza {
		if strings.Contains(fragment, "\nGIT binary patch\n") {
			post, err := reconstructBinaryRecord(fragment, pre, limit)
			return post, err == nil, err
		}
		return nil, false, nil
	}
	return reconstructTextEffect(patch, effect, pre, limit)
}

// Binary payloads are transformations too. A marker-only binary diff has no
// body; a GIT binary patch has a literal/delta body and must not be downgraded
// to that limitation when decompression or exact delta application fails.
func reconstructBinaryRecord(fragment string, pre []byte, limit int64, expected ...[]byte) ([]byte, error) {
	_, payload, ok := strings.Cut(fragment, "\nGIT binary patch\n")
	if !ok {
		return nil, fmt.Errorf("binary payload header absent")
	}
	lines := splitPatchLines(payload)
	if len(lines) < 2 {
		return nil, fmt.Errorf("binary payload truncated")
	}
	fields := strings.Fields(lines[0].text)
	if len(fields) != 2 || (fields[0] != "literal" && fields[0] != "delta") ||
		lines[0].text != fields[0]+" "+fields[1] {
		return nil, fmt.Errorf("invalid binary payload kind")
	}
	size, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || size < 0 {
		return nil, fmt.Errorf("invalid binary payload size")
	}
	if size > 32<<20 || (len(expected) == 0 && fields[0] == "literal" && size > limit) {
		return nil, ErrPatchImageBudget
	}
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz!#$%&()*+-;<=>?@^_`{|}~"
	var compressed []byte
	var rest string
	for _, line := range lines[1:] {
		if line.text == "" {
			rest = strings.Trim(payload[line.end:], "\n")
			break
		}
		if len(expected) != 0 && rest != "" {
			return nil, fmt.Errorf("binary patch has more than two payloads")
		}
		text := line.text
		n := 0
		if text[0] >= 'A' && text[0] <= 'Z' {
			n = int(text[0]-'A') + 1
		}
		if text[0] >= 'a' && text[0] <= 'z' {
			n = int(text[0]-'a') + 27
		}
		if n == 0 || len(text)-1 != (n+3)/4*5 {
			return nil, fmt.Errorf("invalid binary base85 line")
		}
		decoded := make([]byte, 0, (n+3)/4*4)
		for i := 1; i < len(text); i += 5 {
			value := uint64(0)
			for j := 0; j < 5; j++ {
				digit := strings.IndexByte(alphabet, text[i+j])
				if digit < 0 {
					return nil, fmt.Errorf("invalid binary base85 digit")
				}
				value = value*85 + uint64(digit)
			}
			if value > 0xffffffff {
				return nil, fmt.Errorf("binary base85 overflow")
			}
			decoded = append(decoded, byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
		}
		compressed = append(compressed, decoded[:n]...)
	}
	compressedReader := bytes.NewReader(compressed)
	reader, err := zlib.NewReader(compressedReader)
	if err != nil {
		return nil, fmt.Errorf("invalid binary compressed stream")
	}
	defer reader.Close()
	if len(expected) != 0 && fields[0] == "literal" {
		if size != int64(len(expected[0])) {
			return nil, fmt.Errorf("binary reverse payload size differs from the reference")
		}
		var scratch [32768]byte
		for at := 0; at < len(expected[0]); {
			n := min(len(scratch), len(expected[0])-at)
			if _, err := io.ReadFull(reader, scratch[:n]); err != nil || !bytes.Equal(scratch[:n], expected[0][at:at+n]) {
				return nil, fmt.Errorf("binary reverse payload differs from the reference")
			}
			at += n
		}
		n, err := reader.Read(scratch[:1])
		if n != 0 || err != io.EOF || compressedReader.Len() != 0 {
			return nil, fmt.Errorf("binary reverse payload has trailing or corrupt data")
		}
		return nil, nil
	}
	body := make([]byte, int(size))
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, fmt.Errorf("binary payload size differs")
	}
	var trailing [1]byte
	n, err := reader.Read(trailing[:])
	if n != 0 || err != io.EOF || compressedReader.Len() != 0 {
		return nil, fmt.Errorf("binary payload has trailing or corrupt data")
	}
	post := body
	if fields[0] == "delta" {
		post, err = reconstructBinaryDelta(pre, body, limit, expected...)
		if err != nil {
			return nil, err
		}
	}
	if rest != "" {
		if _, err := reconstructBinaryRecord("\nGIT binary patch\n"+rest+"\n", post, int64(len(pre)), pre); err != nil {
			return nil, err
		}
	}
	return post, nil
}

func reconstructBinaryDelta(pre, delta []byte, limit int64, expected ...[]byte) ([]byte, error) {
	at := 0
	readSize := func() (uint64, bool) {
		value := uint64(0)
		for shift := uint(0); shift < 63 && at < len(delta); shift += 7 {
			b := delta[at]
			at++
			value |= uint64(b&127) << shift
			if b&128 == 0 {
				return value, true
			}
		}
		return 0, false
	}
	baseSize, ok := readSize()
	if !ok || baseSize != uint64(len(pre)) {
		return nil, fmt.Errorf("binary delta preimage size differs")
	}
	size, ok := readSize()
	if !ok {
		return nil, fmt.Errorf("invalid binary delta size")
	}
	if limit < 0 || size > uint64(limit) {
		return nil, ErrPatchImageBudget
	}
	var out []byte
	if len(expected) == 0 {
		out = make([]byte, 0, int(size))
	} else if size != uint64(len(expected[0])) {
		return nil, fmt.Errorf("binary reverse delta size differs from the reference")
	}
	produced := 0
	emit := func(piece []byte) error {
		if len(expected) == 0 {
			out = append(out, piece...)
		} else if !bytes.Equal(piece, expected[0][produced:produced+len(piece)]) {
			return fmt.Errorf("binary reverse delta differs from the reference")
		}
		produced += len(piece)
		return nil
	}
	for at < len(delta) {
		op := delta[at]
		at++
		if op == 0 {
			return nil, fmt.Errorf("invalid binary delta opcode")
		}
		if op&128 == 0 {
			n := int(op)
			if n > len(delta)-at || n > int(size)-produced {
				return nil, fmt.Errorf("binary delta insertion exceeds bounds")
			}
			if err := emit(delta[at : at+n]); err != nil {
				return nil, err
			}
			at += n
			continue
		}
		offset, count := uint64(0), uint64(0)
		for bit := uint(0); bit < 7; bit++ {
			if op&(1<<bit) == 0 {
				continue
			}
			if at >= len(delta) {
				return nil, fmt.Errorf("binary delta copy truncated")
			}
			if bit < 4 {
				offset |= uint64(delta[at]) << (8 * bit)
			} else {
				count |= uint64(delta[at]) << (8 * (bit - 4))
			}
			at++
		}
		if count == 0 {
			count = 0x10000
		}
		if offset > uint64(len(pre)) || count > uint64(len(pre))-offset || count > size-uint64(produced) {
			return nil, fmt.Errorf("binary delta copy exceeds bounds")
		}
		if err := emit(pre[int(offset):int(offset+count)]); err != nil {
			return nil, err
		}
	}
	if produced != int(size) {
		return nil, fmt.Errorf("binary delta result size differs")
	}
	return out, nil
}
func reconstructTextEffect(patch string, effect PatchEffect, pre []byte, limit int64) ([]byte, bool, error) {
	fragment := patch[effect.FragmentStart:effect.FragmentEnd]
	post, err := reconstructTextRecord(fragment, pre, limit)
	if err != nil {
		return nil, false, err
	}
	if effect.ChangeKind == ChangeKindDelete && len(post) != 0 {
		return nil, false, fmt.Errorf("deletion leaves reference content")
	}
	return post, true, nil
}

func reconstructTextRecord(fragment string, pre []byte, limit int64) ([]byte, error) {
	lines := splitPatchLines(fragment)
	old := bytes.SplitAfter(pre, []byte{'\n'})
	if len(old[len(old)-1]) == 0 {
		old = old[:len(old)-1]
	}
	cursor, outputLines := 0, 0
	type imagePiece struct {
		body []byte
		text string
	}
	var pieces []imagePiece
	size := int64(0)
	terminated := true
	appendText := func(body string) error {
		if !terminated && len(body) != 0 {
			return fmt.Errorf("payload follows an end-of-file marker")
		}
		if len(body) != 0 {
			terminated = strings.HasSuffix(body, "\n")
		}
		pieces = append(pieces, imagePiece{text: body})
		size += int64(len(body))
		return nil
	}
	appendBody := func(body []byte) error {
		if !terminated && len(body) != 0 {
			return fmt.Errorf("payload follows an end-of-file marker")
		}
		if len(body) != 0 {
			terminated = body[len(body)-1] == '\n'
		}
		pieces = append(pieces, imagePiece{body: body})
		size += int64(len(body))
		return nil
	}
	for i := 0; i < len(lines); i++ {
		text := lines[i].text
		if !strings.HasPrefix(text, "@@") {
			continue
		}
		fields := strings.Fields(text)
		start, count, newStart, newCount, ok := parseUnifiedHunkRange(text)
		if !ok || len(fields) < 4 || fields[0] != "@@" || fields[3] != "@@" ||
			start < 0 || count < 0 || newStart < 0 || newCount < 0 ||
			(count > 0 && start == 0) || (newCount > 0 && newStart == 0) {
			return nil, fmt.Errorf("invalid hunk range")
		}
		offset := start
		if count != 0 {
			offset--
		}
		if offset < cursor || offset > len(old) || count > len(old)-offset {
			return nil, fmt.Errorf("hunk lies outside the reference or overlaps")
		}
		for cursor < offset {
			if err := appendBody(old[cursor]); err != nil {
				return nil, err
			}
			cursor++
			outputLines++
		}
		expectedNew := newStart
		if newCount != 0 {
			expectedNew--
		}
		if outputLines != expectedNew {
			return nil, fmt.Errorf("hunk postimage position differs")
		}
		removed, added := 0, 0
		for removed < count || added < newCount {
			i++
			if i >= len(lines) || len(lines[i].text) == 0 {
				return nil, fmt.Errorf("truncated hunk body")
			}
			line := lines[i]
			prefix := line.text[0]
			if prefix != ' ' && prefix != '-' && prefix != '+' {
				return nil, fmt.Errorf("invalid hunk body prefix")
			}
			body := fragment[line.start+1 : line.end]
			if !strings.HasSuffix(body, "\n") {
				return nil, fmt.Errorf("unterminated hunk body line")
			}
			if i+1 < len(lines) && strings.HasPrefix(lines[i+1].text, `\`) {
				if lines[i+1].text != `\ No newline at end of file` {
					return nil, fmt.Errorf("invalid end-of-file marker")
				}
				body = strings.TrimSuffix(body, "\n")
				i++
			}
			if prefix != '+' {
				if removed >= count || cursor >= len(old) || !bytes.Equal(old[cursor], []byte(body)) {
					return nil, fmt.Errorf("hunk context differs from the reference")
				}
				cursor++
				removed++
			}
			if prefix != '-' {
				if added >= newCount {
					return nil, fmt.Errorf("hunk exceeds its declared range")
				}
				if err := appendText(body); err != nil {
					return nil, err
				}
				added++
				outputLines++
			}
		}
		// Extra payload cannot disappear into the header scan.
		if i+1 < len(lines) {
			next := lines[i+1].text
			if strings.HasPrefix(next, "+") || strings.HasPrefix(next, "-") ||
				strings.HasPrefix(next, " ") || strings.HasPrefix(next, `\`) {
				return nil, fmt.Errorf("hunk has undeclared trailing payload")
			}
		}
	}
	for cursor < len(old) {
		if err := appendBody(old[cursor]); err != nil {
			return nil, err
		}
		cursor++
	}
	if size > limit {
		return nil, ErrPatchImageBudget
	}
	post := make([]byte, int(size))
	offset := 0
	for _, piece := range pieces {
		if piece.body != nil {
			offset += copy(post[offset:], piece.body)
		} else {
			offset += copy(post[offset:], piece.text)
		}
	}
	return post, nil
}
