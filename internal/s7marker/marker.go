package s7marker

import (
	"fmt"
	"os"
)

func Emit(path, correlationToken string) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		panic(fmt.Sprintf("create S7 marker: %v", err))
	}
	if _, err := file.WriteString(correlationToken); err != nil {
		_ = file.Close()
		panic(fmt.Sprintf("write S7 marker: %v", err))
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		panic(fmt.Sprintf("sync S7 marker: %v", err))
	}
	if err := file.Close(); err != nil {
		panic(fmt.Sprintf("close S7 marker: %v", err))
	}
}
