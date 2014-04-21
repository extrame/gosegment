package config

import (
	"flag"
	"path/filepath"
)

var Config t_Config

type t_Config struct {
	dictDir string
}

func (t *t_Config) Init() {
	flag.StringVar(&t.dictDir, "seg_dict_dir", "./upload", "中文切词词库路径")
}
func Init() {
	Config.Init()
}
func (t *t_Config) DictDir() string {
	return filepath.FromSlash(t.dictDir)
}
