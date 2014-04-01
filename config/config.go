package config

import (
	"flag"
)

var Config t_Config

type t_Config struct {
	DictDir string
}

func (t *t_Config) Init() {
	flag.StringVar(&t.DictDir, "seg_dict_dir", "./upload", "中文切词词库路径")
}
func Init() {
	Config.Init()
}
