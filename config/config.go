package config

import (
	"flag"
	"github.com/BurntSushi/toml"
	"os"
	csLog "web/csgo/log"
)

var Conf = &CsConfig{
	logger: csLog.Default(),
}

type CsConfig struct {
	logger   *csLog.Logger
	Log      map[string]any
	Pool     map[string]any
	Template map[string]any
	Redis    map[string]any
	Mysql    map[string]any
}

//默认初始化的方法
func init() {
	loadToml()
}

func loadToml() {
	//默认的配置，若是自己不配置，那么使用默认配置路径
	configFile := flag.String("conf", "conf/app.toml", "app config file")
	flag.Parse()
	//判断是否存在对应路径上的文件
	if _, err := os.Stat(*configFile); err != nil {
		Conf.logger.Info("conf/app.toml file not load,because not exist")
		return
	}
	//通过插件读取配置文件的内容,并且解码到Conf结构体中对应的属性
	_, err := toml.DecodeFile(*configFile, Conf)
	if err != nil {
		Conf.logger.Info("conf/app.toml file not load")
		return
	}

}
