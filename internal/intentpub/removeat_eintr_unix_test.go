//go:build (linux && !android) || (darwin && !ios)

package intentpub

import (
	"errors"
	"io/fs"
	"reflect"
	"syscall"
	"testing"
)

func TestReadTempContentRetriesEINTRWithoutNegativeOffset(t *testing.T) {
	scratch := make([]byte, 4)
	var offsets []int64
	calls := 0
	count, err := readTempContent(7, scratch, len(scratch), func(
		fd int, destination []byte, offset int64,
	) (int, error) {
		if fd != 7 {
			t.Fatalf("descriptor = %d, want 7", fd)
		}
		offsets = append(offsets, offset)
		calls++
		switch calls {
		case 1:
			return -1, syscall.EINTR
		case 2:
			return copy(destination, "abc"), nil
		default:
			return 0, nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 || string(scratch[:count]) != "abc" {
		t.Fatalf("read = %d/%q, want 3/abc", count, scratch[:count])
	}
	if !reflect.DeepEqual(offsets, []int64{0, 0, 3}) {
		t.Fatalf("pread offsets = %v, want [0 0 3]", offsets)
	}
}

func TestReadTempContentAccountsForPartialEINTR(t *testing.T) {
	scratch := make([]byte, 3)
	var offsets []int64
	calls := 0
	count, err := readTempContent(9, scratch, len(scratch), func(
		_ int, destination []byte, offset int64,
	) (int, error) {
		offsets = append(offsets, offset)
		calls++
		switch calls {
		case 1:
			destination[0] = 'a'
			return 1, syscall.EINTR
		case 2:
			return copy(destination, "bc"), nil
		default:
			return 0, nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 || string(scratch) != "abc" {
		t.Fatalf("read = %d/%q, want 3/abc", count, scratch)
	}
	if !reflect.DeepEqual(offsets, []int64{0, 1}) {
		t.Fatalf("pread offsets = %v, want [0 1]", offsets)
	}
}

func TestReadTempContentRejectsInvalidNegativeRead(t *testing.T) {
	count, err := readTempContent(1, make([]byte, 1), 1, func(
		int, []byte, int64,
	) (int, error) {
		return -1, nil
	})
	if count != 0 || !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("negative read = %d/%v, want 0/%v", count, err, fs.ErrInvalid)
	}
}
