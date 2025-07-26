package recorder

import (
	"encoding/json"
	"os"
)

// JSON 文件记录器
type JSONFileRecorder struct {
	Path string
}

func NewJSONFileRecorder(path string) *JSONFileRecorder {
	return &JSONFileRecorder{
		path,
	}
}

func (r *JSONFileRecorder) Record(result any) error {
	file, err := os.OpenFile(r.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	//defer file.Close()

	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	data = append(data, '\n')
	_, err = file.Write(data)
	return err
}
