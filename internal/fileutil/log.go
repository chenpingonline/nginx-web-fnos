package fileutil

import "os"

// AppendLog reopens the active path for each entry so rotation never leaves
// the logger writing to a renamed file. The standard logger serializes writes.
type AppendLog struct{ Path string }

func (w AppendLog) Write(data []byte) (int, error) {
	file, err := os.OpenFile(w.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	n, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return n, writeErr
	}
	return n, closeErr
}
