package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type SiteConfig struct {
	Logo, Link, Icon, Name, Title, Author, Desc, Summry, Copyright string
}

func loadConfig(confFile string) (site *SiteConfig) {
	data, err := os.ReadFile(confFile)
	fatalErr(err)

	site = &SiteConfig{}

	err = yaml.Unmarshal(data, site)
	fatalErr(err)
	return
}
