package content

import (
	"io"

	"gopkg.in/yaml.v3"
)

type Action struct {
	ActionTitle      string `yaml:"ActionTitle"`
	Description      string `yaml:"Description"`
	CompletedMessage string `yaml:"CompletedMessage"`
}

type Task struct {
	TaskTitle    string   `yaml:"TaskTitle"`
	StartMessage string   `yaml:"StartMessage"`
	Actions      []Action `yaml:"Actions"`
}

type Chapter struct {
	ChapterTitle                string   `yaml:"ChapterTitle"`
	Description                 string   `yaml:"Description"`
	SkipMessage                 string   `yaml:"SkipMessage"`
	Group                       string   `yaml:"Group"`
	StartMessage                string   `yaml:"StartMessage"`
	CompletedMessage            string   `yaml:"CompletedMessage"`
	Tasks                       []Task   `yaml:"Tasks"`
	ActivateChapterOnCompletion string   `yaml:"ActivateChapterOnCompletion"`
	RewardedChapters            []string `yaml:"RewardedChapters"`
}

type pdaDocument struct {
	Chapters []Chapter `yaml:"Chapters"`
}

func ParsePDAYAML(r io.Reader) ([]Chapter, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var doc pdaDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc.Chapters, nil
}
