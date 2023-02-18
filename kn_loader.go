package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type folder_t struct {
	// private fields
	path string

	// public fields
	Name       string
	SubFolders []*folder_t
	Keynotes   []*keynote_t
}

type keynote_t struct {
	Name, Title string
	Ctime       time.Time
}

func loadKeynotes(keynotesDir, folderName string) (folder *folder_t) {
	folder = &folder_t{
		path: keynotesDir,
		Name: folderName,
	}

	entries, _ := os.ReadDir(keynotesDir)
	for _, v := range entries {
		if v.IsDir() {
			subFolder := loadKeynotes(filepath.Join(keynotesDir, v.Name()), v.Name())
			folder.SubFolders = append(folder.SubFolders, subFolder)
			continue
		}

		if name, found := strings.CutSuffix(v.Name(), ".md"); found {
			info, err := v.Info()
			if err != nil {
				continue
			}

			stat := info.Sys().(*syscall.Stat_t)
			ctime := time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))

			folder.Keynotes = append(folder.Keynotes, &keynote_t{
				Name:  name,
				Title: strings.ReplaceAll(name, "-", " "),
				Ctime: ctime,
			})
		}
	}

	if len(folder.Keynotes) > 0 {
		sort.Slice(folder.Keynotes, func(i, j int) bool {
			return folder.Keynotes[i].Ctime.After(folder.Keynotes[j].Ctime)
		})
	}
	return
}
