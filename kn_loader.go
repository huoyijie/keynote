package main

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/xeonx/timeago"
)

type keynote_t struct {
	Name, Title, Desc, Ago string
	Ctime                  time.Time
}

func loadKeynotes(keynotesDir string) (keynotes []keynote_t) {
	entries, _ := os.ReadDir(keynotesDir)
	for _, v := range entries {
		if v.IsDir() {
			continue
		}
		if name, found := strings.CutSuffix(v.Name(), ".md"); found {
			info, err := v.Info()
			if err != nil {
				continue
			}

			stat := info.Sys().(*syscall.Stat_t)
			ctime := time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))

			var desc string
			file, err := os.Open(filepath.Join(keynotesDir, v.Name()))
			if err != nil {
				continue
			}

			if scanner := bufio.NewScanner(file); scanner.Scan() {
				desc = strings.TrimPrefix(scanner.Text(), "[//]: # (")
				desc = strings.TrimSuffix(desc, ")")
			}

			keynotes = append(keynotes, keynote_t{
				Name:  name,
				Title: strings.ReplaceAll(name, "-", " "),
				Desc:  desc,
				Ctime: ctime,
				Ago:   timeago.Chinese.Format(ctime),
			})
		}
	}

	if len(keynotes) > 0 {
		sort.Slice(keynotes, func(i, j int) bool {
			return keynotes[i].Ctime.After(keynotes[j].Ctime)
		})
	}
	return
}
