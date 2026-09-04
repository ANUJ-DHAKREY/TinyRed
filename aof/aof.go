package aof

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"tinyred/resp"
)

type AOF interface {
	Load(filepath string, replay ReplayFunc) error
	Append(data []byte) error
}

type ManifestMetaData struct {
	// "file appendonly.aof.1.incr.aof seq 1 type i\n
	NodeType    string
	AofFilename string
	Seq         int
	FileType    byte
}

type AOFLocal struct {
	file *os.File
	mu   sync.RWMutex
}

var a AOFLocal

func (a *AOFLocal) AOF(filepath string) (AOFLocal, error) {

	file, err := os.Open(filepath)
	if err != nil {
		return AOFLocal{}, err
	}

	return AOFLocal{}, nil
}

type ReplayFunc func(args []string) error

func (a *AOFLocal) Load(filePath string, replay ReplayFunc) (err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(file)
	defer file.Close()
	for {
		arr := resp.Array{}
		err := arr.Unmarshal(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if len(arr.Value) == 0 {
			return fmt.Errorf("Replay Error: empty command array")
		}

		err = replay(arr.Value)
		if err != nil {
			return fmt.Errorf("Replay error: %s", err.Error())
		}
	}
}

func (a *AOFLocal) Append(data []byte) error {
	file := AOF()
}
